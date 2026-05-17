package elearning

import (
	"context"
	"testing"

	"erg.ninja/internal/modules/elearning/api/dto"
)

func TestStudentDashboardContractUsesCurrentActor(t *testing.T) {
	result, err := (&Service{}).StudentDashboard(context.Background(), "tenant-1", StudentActor{
		UserID: "student-1",
		Roles:  []string{"student"},
	})
	if err != nil {
		t.Fatalf("StudentDashboard returned error: %v", err)
	}
	if result.Student.ID != "student-1" || result.Student.TenantID != "tenant-1" {
		t.Fatalf("unexpected student contract: %+v", result.Student)
	}
	if result.Assignments == nil || result.Notifications == nil {
		t.Fatal("expected list fields to be non-nil for frontend contract")
	}
}

func TestCreateStudentDiscussionContractUsesActorAsAuthor(t *testing.T) {
	result, err := (&Service{}).CreateStudentDiscussion(context.Background(), "tenant-1", StudentActor{UserID: "student-1"}, dtoCreateDiscussion("Question"))
	if err != nil {
		t.Fatalf("CreateStudentDiscussion returned error: %v", err)
	}
	if result.StudentID != "student-1" || result.AuthorID != "student-1" {
		t.Fatalf("unexpected discussion author contract: %+v", result)
	}
}

func dtoCreateDiscussion(title string) dto.CreateStudentDiscussionRequest {
	return dto.CreateStudentDiscussionRequest{Title: title, Content: "Body"}
}
