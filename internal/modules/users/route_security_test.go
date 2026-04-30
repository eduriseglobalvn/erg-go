package users

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"erg.ninja/pkg/auth"
	"github.com/gin-gonic/gin"
)

func TestUsersAdminRoutesRequireAdminRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	validator, err := auth.NewHS256Validator("test-secret")
	if err != nil {
		t.Fatal(err)
	}
	token, err := validator.GenerateHS256(&auth.JWTClaims{
		UserID:    "507f1f77bcf86cd799439011",
		SessionID: "session-1",
		Roles:     []string{"user"},
	}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	router := gin.New()
	newController(nil, validator, nil).registerRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/api/users/507f1f77bcf86cd799439012", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}
