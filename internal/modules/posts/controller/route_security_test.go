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

func TestPostWriteRoutesUseSharedRBAC(t *testing.T) {
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
	NewController(nil, validator, nil).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/api/posts/", strings.NewReader(`{"title":"T","category_id":"c"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestPostWriteRoutesRejectAnonymous(t *testing.T) {
	gin.SetMode(gin.TestMode)
	validator, err := auth.NewHS256Validator("test-secret")
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	NewController(nil, validator, nil).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/api/posts/", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}
