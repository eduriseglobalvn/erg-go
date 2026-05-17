package service

import (
	"context"
	"testing"
)

func TestSeedDemoDataCoversScopeDashboardAndQuizPackage(t *testing.T) {
	ctx := context.Background()
	svc := NewService(newMemoryRepository(), nil)
	seed, err := svc.SeedDemoData(ctx, "tenant-seed")
	if err != nil {
		t.Fatalf("seed demo data: %v", err)
	}

	scope, err := svc.Scope(ctx, seed.TenantID, seed.Admin)
	if err != nil {
		t.Fatalf("scope: %v", err)
	}
	assertScopeOption(t, scope.AvailableScopes, scopeLevelGlobal, scopeLevelSystem)
	assertScopeOption(t, scope.AvailableScopes, scopeLevelCenter, educationUnitTypeSchool)
	assertScopeOption(t, scope.AvailableScopes, scopeLevelCenter, educationUnitTypeCenter)
	assertScopeOption(t, scope.AvailableScopes, scopeLevelClass, educationUnitTypeCenter)

	overview, err := svc.DashboardOverview(ctx, seed.TenantID, seed.Admin, DashboardScopeRequestDTO{CenterID: seed.CenterID})
	if err != nil {
		t.Fatalf("dashboard overview: %v", err)
	}
	if overview.Scope.StudentCount != 3 || overview.Metrics.TotalAssignments != 1 || overview.Metrics.OpenAssignments != 1 {
		t.Fatalf("unexpected dashboard overview seed contract: %+v", overview)
	}
	if overview.Metrics.Completed == 0 || overview.Metrics.CompletionRate <= 0 {
		t.Fatalf("expected dashboard completion data, got %+v", overview.Metrics)
	}

	active, err := svc.ActiveAssignments(ctx, seed.TenantID, seed.Admin, DashboardScopeRequestDTO{CenterID: seed.CenterID})
	if err != nil {
		t.Fatalf("active assignments: %v", err)
	}
	if len(active.Items) != 1 || active.Items[0].TotalStudents != 3 || active.Items[0].Submitted != 1 || active.Items[0].InProgress != 1 {
		t.Fatalf("unexpected active assignment seed contract: %+v", active.Items)
	}

	pkg, err := svc.QuizPackage(ctx, seed.TenantID, seed.QuizID)
	if err != nil {
		t.Fatalf("quiz package: %v", err)
	}
	if pkg.PackageHash == "" || pkg.Quiz.Quiz.ID != seed.QuizID || len(pkg.Quiz.Slides) == 0 {
		t.Fatalf("unexpected quiz package seed contract: %+v", pkg)
	}
}

func TestSeedDemoDataCoversStudentAssignmentAndScoreSmoke(t *testing.T) {
	ctx := context.Background()
	svc := NewService(newMemoryRepository(), nil)
	seed, err := svc.SeedDemoData(ctx, "tenant-seed")
	if err != nil {
		t.Fatalf("seed demo data: %v", err)
	}

	assignments, err := svc.StudentAssignments(ctx, seed.TenantID, seed.Student, "")
	if err != nil {
		t.Fatalf("student assignments: %v", err)
	}
	if len(assignments.Items) != 1 || assignments.Items[0].ID != seed.AssignmentID {
		t.Fatalf("unexpected student assignments: %+v", assignments.Items)
	}

	scores, err := svc.StudentScores(ctx, seed.TenantID, seed.Student, seed.SubjectID)
	if err != nil {
		t.Fatalf("student scores: %v", err)
	}
	if len(scores.Items) != 1 || scores.Items[0].BestScore <= 0 || len(scores.Items[0].RecentAttemptsTop3) == 0 {
		t.Fatalf("unexpected student scores: %+v", scores.Items)
	}

	progress, err := svc.QuizStudentProgress(ctx, seed.TenantID, seed.QuizID, seed.ClassID, "")
	if err != nil {
		t.Fatalf("quiz student progress: %v", err)
	}
	if len(progress.Items) != 3 {
		t.Fatalf("expected quiz progress for 3 students, got %+v", progress.Items)
	}
}

func TestSeedDemoDataCoversQuestionBankCatalogSmoke(t *testing.T) {
	ctx := context.Background()
	svc := NewService(newMemoryRepository(), nil)
	seed, err := svc.SeedDemoData(ctx, "tenant-seed")
	if err != nil {
		t.Fatalf("seed demo data: %v", err)
	}

	subjects, err := svc.ListSubjects(ctx, seed.TenantID, seed.Admin, scopeLevelGlobal, "")
	if err != nil {
		t.Fatalf("list subjects: %v", err)
	}
	if len(subjects.Items) != 1 || subjects.Items[0].ID != seed.SubjectID {
		t.Fatalf("unexpected seeded subjects: %+v", subjects.Items)
	}

	categories, err := svc.ListQuestionBankCategories(ctx, seed.TenantID, seed.SubjectID, "")
	if err != nil {
		t.Fatalf("list question bank categories: %v", err)
	}
	if len(categories.Items) != 1 || categories.Items[0].ID != seed.LevelID {
		t.Fatalf("unexpected seeded categories: %+v", categories.Items)
	}

	questions, err := svc.ListQuestions(ctx, seed.TenantID, seed.Admin, QuestionListRequestDTO{SubjectID: seed.SubjectID, LevelID: seed.LevelID, Limit: 20})
	if err != nil {
		t.Fatalf("list questions: %v", err)
	}
	if len(questions.Items) != 1 || questions.Items[0].ID != seed.QuestionID {
		t.Fatalf("unexpected seeded questions: %+v", questions.Items)
	}

	quizzes, err := svc.ListQuizzes(ctx, seed.TenantID, seed.Admin, QuizListRequestDTO{SubjectID: seed.SubjectID, Limit: 20})
	if err != nil {
		t.Fatalf("list quizzes: %v", err)
	}
	if len(quizzes.Items) != 1 || quizzes.Items[0].ID != seed.QuizID {
		t.Fatalf("unexpected seeded quizzes: %+v", quizzes.Items)
	}
}
