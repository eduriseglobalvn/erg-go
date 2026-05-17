package service

import (
	"context"
	"net/url"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestCreateStudentWithoutAuthRepoDoesNotReturnUnbackedPassword(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	svc := NewService(repo, nil)
	tenantID := "tenant-lms"
	center, class := seedTestRoster(t, ctx, repo, tenantID)

	resp, err := svc.CreateStudent(ctx, tenantID, Actor{Roles: []string{"admin"}}, CreateStudentRequestDTO{
		FullName:    "Tran Thi Binh",
		ClassID:     class.ID.Hex(),
		StudentCode: "ERG-2026-0002",
		Username:    "binh.tran",
		Password:    "BinhTran2026!",
	})
	if err != nil {
		t.Fatalf("create student: %v", err)
	}
	if resp.TempPassword != "" {
		t.Fatalf("expected no temp password without auth provisioning, got %q", resp.TempPassword)
	}
	if resp.Student.CenterID != center.ID.Hex() || resp.Student.ClassID != class.ID.Hex() {
		t.Fatalf("unexpected roster scope: %+v", resp.Student)
	}
}

func TestExportReportRosterIncludesStudentProfileFields(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	svc := NewService(repo, nil)
	tenantID := "tenant-lms"
	_, class := seedTestRoster(t, ctx, repo, tenantID)
	_, err := svc.CreateStudent(ctx, tenantID, Actor{Roles: []string{"admin"}}, CreateStudentRequestDTO{
		FullName:    "Le Van Cuong",
		ClassID:     class.ID.Hex(),
		StudentCode: "ERG-2026-0003",
		Username:    "cuong.le",
		Password:    "CuongLe2026!",
		ParentName:  "Le Parent",
		ParentPhone: "0909000003",
	})
	if err != nil {
		t.Fatalf("create student: %v", err)
	}

	resp, err := svc.ExportReport(ctx, tenantID, Actor{Roles: []string{"admin"}}, "roster", "", class.ID.Hex())
	if err != nil {
		t.Fatalf("export roster: %v", err)
	}
	decoded, err := url.QueryUnescape(strings.TrimPrefix(resp.DownloadURL, "data:text/csv;charset=utf-8,"))
	if err != nil {
		t.Fatalf("decode csv: %v", err)
	}
	if !strings.Contains(decoded, "studentCode") || !strings.Contains(decoded, "parentName") || !strings.Contains(decoded, "ERG-2026-0003") || !strings.Contains(decoded, "Le Parent") {
		t.Fatalf("roster export missing profile fields:\n%s", decoded)
	}
}

func TestCreateQuestionRejectsUnsafePayload(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	svc := NewService(repo, nil)
	tenantID := "tenant-lms"
	subjectID := bson.NewObjectID()
	levelID := bson.NewObjectID()
	repo.memory.subjects = append(repo.memory.subjects, Subject{ID: subjectID, TenantID: tenantID, Scope: ContentScope{Type: "global"}, Name: "Math", Code: "math", Status: statusActive})
	repo.memory.levels = append(repo.memory.levels, Level{ID: levelID, TenantID: tenantID, SubjectID: subjectID, Name: "Grade 10", Code: "g10", Status: statusActive})

	_, err := svc.CreateQuestion(ctx, tenantID, Actor{Roles: []string{"admin"}}, CreateQuestionRequestDTO{
		Scope:     ContentScopeDTO{Type: "global"},
		SubjectID: subjectID.Hex(),
		LevelID:   levelID.Hex(),
		Kind:      QuestionKindSingleChoice,
		Stem:      "Pick one",
		Choices: []QuestionChoiceDTO{
			{ID: "a", Label: "A", Correct: true},
			{ID: "b", Label: "B", Correct: true},
		},
	})
	if err == nil {
		t.Fatal("expected invalid content payload for single-choice question with multiple correct choices")
	}
}

func seedTestRoster(t *testing.T, ctx context.Context, repo *Repository, tenantID string) (*Center, *Class) {
	t.Helper()
	center := &Center{TenantID: tenantID, Type: educationUnitTypeSchool, Name: "ERG High School", Code: "ERG-HS"}
	if err := repo.CreateCenter(ctx, center); err != nil {
		t.Fatalf("create center: %v", err)
	}
	class := &Class{TenantID: tenantID, CenterID: center.ID, Name: "10A1", Grade: "10", AcademicYear: "2026-2027"}
	if err := repo.CreateClass(ctx, class); err != nil {
		t.Fatalf("create class: %v", err)
	}
	return center, class
}
