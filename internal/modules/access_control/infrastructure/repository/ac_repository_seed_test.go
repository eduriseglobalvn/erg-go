package repository

import (
	"slices"
	"testing"

	"erg.ninja/internal/modules/access_control/domain/entity"
	"erg.ninja/internal/modules/access_control/domain/policy"
)

func TestDefaultSeedIncludesEnterpriseNamespaces(t *testing.T) {
	perms := defaultPermissions()
	for _, required := range []string{
		policy.PermissionHocLieuAll,
		policy.PermissionLMSAll,
		policy.PermissionElearningAll,
		policy.PermissionRBACAll,
		policy.PermissionAuditRead,
		policy.PermissionMediaAll,
		policy.PermissionMediaUpload,
	} {
		if !seedHasPermission(perms, required) {
			t.Fatalf("defaultPermissions missing %q", required)
		}
	}
}

func TestDefaultSeedIncludesEnterpriseRoleTaxonomy(t *testing.T) {
	roles := enterpriseSeedRoles(defaultPermissions())
	roleNames := make([]string, 0, len(roles))
	for _, role := range roles {
		roleNames = append(roleNames, role.Name)
	}

	for _, required := range []string{
		policy.RoleSystemSuperAdmin,
		policy.RoleSystemAdmin,
		policy.RoleERGSuperAdmin,
		policy.RoleMediaManager,
		policy.RoleHocLieuAdmin,
		policy.RoleLMSAdmin,
		policy.RoleElearningAdmin,
	} {
		if !slices.Contains(roleNames, required) {
			t.Fatalf("enterpriseSeedRoles missing %q; got %v", required, roleNames)
		}
	}
}

func TestSystemSuperAdminSeedExpandsWildcard(t *testing.T) {
	roles := enterpriseSeedRoles(defaultPermissions())
	var superAdmin entities.Role
	for _, role := range roles {
		if role.Name == policy.RoleSystemSuperAdmin {
			superAdmin = role
			break
		}
	}

	if len(superAdmin.Permissions) == 0 {
		t.Fatal("system.super_admin seed permissions empty")
	}
	if slices.Contains(superAdmin.Permissions, policy.PermissionAll) {
		t.Fatalf("system.super_admin seed should expand wildcard to concrete permissions, got %v", superAdmin.Permissions)
	}
	if !slices.Contains(superAdmin.Permissions, policy.PermissionRBACAll) {
		t.Fatalf("system.super_admin permissions missing %q", policy.PermissionRBACAll)
	}
}

func seedHasPermission(perms []entities.Permission, required string) bool {
	for _, perm := range perms {
		if perm.Name == required {
			return true
		}
	}
	return false
}
