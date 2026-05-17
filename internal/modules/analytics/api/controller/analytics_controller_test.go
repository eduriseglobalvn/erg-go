package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"erg.ninja/pkg/auth"
)

func TestProtectedAnalyticsRoutesRejectInvalidBearer(t *testing.T) {
	gin.SetMode(gin.TestMode)

	validator, err := auth.NewHS256Validator("test-secret")
	if err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	NewController(nil, nil, nil, validator).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/api/insight/overview", nil)
	req.Header.Set("Authorization", "Bearer definitely-invalid")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestProtectedAnalyticsRoutesRequireAnalyticsReadPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)

	validator, err := auth.NewHS256Validator("test-secret")
	if err != nil {
		t.Fatal(err)
	}
	token, err := validator.GenerateHS256(&auth.JWTClaims{
		UserID:      "user-1",
		Roles:       []string{"viewer"},
		Permissions: []string{"posts.read"},
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "user-1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	NewController(nil, nil, nil, validator).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/api/insight/overview", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}
