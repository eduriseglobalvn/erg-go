package middleware

import "testing"

func TestPermissionServiceIsAdminUsesConfiguredAdminIDs(t *testing.T) {
	svc := NewPermissionService(nil, WithAdminIDs([]string{"admin-1", "", "admin-2"}))

	if !svc.isAdmin(nil, "admin-1") {
		t.Fatal("expected configured admin to bypass permission checks")
	}
	if svc.isAdmin(nil, "viewer-1") {
		t.Fatal("did not expect unknown user to be treated as admin")
	}
}
