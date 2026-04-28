package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestNewHS256Validator(t *testing.T) {
	v, err := NewHS256Validator("super-secret-key-32-chars-long!")
	if err != nil {
		t.Fatalf("NewHS256Validator: %v", err)
	}
	if v == nil {
		t.Fatal("NewHS256Validator returned nil")
	}
	if len(v.secretKey) == 0 {
		t.Error("secret key should not be empty")
	}
}

func TestNewHS256ValidatorEmptySecret(t *testing.T) {
	_, err := NewHS256Validator("")
	if err == nil {
		t.Error("expected error for empty secret")
	}
}

func TestHS256Validate(t *testing.T) {
	v, err := NewHS256Validator("test-secret-key-for-testing-only!")
	if err != nil {
		t.Fatalf("NewHS256Validator: %v", err)
	}

	// Generate a valid token.
	claims := &JWTClaims{
		UserID:      "user-123",
		Email:       "alice@example.com",
		Permissions: []string{"read", "write"},
		Roles:       []string{"admin"},
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-123",
			Issuer:    "test-issuer",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token, err := v.GenerateHS256(claims, 1*time.Hour)
	if err != nil {
		t.Fatalf("GenerateHS256: %v", err)
	}

	// Validate it.
	validated, err := v.Validate(token)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if validated.UserID != "user-123" {
		t.Errorf("UserID = %q, want 'user-123'", validated.UserID)
	}
	if validated.Email != "alice@example.com" {
		t.Errorf("Email = %q, want 'alice@example.com'", validated.Email)
	}
}

func TestHS256ValidateExpiredToken(t *testing.T) {
	v, err := NewHS256Validator("test-secret-key-for-testing-only!")
	if err != nil {
		t.Fatalf("NewHS256Validator: %v", err)
	}

	// Create an expired token manually.
	claims := &JWTClaims{
		UserID: "user-123",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-123",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString(v.secretKey)

	_, err = v.Validate(signed)
	if err == nil {
		t.Error("expected error for expired token")
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Errorf("error should mention 'expired': %v", err)
	}
}

func TestHS256ValidateInvalidToken(t *testing.T) {
	v, err := NewHS256Validator("test-secret-key-for-testing-only!")
	if err != nil {
		t.Fatalf("NewHS256Validator: %v", err)
	}

	_, err = v.Validate("not.a.valid.jwt.token")
	if err == nil {
		t.Error("expected error for invalid token")
	}
}

func TestHS256ValidateWrongSecret(t *testing.T) {
	v1, _ := NewHS256Validator("secret-one")
	v2, _ := NewHS256Validator("secret-two")

	token, _ := v1.GenerateHS256(&JWTClaims{UserID: "user"}, 1*time.Hour)

	_, err := v2.Validate(token)
	if err == nil {
		t.Error("expected error when validating with wrong secret")
	}
}

func TestValidateRequest(t *testing.T) {
	v, _ := NewHS256Validator("test-secret-key-for-testing-only!")
	claims := &JWTClaims{UserID: "user-abc", RegisteredClaims: jwt.RegisteredClaims{
		Subject:   "user-abc",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
	}}
	token, _ := v.GenerateHS256(claims, 1*time.Hour)

	validated, err := v.ValidateRequest("Bearer " + token)
	if err != nil {
		t.Fatalf("ValidateRequest: %v", err)
	}
	if validated.UserID != "user-abc" {
		t.Errorf("UserID = %q, want 'user-abc'", validated.UserID)
	}
}

func TestValidateRequestInvalidFormat(t *testing.T) {
	v, _ := NewHS256Validator("test-secret")
	_, err := v.ValidateRequest("InvalidFormat")
	if err == nil {
		t.Error("expected error for invalid header format")
	}

	_, err = v.ValidateRequest("Basic abc123")
	if err == nil {
		t.Error("expected error for non-Bearer prefix")
	}
}

func TestJWTValidatorWithIssuer(t *testing.T) {
	v, _ := NewHS256Validator("test-secret", WithJWTIssuer("my-app"))

	claims := &JWTClaims{
		UserID: "user-1",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			Issuer:    "my-app",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
	}
	token, _ := v.GenerateHS256(claims, 1*time.Hour)

	validated, err := v.Validate(token)
	if err != nil {
		t.Fatalf("Validate with issuer: %v", err)
	}
	if validated.UserID != "user-1" {
		t.Errorf("UserID = %q, want 'user-1'", validated.UserID)
	}
}

func TestJWTValidatorWrongIssuer(t *testing.T) {
	v, _ := NewHS256Validator("test-secret", WithJWTIssuer("my-app"))

	claims := &JWTClaims{
		UserID: "user-1",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			Issuer:    "wrong-issuer",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString([]byte("test-secret"))

	_, err := v.Validate(signed)
	if err == nil {
		t.Error("expected error for wrong issuer")
	}
}

func TestContains(t *testing.T) {
	cases := []struct {
		slice []string
		item  string
		want  bool
	}{
		{[]string{"a", "b", "c"}, "b", true},
		{[]string{"a", "b", "c"}, "d", false},
		{[]string{}, "a", false},
		{[]string{"HS256", "RS256"}, "HS256", true},
	}
	for _, c := range cases {
		got := contains(c.slice, c.item)
		if got != c.want {
			t.Errorf("contains(%v, %q) = %v, want %v", c.slice, c.item, got, c.want)
		}
	}
}

func TestSessionIDFromClaimsPrefersDedicatedClaim(t *testing.T) {
	claims := &JWTClaims{
		SessionID:   "session-123",
		Permissions: []string{"legacy-session", "roles.read"},
	}

	if got := SessionIDFromClaims(claims); got != "session-123" {
		t.Fatalf("SessionIDFromClaims() = %q, want %q", got, "session-123")
	}
}

func TestSessionIDFromClaimsFallsBackToLegacyPermissions(t *testing.T) {
	claims := &JWTClaims{
		Permissions: []string{"legacy-session", "roles.read"},
	}

	if got := SessionIDFromClaims(claims); got != "legacy-session" {
		t.Fatalf("SessionIDFromClaims() = %q, want %q", got, "legacy-session")
	}
}

func TestIssuePairUsesConfiguredIssuerAndDedicatedSessionClaim(t *testing.T) {
	provider := NewAuthServiceProvider(
		"access-secret",
		"refresh-secret",
		WithIssuer("erg-backend"),
	)

	pair, err := provider.IssuePair(
		"session-xyz",
		"user-1",
		"admin@erg.edu.vn",
		[]string{"admin"},
		[]string{"roles.read", "system.settings"},
	)
	if err != nil {
		t.Fatalf("IssuePair: %v", err)
	}

	accessClaims, err := provider.ValidateAccessToken(pair.AccessToken)
	if err != nil {
		t.Fatalf("ValidateAccessToken: %v", err)
	}
	if accessClaims.SessionID != "session-xyz" {
		t.Fatalf("access session_id = %q, want %q", accessClaims.SessionID, "session-xyz")
	}
	if got := SessionIDFromClaims(accessClaims); got != "session-xyz" {
		t.Fatalf("SessionIDFromClaims(access) = %q, want %q", got, "session-xyz")
	}
	if len(accessClaims.Permissions) < 2 || accessClaims.Permissions[0] != "roles.read" {
		t.Fatalf("access permissions = %v, expected real permissions without legacy session prefix", accessClaims.Permissions)
	}

	refreshClaims, err := provider.ValidateRefreshToken(pair.RefreshToken)
	if err != nil {
		t.Fatalf("ValidateRefreshToken: %v", err)
	}
	if refreshClaims.SessionID != "session-xyz" {
		t.Fatalf("refresh session_id = %q, want %q", refreshClaims.SessionID, "session-xyz")
	}
}
