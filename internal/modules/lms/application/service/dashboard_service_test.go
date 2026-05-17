package service

import (
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestBuildDashboardMetricsCountsOpenSupportCompletedAndScope(t *testing.T) {
	now := time.Date(2026, 5, 7, 9, 0, 0, 0, time.UTC)
	classID := bson.NewObjectID()
	quizID := bson.NewObjectID()
	students := []bson.ObjectID{bson.NewObjectID(), bson.NewObjectID(), bson.NewObjectID()}
	openAssignment := Assignment{
		ID:         bson.NewObjectID(),
		ClassID:    classID,
		QuizID:     quizID,
		StudentIDs: students,
		Status:     "open",
		CreatedAt:  now.Add(-time.Hour),
	}
	closedAssignment := Assignment{
		ID:         bson.NewObjectID(),
		ClassID:    classID,
		QuizID:     quizID,
		StudentIDs: students[:2],
		Status:     "closed",
		CreatedAt:  now.Add(-2 * time.Hour),
	}
	states := []dashboardAssignmentState{
		buildDashboardAssignmentState(openAssignment, "6A", classID.Hex(), []Attempt{
			submittedAttempt(openAssignment, students[0], 88, now),
			{ID: bson.NewObjectID(), AssignmentID: openAssignment.ID, QuizID: quizID, StudentID: students[1], Status: "in_progress", UpdatedAt: now},
		}, now),
		buildDashboardAssignmentState(closedAssignment, "6A", classID.Hex(), []Attempt{
			submittedAttempt(closedAssignment, students[0], 91, now),
			submittedAttempt(closedAssignment, students[1], 77, now),
		}, now),
	}

	metrics := buildDashboardMetrics(states, len(students), 2)

	if metrics.OpenAssignments != 1 {
		t.Fatalf("expected 1 open assignment, got %d", metrics.OpenAssignments)
	}
	if metrics.Completed != 3 || metrics.CompletedAssignments != 1 {
		t.Fatalf("unexpected completion metrics: %+v", metrics)
	}
	if metrics.NeedsSupport != 2 || metrics.TotalStudents != 3 || metrics.TotalAssignments != 2 {
		t.Fatalf("unexpected dashboard scope metrics: %+v", metrics)
	}
	if metrics.CompletionRate != 60 {
		t.Fatalf("expected 60 completion rate, got %.2f", metrics.CompletionRate)
	}
}

func TestBuildDashboardInterventionsFlagsLowScoresAndOverdue(t *testing.T) {
	now := time.Date(2026, 5, 7, 9, 0, 0, 0, time.UTC)
	classID := bson.NewObjectID()
	quizID := bson.NewObjectID()
	lowScore := 45.0
	studentLowAverage := Student{ID: bson.NewObjectID(), CenterID: classID, ClassID: classID, FullName: "Low Average", Username: "low.avg", Metrics: StudentMetrics{AverageScore: &lowScore}}
	studentOverdue := Student{ID: bson.NewObjectID(), CenterID: classID, ClassID: classID, FullName: "Over Due", Username: "over.due"}
	dueAt := now.Add(-24 * time.Hour)
	assignment := Assignment{
		ID:         bson.NewObjectID(),
		ClassID:    classID,
		QuizID:     quizID,
		StudentIDs: []bson.ObjectID{studentOverdue.ID},
		DueAt:      &dueAt,
		Status:     "open",
		CreatedAt:  now.Add(-48 * time.Hour),
	}
	states := []dashboardAssignmentState{buildDashboardAssignmentState(assignment, "6A", classID.Hex(), nil, now)}
	studentItems := []StudentListItemDTO{
		{ID: studentLowAverage.ID.Hex(), FullName: studentLowAverage.FullName, ClassID: classID.Hex(), ClassName: "6A"},
		{ID: studentOverdue.ID.Hex(), FullName: studentOverdue.FullName, ClassID: classID.Hex(), ClassName: "6A"},
	}

	items := buildDashboardInterventions([]Student{studentLowAverage, studentOverdue}, studentItems, states, now)

	if len(items) != 2 {
		t.Fatalf("expected 2 interventions, got %d: %+v", len(items), items)
	}
	reasons := map[string]bool{}
	for _, item := range items {
		reasons[item.Reason] = true
	}
	if !reasons["low_average_score"] || !reasons["overdue_assignment"] {
		t.Fatalf("expected low average and overdue reasons, got %+v", reasons)
	}
}

func TestBuildActiveAssignmentDTOsSummarizesProgress(t *testing.T) {
	now := time.Date(2026, 5, 7, 9, 0, 0, 0, time.UTC)
	classID := bson.NewObjectID()
	quizID := bson.NewObjectID()
	students := []bson.ObjectID{bson.NewObjectID(), bson.NewObjectID(), bson.NewObjectID()}
	dueAt := now.Add(-time.Hour)
	assignment := Assignment{
		ID:            bson.NewObjectID(),
		ClassID:       classID,
		QuizID:        quizID,
		SubjectID:     bson.NewObjectID(),
		StudentIDs:    students,
		DueAt:         &dueAt,
		Status:        "open",
		RecipientMode: "all",
		CreatedAt:     now.Add(-2 * time.Hour),
		UpdatedAt:     now.Add(-time.Hour),
	}
	state := buildDashboardAssignmentState(assignment, "6A", classID.Hex(), []Attempt{
		submittedAttempt(assignment, students[0], 88, now),
		{ID: bson.NewObjectID(), AssignmentID: assignment.ID, QuizID: quizID, StudentID: students[1], Status: "in_progress", UpdatedAt: now},
	}, now)

	items := buildActiveAssignmentDTOs([]dashboardAssignmentState{state}, true)

	if len(items) != 1 {
		t.Fatalf("expected 1 active assignment, got %d", len(items))
	}
	got := items[0]
	if got.Submitted != 1 || got.InProgress != 1 || got.NotStarted != 1 || got.Overdue != 2 {
		t.Fatalf("unexpected active assignment progress: %+v", got)
	}
	if got.CompletionPercent != float64(1)/float64(3)*100 {
		t.Fatalf("unexpected completion percent: %.2f", got.CompletionPercent)
	}
}

func TestBuildClassReportResponseComposesDashboardSections(t *testing.T) {
	class := Class{ID: bson.NewObjectID(), CenterID: bson.NewObjectID(), Name: "6A", Grade: "6", Status: statusActive}
	students := []StudentListItemDTO{{ID: bson.NewObjectID().Hex(), FullName: "Student One", ClassID: class.ID.Hex(), ClassName: class.Name}}
	assignments := []ActiveAssignmentDTO{{ID: bson.NewObjectID().Hex(), ClassID: class.ID.Hex(), ClassName: class.Name, TotalStudents: 1}}
	interventions := []DashboardInterventionDTO{{StudentID: students[0].ID, ClassID: class.ID.Hex(), Reason: "low_average_score"}}
	metrics := DashboardMetricsDTO{OpenAssignments: 1, NeedsSupport: 1, TotalStudents: 1}

	report := buildClassReportResponse(class, students, assignments, interventions, metrics)

	if report.Class.ID != class.ID.Hex() || report.Class.UnitID != class.CenterID.Hex() {
		t.Fatalf("unexpected class report class DTO: %+v", report.Class)
	}
	if len(report.Students) != 1 || len(report.ActiveAssignments) != 1 || len(report.Interventions) != 1 {
		t.Fatalf("unexpected class report sections: %+v", report)
	}
}

func TestBuildStudentJourneyAddsMilestonesAndFocusAreas(t *testing.T) {
	now := time.Date(2026, 5, 7, 9, 0, 0, 0, time.UTC)
	average := 85.0
	student := Student{ID: bson.NewObjectID(), FullName: "Journey Student", Note: "Keep mentoring", Metrics: StudentMetrics{AverageScore: &average, CompletedAssignments: 2}}
	assignment := Assignment{ID: bson.NewObjectID(), ClassID: bson.NewObjectID(), QuizID: bson.NewObjectID()}
	attempt := submittedAttempt(assignment, student.ID, 55, now)

	journey := buildStudentJourney(student, []Attempt{attempt}, map[string]Assignment{assignment.ID.Hex(): assignment})

	if !containsString(journey.Strengths, "strong_average_score") || !containsString(journey.Strengths, "assignment_completion") {
		t.Fatalf("unexpected strengths: %+v", journey.Strengths)
	}
	if !containsString(journey.FocusAreas, "review_low_score_assignments") {
		t.Fatalf("expected low-score focus area, got %+v", journey.FocusAreas)
	}
	if len(journey.Milestones) != 1 || journey.Milestones[0]["assignmentId"] != assignment.ID.Hex() {
		t.Fatalf("unexpected milestones: %+v", journey.Milestones)
	}
	if journey.MentorNote != "Keep mentoring" {
		t.Fatalf("expected mentor note to be preserved")
	}
}

func submittedAttempt(assignment Assignment, studentID bson.ObjectID, percent float64, at time.Time) Attempt {
	return Attempt{
		ID:           bson.NewObjectID(),
		AssignmentID: assignment.ID,
		QuizID:       assignment.QuizID,
		StudentID:    studentID,
		Status:       "submitted",
		Score:        percent,
		MaxScore:     100,
		Percent:      percent,
		SubmittedAt:  &at,
		UpdatedAt:    at,
	}
}
