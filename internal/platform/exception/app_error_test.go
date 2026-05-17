package exception

import (
	"errors"
	"net/http"
	"testing"
)

func TestNewBuildsAppError(t *testing.T) {
	err := New("AUTH_INVALID_CREDENTIALS", "invalid credentials", http.StatusUnauthorized)

	if err.Code != "AUTH_INVALID_CREDENTIALS" {
		t.Fatalf("Code = %q", err.Code)
	}
	if err.Message != "invalid credentials" {
		t.Fatalf("Message = %q", err.Message)
	}
	if err.HTTPStatus != http.StatusUnauthorized {
		t.Fatalf("HTTPStatus = %d", err.HTTPStatus)
	}
}

func TestWrapPreservesCause(t *testing.T) {
	root := errors.New("boom")
	err := Wrap("DATABASE_ERROR", "query failed", http.StatusInternalServerError, root)

	if !errors.Is(err, root) {
		t.Fatal("wrapped error should preserve cause")
	}
}

func TestStatusFallsBackToInternalServerError(t *testing.T) {
	err := New("UNKNOWN", "unknown", 0)

	if got := err.Status(); got != http.StatusInternalServerError {
		t.Fatalf("Status() = %d, want %d", got, http.StatusInternalServerError)
	}
}
