package policy

import (
	"slices"
	"testing"

	"erg.ninja/pkg/auth"
)

func TestEnterprisePermissionNamespaces(t *testing.T) {
	perms := EnterprisePermissionDefinitions()
	requiredPrefixes := []string{"hoclieu.", "lms.", "elearning.", "rbac."}
	for _, prefix := range requiredPrefixes {
		if !hasPermissionPrefix(perms, prefix) {
			t.Fatalf("missing permission namespace %q in enterprise contract", prefix)
		}
	}
	if !hasPermission(perms, PermissionAuditRead) {
		t.Fatalf("missing %q permission", PermissionAuditRead)
	}
}

func TestEnterpriseRoleTaxonomyCoversDomains(t *testing.T) {
	defs := EnterpriseRoleDefinitions()
	for _, domain := range []string{"system", "hoclieu", "lms", "elearning"} {
		if !hasRoleDomain(defs, domain) {
			t.Fatalf("missing role domain %q in enterprise role taxonomy", domain)
		}
	}
}

func TestDecideAllowsRolePermissionWithPortal(t *testing.T) {
	decision := Decide(Request{
		Subject:  Subject{ID: "teacher-1", Roles: []string{RoleLMSTeacher}},
		Action:   "read",
		Resource: Resource{Namespace: "lms", Type: "grade"},
		Scope:    Scope{Portal: PortalLMS},
	})

	if !decision.Allowed {
		t.Fatalf("decision = %#v, want allowed", decision)
	}
	if decision.RequiredPermission != PermissionLMSGradeRead {
		t.Fatalf("required permission = %q, want %q", decision.RequiredPermission, PermissionLMSGradeRead)
	}
}

func TestDecideDenyOverrideWinsOverWildcardGrant(t *testing.T) {
	decision := Decide(Request{
		Subject: Subject{
			ID:                "admin-1",
			Permissions:       []string{PermissionLMSAll},
			DeniedPermissions: []string{PermissionLMSGradeUpdate},
			Portals:           []Portal{PortalLMS},
		},
		Resource: Resource{Permission: PermissionLMSGradeUpdate},
		Scope:    Scope{Portal: PortalLMS},
	})

	if decision.Allowed {
		t.Fatalf("decision = %#v, want denied", decision)
	}
	if decision.Reason != "deny override" {
		t.Fatalf("reason = %q, want deny override", decision.Reason)
	}
}

func TestDecideAcceptsLegacyPermissionAlias(t *testing.T) {
	decision := Decide(Request{
		Subject:  Subject{ID: "admin-1", Permissions: []string{PermissionRBACRoleRead}},
		Resource: Resource{Permission: "roles.read"},
	})

	if !decision.Allowed {
		t.Fatalf("decision = %#v, want allowed through legacy alias", decision)
	}
}

func TestDecideRejectsWrongPortal(t *testing.T) {
	decision := Decide(Request{
		Subject:  Subject{ID: "user-1", Permissions: []string{PermissionLMSCourseRead}, Portals: []Portal{PortalHocLieu}},
		Resource: Resource{Permission: PermissionLMSCourseRead},
		Scope:    Scope{Portal: PortalLMS},
	})

	if decision.Allowed {
		t.Fatalf("decision = %#v, want denied", decision)
	}
	if decision.Reason != "portal denied" {
		t.Fatalf("reason = %q, want portal denied", decision.Reason)
	}
}

func TestSubjectFromClaimsInfersPortalsAndDenies(t *testing.T) {
	subject := SubjectFromClaims(&auth.JWTClaims{
		UserID:            "user-1",
		Roles:             []string{RoleHocLieuEditor},
		Portal:            "cms",
		DeniedPermissions: []string{PermissionHocLieuContentDelete},
	})

	if subject.ID != "user-1" {
		t.Fatalf("subject ID = %q, want user-1", subject.ID)
	}
	if !slices.Contains(subject.Portals, PortalHocLieu) || !slices.Contains(subject.Portals, PortalCMS) {
		t.Fatalf("portals = %v, want hoclieu and cms", subject.Portals)
	}
	if !slices.Contains(subject.DeniedPermissions, PermissionHocLieuContentDelete) {
		t.Fatalf("denied permissions = %v, want %q", subject.DeniedPermissions, PermissionHocLieuContentDelete)
	}
}

func hasPermissionPrefix(perms []PermissionDefinition, prefix string) bool {
	for _, perm := range perms {
		if len(perm.Name) >= len(prefix) && perm.Name[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

func hasPermission(perms []PermissionDefinition, name string) bool {
	for _, perm := range perms {
		if perm.Name == name {
			return true
		}
	}
	return false
}

func hasRoleDomain(defs []RoleDefinition, domain string) bool {
	for _, def := range defs {
		if def.Domain == domain {
			return true
		}
	}
	return false
}
