package service

import (
	"context"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const supportScoreThreshold = 60

type dashboardData struct {
	Scope        DashboardScopeSummaryDTO
	Classes      []Class
	Students     []Student
	StudentItems []StudentListItemDTO
	States       []dashboardAssignmentState
}

type dashboardAssignmentState struct {
	Assignment Assignment
	ClassName  string
	UnitID     string
	Attempts   []Attempt
	Progress   dashboardAssignmentProgress
}

type dashboardAssignmentProgress struct {
	TotalStudents     int
	Submitted         int
	InProgress        int
	NotStarted        int
	NeedsReview       int
	Overdue           int
	CompletionPercent float64
	LatestByStudent   map[string]Attempt
}

func (s *Service) DashboardOverview(ctx context.Context, tenantID string, actor Actor, req DashboardScopeRequestDTO) (DashboardOverviewResponseDTO, error) {
	now := time.Now().UTC()
	data, err := s.loadDashboardData(ctx, tenantID, actor, req, now)
	if err != nil {
		return DashboardOverviewResponseDTO{}, err
	}
	interventions := buildDashboardInterventions(data.Students, data.StudentItems, data.States, now)
	return DashboardOverviewResponseDTO{
		Scope:       data.Scope,
		Metrics:     buildDashboardMetrics(data.States, len(data.Students), len(interventions)),
		GeneratedAt: now,
	}, nil
}

func (s *Service) DashboardInterventions(ctx context.Context, tenantID string, actor Actor, req DashboardScopeRequestDTO) (DashboardInterventionListResponseDTO, error) {
	now := time.Now().UTC()
	data, err := s.loadDashboardData(ctx, tenantID, actor, req, now)
	if err != nil {
		return DashboardInterventionListResponseDTO{}, err
	}
	items := buildDashboardInterventions(data.Students, data.StudentItems, data.States, now)
	return DashboardInterventionListResponseDTO{Scope: data.Scope, Items: items, Total: len(items)}, nil
}

func (s *Service) ActiveAssignments(ctx context.Context, tenantID string, actor Actor, req DashboardScopeRequestDTO) (ActiveAssignmentListResponseDTO, error) {
	now := time.Now().UTC()
	data, err := s.loadDashboardData(ctx, tenantID, actor, req, now)
	if err != nil {
		return ActiveAssignmentListResponseDTO{}, err
	}
	items := buildActiveAssignmentDTOs(data.States, true)
	return ActiveAssignmentListResponseDTO{Scope: data.Scope, Items: items, Total: len(items)}, nil
}

func (s *Service) ClassReport(ctx context.Context, tenantID string, actor Actor, classID string) (ClassReportResponseDTO, error) {
	now := time.Now().UTC()
	data, err := s.loadDashboardData(ctx, tenantID, actor, DashboardScopeRequestDTO{ScopeType: scopeLevelClass, ClassID: classID}, now)
	if err != nil {
		return ClassReportResponseDTO{}, err
	}
	if len(data.Classes) == 0 {
		return ClassReportResponseDTO{}, errNotFound
	}
	interventions := buildDashboardInterventions(data.Students, data.StudentItems, data.States, now)
	return buildClassReportResponse(
		data.Classes[0],
		data.StudentItems,
		buildActiveAssignmentDTOs(data.States, false),
		interventions,
		buildDashboardMetrics(data.States, len(data.Students), len(interventions)),
	), nil
}

func (s *Service) CreateAssignmentDelivery(ctx context.Context, tenantID string, actor Actor, req AssignmentDeliveryRequestDTO) (AssignmentDeliveryResponseDTO, error) {
	result, err := s.CreateAssignment(ctx, tenantID, actor, CreateAssignmentRequestDTO{
		ClassID:       req.ClassID,
		QuizIDs:       req.QuizIDs,
		RecipientMode: req.RecipientMode,
		StudentIDs:    req.StudentIDs,
		DueAt:         req.DueAt,
		TeacherNote:   req.TeacherNote,
	})
	if err != nil {
		return AssignmentDeliveryResponseDTO{}, err
	}
	return AssignmentDeliveryResponseDTO{
		AssignmentIDs:  result.AssignmentIDs,
		RecipientCount: result.RecipientCount,
		Status:         "delivered",
		DeliveredAt:    time.Now().UTC(),
	}, nil
}

func (s *Service) loadDashboardData(ctx context.Context, tenantID string, actor Actor, req DashboardScopeRequestDTO, now time.Time) (dashboardData, error) {
	unitID := dashboardUnitID(req)
	classes, err := s.dashboardClasses(ctx, tenantID, actor, unitID, req.ClassID)
	if err != nil {
		return dashboardData{}, err
	}
	classNames := make(map[string]string, len(classes))
	for _, class := range classes {
		classNames[class.ID.Hex()] = class.Name
	}

	students := make([]Student, 0)
	for _, class := range classes {
		items, _, _, err := s.repo.ListStudents(ctx, tenantID, StudentListRequestDTO{ClassID: class.ID.Hex(), Limit: maxStudentBatchSize}, nil, nil)
		if err != nil {
			return dashboardData{}, err
		}
		students = append(students, items...)
	}

	states := make([]dashboardAssignmentState, 0)
	rangeStart := dashboardRangeStart(req.Range, now)
	for _, class := range classes {
		assignments, err := s.repo.ListAssignments(ctx, tenantID, bson.M{"class_id": class.ID})
		if err != nil {
			return dashboardData{}, err
		}
		for _, assignment := range assignments {
			if !dashboardAssignmentInRange(assignment, rangeStart) {
				continue
			}
			attempts, err := s.repo.ListAttempts(ctx, tenantID, bson.M{"assignment_id": assignment.ID})
			if err != nil {
				return dashboardData{}, err
			}
			states = append(states, buildDashboardAssignmentState(assignment, classNames[class.ID.Hex()], class.CenterID.Hex(), attempts, now))
		}
	}

	scope := DashboardScopeSummaryDTO{
		ScopeType:    normalizeDashboardScope(req),
		UnitID:       unitID,
		CenterID:     unitID,
		ClassID:      req.ClassID,
		Range:        req.Range,
		ClassCount:   len(classes),
		StudentCount: len(students),
	}
	return dashboardData{Scope: scope, Classes: classes, Students: students, StudentItems: s.studentsToDTO(ctx, tenantID, students), States: states}, nil
}

func (s *Service) dashboardClasses(ctx context.Context, tenantID string, actor Actor, unitID, classID string) ([]Class, error) {
	if classID != "" {
		class, err := s.repo.GetClass(ctx, tenantID, classID)
		if err != nil {
			return nil, err
		}
		if class == nil {
			return nil, errNotFound
		}
		if !s.canAccessClass(ctx, tenantID, actor, *class) {
			return nil, errScopeForbidden
		}
		return []Class{*class}, nil
	}

	req := ClassListRequestDTO{CenterID: unitID, Page: 1, Limit: 100}
	if actor.canAccessGlobal() {
		classes, _, err := s.repo.ListClasses(ctx, tenantID, req, nil, "")
		return classes, err
	}

	merged := map[string]Class{}
	centers, _, err := s.repo.ListCenters(ctx, tenantID, CenterListRequestDTO{Page: 1, Limit: 100}, actor.UserID)
	if err != nil {
		return nil, err
	}
	managedCenterIDs := make([]bson.ObjectID, 0, len(centers))
	for _, center := range centers {
		if unitID == "" || center.ID.Hex() == unitID {
			managedCenterIDs = append(managedCenterIDs, center.ID)
		}
	}
	if len(managedCenterIDs) > 0 {
		items, _, err := s.repo.ListClasses(ctx, tenantID, req, managedCenterIDs, "")
		if err != nil {
			return nil, err
		}
		mergeClasses(merged, items)
	}

	items, _, err := s.repo.ListClasses(ctx, tenantID, req, nil, actor.UserID)
	if err != nil {
		return nil, err
	}
	mergeClasses(merged, items)

	classes := make([]Class, 0, len(merged))
	for _, class := range merged {
		classes = append(classes, class)
	}
	sort.Slice(classes, func(i, j int) bool { return classes[i].Name < classes[j].Name })
	return classes, nil
}

func mergeClasses(out map[string]Class, items []Class) {
	for _, item := range items {
		out[item.ID.Hex()] = item
	}
}

func buildDashboardAssignmentState(assignment Assignment, className, unitID string, attempts []Attempt, now time.Time) dashboardAssignmentState {
	progress := summarizeDashboardAssignment(assignment, attempts, now)
	return dashboardAssignmentState{Assignment: assignment, ClassName: className, UnitID: unitID, Attempts: attempts, Progress: progress}
}

func summarizeDashboardAssignment(assignment Assignment, attempts []Attempt, now time.Time) dashboardAssignmentProgress {
	progress := dashboardAssignmentProgress{
		TotalStudents:   len(assignment.StudentIDs),
		NotStarted:      len(assignment.StudentIDs),
		LatestByStudent: map[string]Attempt{},
	}
	for _, attempt := range attempts {
		studentID := attempt.StudentID.Hex()
		if !assignmentHasStudent(assignment, studentID) {
			continue
		}
		current, ok := progress.LatestByStudent[studentID]
		if !ok || attemptActivityAt(attempt).After(attemptActivityAt(current)) {
			progress.LatestByStudent[studentID] = attempt
		}
	}
	for _, studentID := range assignment.StudentIDs {
		attempt, ok := progress.LatestByStudent[studentID.Hex()]
		if !ok {
			if assignment.DueAt != nil && assignment.DueAt.Before(now) {
				progress.Overdue++
			}
			continue
		}
		progress.NotStarted--
		if attemptIsSubmitted(attempt) {
			progress.Submitted++
		} else {
			progress.InProgress++
			if assignment.DueAt != nil && assignment.DueAt.Before(now) {
				progress.Overdue++
			}
		}
		if attemptNeedsReview(attempt) {
			progress.NeedsReview++
		}
	}
	if progress.TotalStudents > 0 {
		progress.CompletionPercent = float64(progress.Submitted) / float64(progress.TotalStudents) * 100
	}
	return progress
}

func buildDashboardMetrics(states []dashboardAssignmentState, totalStudents, needsSupport int) DashboardMetricsDTO {
	metrics := DashboardMetricsDTO{TotalStudents: totalStudents, NeedsSupport: needsSupport, TotalAssignments: len(states)}
	totalRecipients := 0
	for _, state := range states {
		if assignmentIsOpen(state.Assignment) {
			metrics.OpenAssignments++
		}
		metrics.Completed += state.Progress.Submitted
		totalRecipients += state.Progress.TotalStudents
		if state.Progress.TotalStudents > 0 && state.Progress.Submitted == state.Progress.TotalStudents {
			metrics.CompletedAssignments++
		}
	}
	if totalRecipients > 0 {
		metrics.CompletionRate = float64(metrics.Completed) / float64(totalRecipients) * 100
	}
	return metrics
}

func buildActiveAssignmentDTOs(states []dashboardAssignmentState, onlyOpen bool) []ActiveAssignmentDTO {
	items := make([]ActiveAssignmentDTO, 0, len(states))
	for _, state := range states {
		if onlyOpen && !assignmentIsOpen(state.Assignment) {
			continue
		}
		items = append(items, activeAssignmentToDTO(state))
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].DueAt == nil && items[j].DueAt != nil {
			return false
		}
		if items[i].DueAt != nil && items[j].DueAt == nil {
			return true
		}
		if items[i].DueAt != nil && items[j].DueAt != nil && !items[i].DueAt.Equal(*items[j].DueAt) {
			return items[i].DueAt.Before(*items[j].DueAt)
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	return items
}

func activeAssignmentToDTO(state dashboardAssignmentState) ActiveAssignmentDTO {
	assignment := assignmentToDTO(state.Assignment)
	return ActiveAssignmentDTO{
		ID:                assignment.ID,
		ClassID:           assignment.ClassID,
		ClassName:         state.ClassName,
		UnitID:            state.UnitID,
		QuizID:            assignment.QuizID,
		SubjectID:         assignment.SubjectID,
		DueAt:             assignment.DueAt,
		TeacherNote:       assignment.TeacherNote,
		Status:            assignment.Status,
		RecipientMode:     assignment.RecipientMode,
		TotalStudents:     state.Progress.TotalStudents,
		Submitted:         state.Progress.Submitted,
		InProgress:        state.Progress.InProgress,
		NotStarted:        state.Progress.NotStarted,
		NeedsReview:       state.Progress.NeedsReview,
		Overdue:           state.Progress.Overdue,
		CompletionPercent: state.Progress.CompletionPercent,
		CreatedAt:         assignment.CreatedAt,
		UpdatedAt:         assignment.UpdatedAt,
	}
}

func buildDashboardInterventions(students []Student, studentItems []StudentListItemDTO, states []dashboardAssignmentState, now time.Time) []DashboardInterventionDTO {
	itemsByStudent := make(map[string]StudentListItemDTO, len(studentItems))
	for _, item := range studentItems {
		itemsByStudent[item.ID] = item
	}

	interventions := make([]DashboardInterventionDTO, 0)
	for _, student := range students {
		studentID := student.ID.Hex()
		item := itemsByStudent[studentID]
		intervention := DashboardInterventionDTO{
			StudentID:      studentID,
			StudentName:    student.FullName,
			Username:       student.Username,
			UnitID:         student.CenterID.Hex(),
			ClassID:        student.ClassID.Hex(),
			ClassName:      item.ClassName,
			Severity:       "medium",
			LastActivityAt: student.Metrics.LastActivityAt,
		}

		if student.Metrics.AverageScore != nil && *student.Metrics.AverageScore < supportScoreThreshold {
			intervention.Reason = "low_average_score"
			intervention.RecommendedAction = "Review recent submissions and assign focused practice."
			intervention.Score = student.Metrics.AverageScore
			if *student.Metrics.AverageScore < 50 {
				intervention.Severity = "high"
			}
		}

		for _, state := range states {
			if !assignmentHasStudent(state.Assignment, studentID) {
				continue
			}
			attempt, hasAttempt := state.Progress.LatestByStudent[studentID]
			if state.Assignment.DueAt != nil && state.Assignment.DueAt.Before(now) && (!hasAttempt || !attemptIsSubmitted(attempt)) {
				intervention.Reason = "overdue_assignment"
				intervention.Severity = "high"
				intervention.RecommendedAction = "Follow up on the overdue assignment and reopen support time."
				intervention.AssignmentID = state.Assignment.ID.Hex()
				intervention.QuizID = state.Assignment.QuizID.Hex()
				break
			}
			if hasAttempt && attemptNeedsReview(attempt) {
				score := attempt.Percent
				intervention.Reason = "low_assignment_score"
				intervention.RecommendedAction = "Schedule correction review for the low scoring assignment."
				intervention.AssignmentID = state.Assignment.ID.Hex()
				intervention.QuizID = state.Assignment.QuizID.Hex()
				intervention.Score = &score
				activityAt := attemptActivityAt(attempt)
				intervention.LastActivityAt = &activityAt
				if attempt.Percent < 50 {
					intervention.Severity = "high"
				}
				break
			}
		}

		if intervention.Reason != "" {
			interventions = append(interventions, intervention)
		}
	}
	sort.Slice(interventions, func(i, j int) bool {
		left := severityRank(interventions[i].Severity)
		right := severityRank(interventions[j].Severity)
		if left != right {
			return left > right
		}
		return interventions[i].StudentName < interventions[j].StudentName
	})
	return interventions
}

func buildClassReportResponse(class Class, students []StudentListItemDTO, activeAssignments []ActiveAssignmentDTO, interventions []DashboardInterventionDTO, metrics DashboardMetricsDTO) ClassReportResponseDTO {
	return ClassReportResponseDTO{
		Class:             classToDTO(class, ""),
		Summary:           metrics,
		Students:          students,
		ActiveAssignments: activeAssignments,
		Interventions:     interventions,
	}
}

func buildStudentJourney(student Student, attempts []Attempt, assignments map[string]Assignment) StudentJourneyResponseDTO {
	strengths := []string{}
	focusAreas := []string{}
	if student.Metrics.AverageScore != nil {
		if *student.Metrics.AverageScore >= 80 {
			strengths = append(strengths, "strong_average_score")
		}
		if *student.Metrics.AverageScore < supportScoreThreshold {
			focusAreas = append(focusAreas, "raise_average_score")
		}
	}
	if student.Metrics.CompletedAssignments > 0 {
		strengths = append(strengths, "assignment_completion")
	}
	if len(attempts) == 0 {
		focusAreas = append(focusAreas, "start_first_assignment")
	}

	milestones := make([]map[string]any, 0, len(attempts))
	for _, attempt := range attempts {
		milestone := map[string]any{
			"type":         "attempt",
			"assignmentId": attempt.AssignmentID.Hex(),
			"quizId":       attempt.QuizID.Hex(),
			"status":       attempt.Status,
			"score":        attempt.Score,
			"percent":      attempt.Percent,
			"updatedAt":    attemptActivityAt(attempt),
		}
		if assignment, ok := assignments[attempt.AssignmentID.Hex()]; ok {
			milestone["classId"] = assignment.ClassID.Hex()
			milestone["dueAt"] = assignment.DueAt
		}
		milestones = append(milestones, milestone)
		if attemptNeedsReview(attempt) && !containsString(focusAreas, "review_low_score_assignments") {
			focusAreas = append(focusAreas, "review_low_score_assignments")
		}
	}
	if len(strengths) == 0 {
		strengths = append(strengths, "consistent_participation")
	}
	if len(focusAreas) == 0 {
		focusAreas = append(focusAreas, "maintain_learning_rhythm")
	}
	return StudentJourneyResponseDTO{Strengths: strengths, FocusAreas: focusAreas, Milestones: milestones, MentorNote: student.Note}
}

func dashboardUnitID(req DashboardScopeRequestDTO) string {
	if req.UnitID != "" {
		return req.UnitID
	}
	return req.CenterID
}

func normalizeDashboardScope(req DashboardScopeRequestDTO) string {
	scopeType := strings.ToLower(strings.TrimSpace(req.ScopeType))
	switch scopeType {
	case "center", "school":
		return "unit"
	case "":
		if req.ClassID != "" {
			return scopeLevelClass
		}
		if dashboardUnitID(req) != "" {
			return "unit"
		}
		return scopeLevelGlobal
	default:
		return scopeType
	}
}

func dashboardRangeStart(value string, now time.Time) *time.Time {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "today":
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		return &start
	case "7d", "week", "last7days":
		start := now.AddDate(0, 0, -7)
		return &start
	case "30d", "month", "last30days":
		start := now.AddDate(0, -1, 0)
		return &start
	case "quarter", "90d":
		start := now.AddDate(0, -3, 0)
		return &start
	case "year", "12m":
		start := now.AddDate(-1, 0, 0)
		return &start
	default:
		return nil
	}
}

func dashboardAssignmentInRange(assignment Assignment, start *time.Time) bool {
	if start == nil {
		return true
	}
	if !assignment.CreatedAt.IsZero() && !assignment.CreatedAt.Before(*start) {
		return true
	}
	if !assignment.UpdatedAt.IsZero() && !assignment.UpdatedAt.Before(*start) {
		return true
	}
	return assignment.DueAt != nil && !assignment.DueAt.Before(*start)
}

func assignmentIsOpen(assignment Assignment) bool {
	switch strings.ToLower(strings.TrimSpace(assignment.Status)) {
	case "", "open", "active", "in_progress", "published":
		return true
	default:
		return false
	}
}

func assignmentHasStudent(assignment Assignment, studentID string) bool {
	for _, id := range assignment.StudentIDs {
		if id.Hex() == studentID {
			return true
		}
	}
	return false
}

func attemptIsSubmitted(attempt Attempt) bool {
	return strings.EqualFold(attempt.Status, "submitted")
}

func attemptNeedsReview(attempt Attempt) bool {
	status := strings.ToLower(strings.TrimSpace(attempt.Status))
	if status == "needs_review" || status == "review" {
		return true
	}
	return attemptIsSubmitted(attempt) && attempt.Percent < supportScoreThreshold
}

func attemptActivityAt(attempt Attempt) time.Time {
	if !attempt.UpdatedAt.IsZero() {
		return attempt.UpdatedAt
	}
	if attempt.SubmittedAt != nil {
		return *attempt.SubmittedAt
	}
	return attempt.StartedAt
}

func severityRank(severity string) int {
	switch strings.ToLower(severity) {
	case "high":
		return 3
	case "medium":
		return 2
	default:
		return 1
	}
}

func containsString(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}
