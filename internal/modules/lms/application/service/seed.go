package service

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	SeedTenantID = "erg-seed"

	SeedAdminUserID   = "lms-admin-seed"
	SeedManagerUserID = "center-manager-seed"
	SeedTeacherUserID = "teacher-seed"
)

type LMSSeedFixture struct {
	TenantID     string
	Admin        Actor
	Manager      Actor
	Teacher      Actor
	Student      Actor
	SchoolID     string
	CenterID     string
	ClassID      string
	StudentID    string
	SubjectID    string
	LevelID      string
	TopicID      string
	QuestionID   string
	QuizID       string
	AssignmentID string
	AttemptID    string
}

func (s *Service) SeedDemoData(ctx context.Context, tenantID string) (LMSSeedFixture, error) {
	_ = ctx
	if tenantID == "" {
		tenantID = SeedTenantID
	}
	if s == nil || s.repo == nil || s.repo.memory == nil {
		return LMSSeedFixture{}, fmt.Errorf("lms seed demo data requires in-memory repository")
	}

	now := time.Now().UTC()
	publishedAt := now.Add(-2 * time.Hour)
	dueAt := now.Add(72 * time.Hour)
	submittedAt := now.Add(-30 * time.Minute)
	lastActivity := now.Add(-20 * time.Minute)
	avgStrong := 88.0
	avgDeveloping := 56.0

	schoolID := mustSeedObjectID("665000000000000000000001")
	centerID := mustSeedObjectID("665000000000000000000002")
	schoolClassID := mustSeedObjectID("665000000000000000000003")
	classID := mustSeedObjectID("665000000000000000000004")
	studentID := mustSeedObjectID("665000000000000000000005")
	studentInProgressID := mustSeedObjectID("665000000000000000000006")
	studentNeedsSupportID := mustSeedObjectID("665000000000000000000007")
	subjectID := mustSeedObjectID("665000000000000000000008")
	levelID := mustSeedObjectID("665000000000000000000009")
	topicID := mustSeedObjectID("66500000000000000000000a")
	questionID := mustSeedObjectID("66500000000000000000000b")
	quizID := mustSeedObjectID("66500000000000000000000c")
	assignmentID := mustSeedObjectID("66500000000000000000000d")
	attemptID := mustSeedObjectID("66500000000000000000000e")
	inProgressAttemptID := mustSeedObjectID("66500000000000000000000f")

	m := s.repo.memory
	m.centers = upsertSeedCenter(m.centers, Center{
		ID:            schoolID,
		TenantID:      tenantID,
		Type:          educationUnitTypeSchool,
		Name:          "ERG Demo School",
		Code:          "ERG-SCH-DEMO",
		Address:       "Demo school campus",
		Status:        statusActive,
		ManagerUserID: SeedManagerUserID,
		CreatedAt:     now.Add(-48 * time.Hour),
		UpdatedAt:     now.Add(-24 * time.Hour),
	})
	m.centers = upsertSeedCenter(m.centers, Center{
		ID:            centerID,
		TenantID:      tenantID,
		Type:          educationUnitTypeCenter,
		Name:          "ERG Demo Center",
		Code:          "ERG-CTR-DEMO",
		Address:       "Demo center campus",
		Status:        statusActive,
		ManagerUserID: SeedManagerUserID,
		CreatedAt:     now.Add(-48 * time.Hour),
		UpdatedAt:     now.Add(-24 * time.Hour),
	})

	m.classes = upsertSeedClass(m.classes, Class{
		ID:                schoolClassID,
		TenantID:          tenantID,
		CenterID:          schoolID,
		Name:              "School ICT 6A",
		Grade:             "6",
		AcademicYear:      "2025-2026",
		Status:            statusActive,
		HomeroomTeacherID: SeedTeacherUserID,
		CreatedAt:         now.Add(-36 * time.Hour),
		UpdatedAt:         now.Add(-20 * time.Hour),
	})
	m.classes = upsertSeedClass(m.classes, Class{
		ID:                classID,
		TenantID:          tenantID,
		CenterID:          centerID,
		Name:              "IC3 GS6 Demo 6A",
		Grade:             "6",
		AcademicYear:      "2025-2026",
		Status:            statusActive,
		HomeroomTeacherID: SeedTeacherUserID,
		CreatedAt:         now.Add(-36 * time.Hour),
		UpdatedAt:         now.Add(-20 * time.Hour),
	})

	m.students = upsertSeedStudent(m.students, Student{
		ID:        studentID,
		TenantID:  tenantID,
		CenterID:  centerID,
		ClassID:   classID,
		FullName:  "Seed Student Completed",
		Username:  "seed.completed",
		Status:    statusActive,
		Metrics:   StudentMetrics{AverageScore: &avgStrong, CompletedAssignments: 1, LastActivityAt: &lastActivity},
		CreatedAt: now.Add(-34 * time.Hour),
		UpdatedAt: now.Add(-20 * time.Minute),
	})
	m.students = upsertSeedStudent(m.students, Student{
		ID:        studentInProgressID,
		TenantID:  tenantID,
		CenterID:  centerID,
		ClassID:   classID,
		FullName:  "Seed Student In Progress",
		Username:  "seed.progress",
		Status:    statusActive,
		Metrics:   StudentMetrics{AverageScore: &avgStrong, CompletedAssignments: 0, LastActivityAt: &lastActivity},
		CreatedAt: now.Add(-33 * time.Hour),
		UpdatedAt: now.Add(-15 * time.Minute),
	})
	m.students = upsertSeedStudent(m.students, Student{
		ID:        studentNeedsSupportID,
		TenantID:  tenantID,
		CenterID:  centerID,
		ClassID:   classID,
		FullName:  "Seed Student Needs Support",
		Username:  "seed.support",
		Status:    statusActive,
		Metrics:   StudentMetrics{AverageScore: &avgDeveloping, CompletedAssignments: 0},
		CreatedAt: now.Add(-32 * time.Hour),
		UpdatedAt: now.Add(-10 * time.Minute),
	})

	m.subjects = upsertSeedSubject(m.subjects, Subject{
		ID:        subjectID,
		TenantID:  tenantID,
		Scope:     ContentScope{Type: scopeLevelGlobal},
		Name:      "IC3 Digital Literacy",
		Code:      "IC3-GS6",
		Status:    statusActive,
		CreatedAt: now.Add(-30 * time.Hour),
		UpdatedAt: now.Add(-20 * time.Hour),
	})
	m.levels = upsertSeedLevel(m.levels, Level{
		ID:        levelID,
		TenantID:  tenantID,
		SubjectID: subjectID,
		Name:      "Computing Fundamentals",
		Code:      "CF",
		Order:     1,
		Status:    statusActive,
		CreatedAt: now.Add(-30 * time.Hour),
		UpdatedAt: now.Add(-20 * time.Hour),
	})
	m.topics = upsertSeedTopic(m.topics, Topic{
		ID:        topicID,
		TenantID:  tenantID,
		LevelID:   levelID,
		Name:      "Digital devices",
		Code:      "devices",
		Order:     1,
		Status:    statusActive,
		CreatedAt: now.Add(-30 * time.Hour),
		UpdatedAt: now.Add(-20 * time.Hour),
	})
	m.questions = upsertSeedQuestion(m.questions, Question{
		ID:        questionID,
		TenantID:  tenantID,
		Scope:     ContentScope{Type: scopeLevelGlobal},
		SubjectID: subjectID,
		LevelID:   levelID,
		TopicID:   topicID,
		Type:      QuestionKindSingleChoice,
		Stem:      "Which device is mainly used to input text?",
		Choices: []QuestionChoice{
			{ID: "a", Label: "Keyboard", Correct: true},
			{ID: "b", Label: "Monitor", Correct: false},
			{ID: "c", Label: "Speaker", Correct: false},
		},
		Answer:    "a",
		Metadata:  map[string]any{"source": "seed", "skill": "digital_literacy"},
		Status:    statusActive,
		CreatedAt: now.Add(-28 * time.Hour),
		UpdatedAt: now.Add(-18 * time.Hour),
	})
	m.quizzes = upsertSeedQuiz(m.quizzes, Quiz{
		ID:          quizID,
		TenantID:    tenantID,
		Scope:       ContentScope{Type: scopeLevelGlobal},
		Title:       "IC3 GS6 Seed Quiz",
		Kind:        "practice",
		SubjectID:   subjectID,
		LevelID:     levelID,
		TopicIDs:    []bson.ObjectID{topicID},
		QuestionIDs: []bson.ObjectID{questionID},
		Slides: []any{
			map[string]any{"type": "question", "questionId": questionID.Hex()},
		},
		Settings:    map[string]any{"timeLimitMinutes": 15, "shuffleQuestions": false},
		Result:      map[string]any{"passingPercent": 50},
		Theme:       map[string]any{"accent": "blue"},
		Status:      "published",
		Version:     1,
		PackageHash: "seed-package-hash",
		PublishedAt: &publishedAt,
		CreatedAt:   now.Add(-24 * time.Hour),
		UpdatedAt:   now.Add(-2 * time.Hour),
	})

	m.assignments = upsertSeedAssignment(m.assignments, Assignment{
		ID:            assignmentID,
		TenantID:      tenantID,
		ClassID:       classID,
		QuizID:        quizID,
		SubjectID:     subjectID,
		StudentIDs:    []bson.ObjectID{studentID, studentInProgressID, studentNeedsSupportID},
		DueAt:         &dueAt,
		TeacherNote:   "Seed smoke data for ERG-81 dashboard.",
		Status:        "open",
		AssignedBy:    SeedTeacherUserID,
		RecipientMode: "all",
		CreatedAt:     now.Add(-90 * time.Minute),
		UpdatedAt:     now.Add(-45 * time.Minute),
	})
	m.attempts = upsertSeedAttempt(m.attempts, Attempt{
		ID:           attemptID,
		TenantID:     tenantID,
		AssignmentID: assignmentID,
		QuizID:       quizID,
		StudentID:    studentID,
		PackageID:    quizID.Hex(),
		PackageHash:  "seed-package-hash",
		QuizVersion:  "1",
		Status:       "submitted",
		Answers: map[string]AttemptAnswer{
			questionID.Hex(): {QuestionID: questionID.Hex(), Answer: "a", AnsweredAt: &submittedAt},
		},
		Score:       88,
		MaxScore:    100,
		Percent:     88,
		Passed:      true,
		StartedAt:   now.Add(-45 * time.Minute),
		SubmittedAt: &submittedAt,
		UpdatedAt:   submittedAt,
	})
	m.attempts = upsertSeedAttempt(m.attempts, Attempt{
		ID:           inProgressAttemptID,
		TenantID:     tenantID,
		AssignmentID: assignmentID,
		QuizID:       quizID,
		StudentID:    studentInProgressID,
		PackageID:    quizID.Hex(),
		PackageHash:  "seed-package-hash",
		QuizVersion:  "1",
		Status:       "in_progress",
		Answers:      map[string]AttemptAnswer{},
		MaxScore:     100,
		StartedAt:    now.Add(-20 * time.Minute),
		UpdatedAt:    now.Add(-15 * time.Minute),
	})

	return LMSSeedFixture{
		TenantID:     tenantID,
		Admin:        Actor{UserID: SeedAdminUserID, Roles: []string{"lms_admin"}},
		Manager:      Actor{UserID: SeedManagerUserID, Roles: []string{"manager"}},
		Teacher:      Actor{UserID: SeedTeacherUserID, Roles: []string{"teacher"}},
		Student:      Actor{UserID: studentID.Hex(), Roles: []string{"student"}},
		SchoolID:     schoolID.Hex(),
		CenterID:     centerID.Hex(),
		ClassID:      classID.Hex(),
		StudentID:    studentID.Hex(),
		SubjectID:    subjectID.Hex(),
		LevelID:      levelID.Hex(),
		TopicID:      topicID.Hex(),
		QuestionID:   questionID.Hex(),
		QuizID:       quizID.Hex(),
		AssignmentID: assignmentID.Hex(),
		AttemptID:    attemptID.Hex(),
	}, nil
}

func mustSeedObjectID(hex string) bson.ObjectID {
	oid, err := bson.ObjectIDFromHex(hex)
	if err != nil {
		panic(err)
	}
	return oid
}

func upsertSeedCenter(items []Center, item Center) []Center {
	for i := range items {
		if items[i].TenantID == item.TenantID && items[i].ID == item.ID {
			items[i] = item
			return items
		}
	}
	return append(items, item)
}

func upsertSeedClass(items []Class, item Class) []Class {
	for i := range items {
		if items[i].TenantID == item.TenantID && items[i].ID == item.ID {
			items[i] = item
			return items
		}
	}
	return append(items, item)
}

func upsertSeedStudent(items []Student, item Student) []Student {
	for i := range items {
		if items[i].TenantID == item.TenantID && items[i].ID == item.ID {
			items[i] = item
			return items
		}
	}
	return append(items, item)
}

func upsertSeedSubject(items []Subject, item Subject) []Subject {
	for i := range items {
		if items[i].TenantID == item.TenantID && items[i].ID == item.ID {
			items[i] = item
			return items
		}
	}
	return append(items, item)
}

func upsertSeedLevel(items []Level, item Level) []Level {
	for i := range items {
		if items[i].TenantID == item.TenantID && items[i].ID == item.ID {
			items[i] = item
			return items
		}
	}
	return append(items, item)
}

func upsertSeedTopic(items []Topic, item Topic) []Topic {
	for i := range items {
		if items[i].TenantID == item.TenantID && items[i].ID == item.ID {
			items[i] = item
			return items
		}
	}
	return append(items, item)
}

func upsertSeedQuestion(items []Question, item Question) []Question {
	for i := range items {
		if items[i].TenantID == item.TenantID && items[i].ID == item.ID {
			items[i] = item
			return items
		}
	}
	return append(items, item)
}

func upsertSeedQuiz(items []Quiz, item Quiz) []Quiz {
	for i := range items {
		if items[i].TenantID == item.TenantID && items[i].ID == item.ID {
			items[i] = item
			return items
		}
	}
	return append(items, item)
}

func upsertSeedAssignment(items []Assignment, item Assignment) []Assignment {
	for i := range items {
		if items[i].TenantID == item.TenantID && items[i].ID == item.ID {
			items[i] = item
			return items
		}
	}
	return append(items, item)
}

func upsertSeedAttempt(items []Attempt, item Attempt) []Attempt {
	for i := range items {
		if items[i].TenantID == item.TenantID && items[i].ID == item.ID {
			items[i] = item
			return items
		}
	}
	return append(items, item)
}
