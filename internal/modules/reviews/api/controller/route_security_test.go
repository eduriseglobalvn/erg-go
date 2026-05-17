package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"erg.ninja/pkg/auth"
	"github.com/gin-gonic/gin"
)

func TestReviewAdminRoutesRequireAdminRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	validator, err := auth.NewHS256Validator("test-secret")
	if err != nil {
		t.Fatal(err)
	}
	token, err := validator.GenerateHS256(&auth.JWTClaims{
		UserID: "user-1",
		Roles:  []string{"user"},
	}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	NewController(nil, nil, validator).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPatch, "/api/reviews/review-1/status", strings.NewReader(`{"status":"approved"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}
