package service

import (
	"context"
	"testing"
	"time"
)

func TestCreateStudentPersistsProfessionalLMSProfile(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	svc := NewService(repo, nil)
	tenantID := "tenant-lms"

	center := &Center{
		TenantID: tenantID,
		Type:     educationUnitTypeSchool,
		Name:     "ERG High School",
		Code:     "ERG-HS",
	}
	if err := repo.CreateCenter(ctx, center); err != nil {
		t.Fatalf("create center: %v", err)
	}
	class := &Class{
		TenantID:     tenantID,
		CenterID:     center.ID,
		Name:         "10A1",
		Grade:        "10",
		AcademicYear: "2026-2027",
	}
	if err := repo.CreateClass(ctx, class); err != nil {
		t.Fatalf("create class: %v", err)
	}

	birthday := time.Date(2011, 9, 2, 0, 0, 0, 0, time.UTC)
	enrollmentDate := time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC)
	resp, err := svc.CreateStudent(ctx, tenantID, Actor{Roles: []string{"admin"}}, CreateStudentRequestDTO{
		FullName:           "Nguyen Van An",
		ClassID:            class.ID.Hex(),
		StudentCode:        "ERG-2026-0001",
		Email:              "an.nguyen@example.edu.vn",
		Gender:             "male",
		Birthday:           &birthday,
		Phone:              "0909000001",
		Address:            "12 Nguyen Trai, Ho Chi Minh City",
		ParentName:         "Nguyen Van B",
		ParentPhone:        "0909000002",
		ParentEmail:        "parent@example.com",
		ParentRelationship: "father",
		EnrollmentDate:     &enrollmentDate,
		Note:               "Needs math support",
	})
	if err != nil {
		t.Fatalf("create student: %v", err)
	}

	student := resp.Student
	if student.StudentCode != "ERG-2026-0001" || student.Email != "an.nguyen@example.edu.vn" || student.Gender != "male" {
		t.Fatalf("missing identity profile fields: %+v", student)
	}
	if student.ParentName != "Nguyen Van B" || student.ParentPhone != "0909000002" || student.ParentRelationship != "father" {
		t.Fatalf("missing guardian profile fields: %+v", student)
	}
	if student.CenterCode != "ERG-HS" || student.CenterType != educationUnitTypeSchool {
		t.Fatalf("missing school fields: %+v", student)
	}
	if student.Grade != "10" || student.AcademicYear != "2026-2027" {
		t.Fatalf("missing class fields: %+v", student)
	}
	if student.EnrollmentDate == nil || !student.EnrollmentDate.Equal(enrollmentDate) {
		t.Fatalf("missing enrollment date: %+v", student)
	}
}
