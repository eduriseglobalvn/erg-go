package service

import (
	"context"
	"errors"
	"testing"
)

func TestNormalizeAccessPolicyHierarchyRules(t *testing.T) {
	ctx := context.Background()
	const tenantID = "default"
	svc := NewService(newMemoryRepository(), nil)

	center, err := svc.CreateEducationUnit(ctx, tenantID, Actor{UserID: "admin", Roles: []string{"admin"}}, CreateEducationUnitRequestDTO{
		Name: "ERG Bình Phú",
		Code: "ERG-BINH-PHU",
		Type: educationUnitTypeCenter,
	})
	if err != nil {
		t.Fatalf("create center: %v", err)
	}
	school, err := svc.CreateEducationUnit(ctx, tenantID, Actor{UserID: "admin", Roles: []string{"admin"}}, CreateEducationUnitRequestDTO{
		Name: "THCS CÁT LÁI",
		Code: "THCS-CAT-LAI",
		Type: educationUnitTypeSchool,
	})
	if err != nil {
		t.Fatalf("create school: %v", err)
	}

	tests := []struct {
		name    string
		policy  UserAccessPolicyDTO
		wantErr bool
	}{
		{
			name: "system admin is valid at system scope",
			policy: UserAccessPolicyDTO{
				ScopeType: accessScopeSystem,
				RoleGroup: accessRoleGroups[0].ID,
				Modules:   []string{"lms"},
			},
		},
		{
			name: "center admin is valid at center scope",
			policy: UserAccessPolicyDTO{
				ScopeType: accessScopeCenter,
				ScopeID:   center.ID,
				RoleGroup: "center_admin",
				Modules:   []string{"lms", "media"},
			},
		},
		{
			name: "teacher is valid at school scope",
			policy: UserAccessPolicyDTO{
				ScopeType: accessScopeSchool,
				ScopeID:   school.ID,
				RoleGroup: "teacher",
				Modules:   []string{"lms"},
			},
		},
		{
			name: "center role is rejected at school scope",
			policy: UserAccessPolicyDTO{
				ScopeType: accessScopeSchool,
				ScopeID:   school.ID,
				RoleGroup: "center_admin",
				Modules:   []string{"lms"},
			},
			wantErr: true,
		},
		{
			name: "school scope cannot point to a center",
			policy: UserAccessPolicyDTO{
				ScopeType: accessScopeSchool,
				ScopeID:   center.ID,
				RoleGroup: "teacher",
				Modules:   []string{"lms"},
			},
			wantErr: true,
		},
		{
			name: "policy requires at least one allowed module",
			policy: UserAccessPolicyDTO{
				ScopeType: accessScopeSystem,
				RoleGroup: "system_admin",
				Modules:   []string{"unknown"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.normalizeAccessPolicy(ctx, tenantID, tt.policy)
			if tt.wantErr {
				if !errors.Is(err, errInvalidAccessPolicy) {
					t.Fatalf("expected invalid policy error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected policy to normalize, got %v", err)
			}
		})
	}
}

func TestAccessManagementOptionsIncludeHierarchyScopes(t *testing.T) {
	ctx := context.Background()
	const tenantID = "default"
	svc := NewService(newMemoryRepository(), nil)

	_, err := svc.CreateEducationUnit(ctx, tenantID, Actor{UserID: "admin", Roles: []string{"admin"}}, CreateEducationUnitRequestDTO{
		Name: "ERG Bình Phú",
		Code: "ERG-BINH-PHU",
		Type: educationUnitTypeCenter,
	})
	if err != nil {
		t.Fatalf("create center: %v", err)
	}
	if _, err := svc.CreateEducationUnit(ctx, tenantID, Actor{UserID: "admin", Roles: []string{"admin"}}, CreateEducationUnitRequestDTO{
		Name: "THCS TRƯƠNG CÔNG ĐỊNH",
		Code: "THCS-TRUONG-CONG-DINH",
		Type: educationUnitTypeSchool,
	}); err != nil {
		t.Fatalf("create school: %v", err)
	}

	options, err := svc.AccessManagementOptions(ctx, tenantID, Actor{UserID: "admin", Roles: []string{"admin"}})
	if err != nil {
		t.Fatalf("options: %v", err)
	}
	seen := map[string]bool{}
	for _, scope := range options.Scopes {
		seen[scope.ScopeType] = true
	}
	for _, scopeType := range []string{accessScopeSystem, accessScopeCenter, accessScopeSchool} {
		if !seen[scopeType] {
			t.Fatalf("expected scope type %q in options, got %#v", scopeType, options.Scopes)
		}
	}
}

func TestCenterManagerAccessManagementIsLimitedToCenterHierarchy(t *testing.T) {
	ctx := context.Background()
	const tenantID = "default"
	const managerID = "center-manager"
	svc := NewService(newMemoryRepository(), nil)

	center, err := svc.CreateEducationUnit(ctx, tenantID, Actor{UserID: "admin", Roles: []string{"admin"}}, CreateEducationUnitRequestDTO{
		Name:          "ERG Bình Phú",
		Code:          "ERG-BINH-PHU",
		Type:          educationUnitTypeCenter,
		ManagerUserID: managerID,
	})
	if err != nil {
		t.Fatalf("create center: %v", err)
	}
	school, err := svc.CreateEducationUnit(ctx, tenantID, Actor{UserID: "admin", Roles: []string{"admin"}}, CreateEducationUnitRequestDTO{
		Name:     "THCS CÁT LÁI",
		Code:     "THCS-CAT-LAI",
		Type:     educationUnitTypeSchool,
		ParentID: center.ID,
	})
	if err != nil {
		t.Fatalf("create school: %v", err)
	}
	otherCenter, err := svc.CreateEducationUnit(ctx, tenantID, Actor{UserID: "admin", Roles: []string{"admin"}}, CreateEducationUnitRequestDTO{
		Name: "ERG Khác",
		Code: "ERG-OTHER",
		Type: educationUnitTypeCenter,
	})
	if err != nil {
		t.Fatalf("create other center: %v", err)
	}

	actor := Actor{UserID: managerID, Roles: []string{"center_admin"}}
	options, err := svc.AccessManagementOptions(ctx, tenantID, actor)
	if err != nil {
		t.Fatalf("options: %v", err)
	}
	if len(options.Scopes) != 2 {
		t.Fatalf("expected only managed center and child school, got %#v", options.Scopes)
	}
	for _, scope := range options.Scopes {
		if scope.ScopeType == accessScopeSystem || scope.ScopeID == otherCenter.ID {
			t.Fatalf("center manager received out-of-scope option: %#v", scope)
		}
	}

	allowed := UserAccessPolicyDTO{ScopeType: accessScopeSchool, ScopeID: school.ID, RoleGroup: "teacher", Modules: []string{"lms"}}
	normalized, err := svc.normalizeAccessPolicy(ctx, tenantID, allowed)
	if err != nil {
		t.Fatalf("normalize allowed policy: %v", err)
	}
	if !svc.canAssignAccessPolicy(ctx, tenantID, actor, center.ID, normalized) {
		t.Fatalf("expected center manager to assign child school policy")
	}

	blocked := UserAccessPolicyDTO{ScopeType: accessScopeCenter, ScopeID: otherCenter.ID, RoleGroup: "center_admin", Modules: []string{"lms"}}
	normalized, err = svc.normalizeAccessPolicy(ctx, tenantID, blocked)
	if err != nil {
		t.Fatalf("normalize blocked policy: %v", err)
	}
	if svc.canAssignAccessPolicy(ctx, tenantID, actor, center.ID, normalized) {
		t.Fatalf("expected center manager to be blocked outside hierarchy")
	}
}
