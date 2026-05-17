package service

import (
	"context"
	"errors"
	"testing"
)

func TestScopeListsSystemSchoolCenterAndClassOptions(t *testing.T) {
	ctx := context.Background()
	svc := NewService(newMemoryRepository(), nil)
	tenantID := "tenant-a"
	admin := Actor{UserID: "admin", Roles: []string{"lms_admin"}}

	school, err := svc.CreateEducationUnit(ctx, tenantID, admin, CreateEducationUnitRequestDTO{
		Type: educationUnitTypeSchool,
		Name: "ERG School",
		Code: "SCH",
	})
	if err != nil {
		t.Fatalf("create school: %v", err)
	}
	center, err := svc.CreateEducationUnit(ctx, tenantID, admin, CreateEducationUnitRequestDTO{
		Type: educationUnitTypeCenter,
		Name: "ERG Center",
		Code: "CTR",
	})
	if err != nil {
		t.Fatalf("create center: %v", err)
	}
	if _, err := svc.CreateClass(ctx, tenantID, admin, CreateClassRequestDTO{CenterID: school.ID, Name: "School 6A", Grade: "6"}); err != nil {
		t.Fatalf("create school class: %v", err)
	}
	if _, err := svc.CreateClass(ctx, tenantID, admin, CreateClassRequestDTO{CenterID: center.ID, Name: "Center 7A", Grade: "7"}); err != nil {
		t.Fatalf("create center class: %v", err)
	}

	got, err := svc.Scope(ctx, tenantID, admin)
	if err != nil {
		t.Fatalf("scope: %v", err)
	}
	if got.CurrentScope.Type != scopeLevelSystem || got.CurrentScope.Badge == "" || got.CurrentScope.Icon == "" || got.CurrentScope.Description == "" {
		t.Fatalf("current scope is not decorated as system: %+v", got.CurrentScope)
	}
	if got.CurrentScope.CenterName != "Hệ thống ERG" {
		t.Fatalf("current scope center name = %q, want Hệ thống ERG", got.CurrentScope.CenterName)
	}
	assertScopeOption(t, got.AvailableScopes, scopeLevelGlobal, scopeLevelSystem)
	assertScopeOption(t, got.AvailableScopes, scopeLevelCenter, educationUnitTypeSchool)
	assertScopeOption(t, got.AvailableScopes, scopeLevelCenter, educationUnitTypeCenter)
	assertScopeOption(t, got.AvailableScopes, scopeLevelClass, educationUnitTypeSchool)
	assertScopeOption(t, got.AvailableScopes, scopeLevelClass, educationUnitTypeCenter)
}

func TestCreateEducationUnitAcceptsSchoolAndCenter(t *testing.T) {
	ctx := context.Background()
	svc := NewService(newMemoryRepository(), nil)
	admin := Actor{UserID: "admin", Roles: []string{"lms_admin"}}

	got, err := svc.CreateEducationUnit(ctx, "tenant-a", admin, CreateEducationUnitRequestDTO{
		Type: educationUnitTypeSchool,
		Name: "Popup School",
		Code: "POP-SCH",
	})
	if err != nil {
		t.Fatalf("create education unit: %v", err)
	}
	if got.ID == "" || got.Type != educationUnitTypeSchool {
		t.Fatalf("unexpected created unit: %+v", got)
	}
}

func TestCreateEducationUnitRejectsInvalidType(t *testing.T) {
	ctx := context.Background()
	svc := NewService(newMemoryRepository(), nil)
	admin := Actor{UserID: "admin", Roles: []string{"lms_admin"}}

	_, err := svc.CreateEducationUnit(ctx, "tenant-a", admin, CreateEducationUnitRequestDTO{
		Type: "system",
		Name: "Invalid",
		Code: "BAD",
	})
	if !errors.Is(err, errInvalidEducationUnitType) {
		t.Fatalf("error = %v, want %v", err, errInvalidEducationUnitType)
	}
}

func TestListEducationUnitClassesIncludesSchoolAndCenterClasses(t *testing.T) {
	ctx := context.Background()
	svc := NewService(newMemoryRepository(), nil)
	tenantID := "tenant-a"
	admin := Actor{UserID: "admin", Roles: []string{"lms_admin"}}

	school, _ := svc.CreateEducationUnit(ctx, tenantID, admin, CreateEducationUnitRequestDTO{Type: educationUnitTypeSchool, Name: "School", Code: "S"})
	center, _ := svc.CreateEducationUnit(ctx, tenantID, admin, CreateEducationUnitRequestDTO{Type: educationUnitTypeCenter, Name: "Center", Code: "C"})
	schoolClass, _ := svc.CreateClass(ctx, tenantID, admin, CreateClassRequestDTO{CenterID: school.ID, Name: "School Class", Grade: "6"})
	centerClass, _ := svc.CreateClass(ctx, tenantID, admin, CreateClassRequestDTO{CenterID: center.ID, Name: "Center Class", Grade: "7"})

	schoolClasses, err := svc.ListEducationUnitClasses(ctx, tenantID, admin, school.ID, ClassListRequestDTO{Page: 1, Limit: 20})
	if err != nil {
		t.Fatalf("list school classes: %v", err)
	}
	if len(schoolClasses.Items) != 1 || schoolClasses.Items[0].ID != schoolClass.ID {
		t.Fatalf("unexpected school classes: %+v", schoolClasses.Items)
	}
	centerClasses, err := svc.ListEducationUnitClasses(ctx, tenantID, admin, center.ID, ClassListRequestDTO{Page: 1, Limit: 20})
	if err != nil {
		t.Fatalf("list center classes: %v", err)
	}
	if len(centerClasses.Items) != 1 || centerClasses.Items[0].ID != centerClass.ID {
		t.Fatalf("unexpected center classes: %+v", centerClasses.Items)
	}
}

func assertScopeOption(t *testing.T, scopes []ManagementScopeDTO, level, scopeType string) {
	t.Helper()
	for _, scope := range scopes {
		if scope.Level == level && scope.Type == scopeType && scope.Badge != "" && scope.Icon != "" && scope.Description != "" {
			return
		}
	}
	t.Fatalf("scope option level=%q type=%q with metadata not found in %+v", level, scopeType, scopes)
}
