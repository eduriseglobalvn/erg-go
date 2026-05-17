package service

import (
	"context"
	"errors"
	"testing"
	"time"

	entities "erg.ninja/internal/modules/auth/domain/entity"
	"erg.ninja/pkg/auth"
	"erg.ninja/pkg/config"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestEnsureAdminRolesAddsRequiredRoles(t *testing.T) {
	roles := ensureAdminRoles([]string{"user", "admin", "user"})

	want := map[string]bool{
		"user":               true,
		"admin":              true,
		"SUPER_ADMIN":        true,
		"system.super_admin": true,
		"erg_super_admin":    true,
	}
	if len(roles) != len(want) {
		t.Fatalf("roles length = %d, want %d: %v", len(roles), len(want), roles)
	}
	for _, role := range roles {
		if !want[role] {
			t.Fatalf("unexpected role %q in %v", role, roles)
		}
	}
}

func TestSameStringSetIgnoresOrderAndDuplicates(t *testing.T) {
	if !sameStringSet([]string{"admin", "user", "admin"}, []string{"user", "admin"}) {
		t.Fatal("expected sets with duplicate values to match")
	}
	if sameStringSet([]string{"admin"}, []string{"admin", "SUPER_ADMIN"}) {
		t.Fatal("expected different sets not to match")
	}
}

func TestConfiguredAdminEmailFallsBackToDefaultRoot(t *testing.T) {
	svc := &AuthService{}
	if !svc.isConfiguredAdminEmail("admin@erg.edu.vn") {
		t.Fatal("default root admin email should be treated as configured")
	}
	if !svc.isConfiguredAdminEmail(" ADMIN@ERG.EDU.VN ") {
		t.Fatal("default root admin email check should be case-insensitive")
	}
}

func TestConfiguredAdminEmailAllowsConfiguredAndDefaultRoot(t *testing.T) {
	svc := &AuthService{adminEmail: "owner@example.com"}
	if !svc.isConfiguredAdminEmail("owner@example.com") {
		t.Fatal("configured admin email should be treated as admin")
	}
	if !svc.isConfiguredAdminEmail("admin@erg.edu.vn") {
		t.Fatal("default root admin email should remain protected")
	}
}

func TestMissingLoginAttemptTableIsIgnoredOnlyOutsideProduction(t *testing.T) {
	err := errors.New(`auth.repository.latestSuccessfulLoginAt: ERROR: relation "auth_login_attempts" does not exist (SQLSTATE 42P01)`)

	devSvc := &AuthService{cfg: &config.Config{App: config.AppConfig{Env: "development"}}}
	if !devSvc.shouldIgnoreLoginAttemptStoreError(err) {
		t.Fatal("development should degrade when login-attempt audit table is missing")
	}

	prodSvc := &AuthService{cfg: &config.Config{App: config.AppConfig{Env: "production"}}}
	if prodSvc.shouldIgnoreLoginAttemptStoreError(err) {
		t.Fatal("production should not ignore missing login-attempt audit table")
	}
}

func TestLoginSessionPersistenceContextSurvivesExpiredLoginContext(t *testing.T) {
	parent, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	time.Sleep(time.Millisecond)

	ctx, ctxCancel := loginSessionPersistenceContext(parent)
	defer ctxCancel()

	if err := ctx.Err(); err != nil {
		t.Fatalf("session persistence context should not inherit expired login deadline, got %v", err)
	}
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) <= 0 {
		t.Fatalf("session persistence context should have an active bounded deadline, deadline=%v ok=%v", deadline, ok)
	}
}

func TestPermissionsForTokenGrantsWildcardToAdmin(t *testing.T) {
	svc := &AuthService{}
	user := &entities.User{
		ID:    bson.NewObjectID(),
		Roles: []string{"user", "SUPER_ADMIN"},
	}

	perms := svc.permissionsForToken(context.Background(), user)
	if len(perms) != 1 || perms[0] != "*" {
		t.Fatalf("admin permissions = %v, want wildcard", perms)
	}
}

func TestClassifyAccountAccessCommunityOnly(t *testing.T) {
	user := &entities.User{ID: bson.NewObjectID(), Roles: []string{"user"}, Provider: "google"}
	accountType, accessLevel := classifyAccountAccess(user, nil, nil)
	if accountType != "community" || accessLevel != "community_only" {
		t.Fatalf("classification = %s/%s, want community/community_only", accountType, accessLevel)
	}
}

func TestClassifyAccountAccessCMS(t *testing.T) {
	user := &entities.User{ID: bson.NewObjectID(), Roles: []string{"content_manager"}}
	accountType, accessLevel := classifyAccountAccess(user, []string{"cms"}, []string{"posts.read"})
	if accountType != "staff" || accessLevel != "cms" {
		t.Fatalf("classification = %s/%s, want staff/cms", accountType, accessLevel)
	}
}

func TestClassifyAccountAccessElearningStudent(t *testing.T) {
	user := &entities.User{ID: bson.NewObjectID(), Roles: []string{"student"}}
	accountType, accessLevel := classifyAccountAccess(user, portalsForToken(user.Roles), nil)
	if accountType != "student" || accessLevel != "elearning" {
		t.Fatalf("classification = %s/%s, want student/elearning", accountType, accessLevel)
	}
	if portals := portalsForToken(user.Roles); len(portals) != 1 || portals[0] != "elearning" {
		t.Fatalf("student portals = %v, want [elearning]", portals)
	}
}

func TestAuthProviderIssuePairReturnsBearerTokens(t *testing.T) {
	provider := auth.NewAuthServiceProvider("access-secret-with-enough-entropy", "refresh-secret-with-enough-entropy")

	tokens, err := provider.IssuePair("session-1", bson.NewObjectID().Hex(), "admin@erg.edu.vn", []string{"admin"}, []string{"*"})
	if err != nil {
		t.Fatalf("IssuePair() error = %v", err)
	}
	if tokens.AccessToken == "" {
		t.Fatal("access token is empty")
	}
	if tokens.RefreshToken == "" {
		t.Fatal("refresh token is empty")
	}
	if tokens.TokenType != "Bearer" {
		t.Fatalf("token type = %q, want Bearer", tokens.TokenType)
	}
}
