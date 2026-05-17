package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"erg.ninja/internal/modules/access_control/domain/policy"
	"erg.ninja/pkg/auth"
	"github.com/gin-gonic/gin"
)

func TestProtectedRouteRejectsAnonymous(t *testing.T) {
	router := enterpriseTestRouter(nil)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/lms/grades", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestProtectedRouteRejectsWrongPortal(t *testing.T) {
	router := enterpriseTestRouter(&auth.JWTClaims{
		UserID:      "user-1",
		Portal:      string(policy.PortalHocLieu),
		Permissions: []string{policy.PermissionLMSGradeRead},
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/lms/grades", nil))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestProtectedRouteRejectsMissingPermission(t *testing.T) {
	router := enterpriseTestRouter(&auth.JWTClaims{
		UserID:      "user-1",
		Portal:      string(policy.PortalLMS),
		Permissions: []string{policy.PermissionLMSCourseRead},
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/lms/grades", nil))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestProtectedRouteAllowsPortalAndPermission(t *testing.T) {
	router := enterpriseTestRouter(&auth.JWTClaims{
		UserID:      "teacher-1",
		Roles:       []string{policy.RoleLMSTeacher},
		Permissions: []string{policy.PermissionLMSGradeRead},
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/lms/grades", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestProtectedRouteDenyOverrideWins(t *testing.T) {
	router := enterpriseTestRouter(&auth.JWTClaims{
		UserID:            "admin-1",
		Portal:            string(policy.PortalLMS),
		Permissions:       []string{policy.PermissionLMSAll},
		DeniedPermissions: []string{policy.PermissionLMSGradeRead},
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/lms/grades", nil))

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func enterpriseTestRouter(claims *auth.JWTClaims) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	if claims != nil {
		router.Use(func(c *gin.Context) {
			c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ClaimsKey, claims))
			c.Next()
		})
	}
	router.GET(
		"/lms/grades",
		RequirePortal(string(policy.PortalLMS)),
		RequireAccessPermission(policy.PermissionLMSGradeRead),
		func(c *gin.Context) { c.Status(http.StatusOK) },
	)
	return router
}
