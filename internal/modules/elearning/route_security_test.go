package elearning

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"erg.ninja/pkg/auth"
)

func TestElearningStudentRoutesRejectAnonymous(t *testing.T) {
	router, _, _ := testElearningRouter(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/elearning/dashboard", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestElearningRoutesFailClosedWithoutJWTValidator(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	mod := &Module{
		deps: Deps{},
		ctrl: NewController(&Service{}, nil),
	}
	mod.RegisterRoutes(router)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/elearning/dashboard", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestElearningStudentRoutesRejectWrongPortal(t *testing.T) {
	router, validator, _ := testElearningRouter(t)
	token := elearningToken(t, validator, &auth.JWTClaims{
		UserID: "student-1",
		Portal: "lms",
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/elearning/dashboard", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestElearningAdminRoutesRejectMissingPermission(t *testing.T) {
	router, validator, _ := testElearningRouter(t)
	token := elearningToken(t, validator, &auth.JWTClaims{
		UserID: "staff-1",
		Portal: "elearning",
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/elearning/categories", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestElearningStudentRoutesAllowElearningPortal(t *testing.T) {
	router, _, token := testElearningRouter(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/elearning/dashboard", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

func testElearningRouter(t *testing.T) (*gin.Engine, *auth.JWTValidator, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	validator, err := auth.NewHS256Validator("test-secret")
	if err != nil {
		t.Fatal(err)
	}
	token := elearningToken(t, validator, &auth.JWTClaims{
		UserID: "student-1",
		Portal: "elearning",
	})
	router := gin.New()
	mod := &Module{
		deps: Deps{JWTValidator: validator},
		ctrl: NewController(&Service{}, nil),
	}
	mod.RegisterRoutes(router)
	return router, validator, token
}

func elearningToken(t *testing.T, validator *auth.JWTValidator, claims *auth.JWTClaims) string {
	t.Helper()
	token, err := validator.GenerateHS256(claims, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return token
}
