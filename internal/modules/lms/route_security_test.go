package lms

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"erg.ninja/internal/middleware"
	"erg.ninja/pkg/auth"
	"github.com/gin-gonic/gin"
)

func TestLMSRoutesRejectAnonymous(t *testing.T) {
	gin.SetMode(gin.TestMode)
	validator, err := auth.NewHS256Validator("test-secret")
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	group := router.Group("/api/lms")
	group.Use(middleware.JWTMiddleware(validator))
	NewController(nil).RegisterRoutes(group)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/lms/quizzes", nil)

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}
