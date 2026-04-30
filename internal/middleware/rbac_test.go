package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"erg.ninja/pkg/auth"
	"github.com/gin-gonic/gin"
)

func TestRequireRolesRejectsMissingRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		claims := &auth.JWTClaims{UserID: "user-1", Roles: []string{"user"}}
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ClaimsKey, claims))
		c.Next()
	})
	router.GET("/admin", RequireRoles("admin"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin", nil))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestRequireRolesAllowsAdminBypass(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		claims := &auth.JWTClaims{UserID: "admin-1", Roles: []string{"admin"}}
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ClaimsKey, claims))
		c.Next()
	})
	router.GET("/teacher", RequireRoles("teacher"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/teacher", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}
