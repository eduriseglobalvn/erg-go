package lms

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"erg.ninja/pkg/auth"
)

func TestLMSRoutesRejectAnonymous(t *testing.T) {
	router, _, _ := testLMSRouter(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/lms/classes", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestLMSRoutesFailClosedWithoutJWTValidator(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	mod := &Module{
		deps: Deps{},
		ctrl: NewController(NewService(newMemoryRepository(), nil)),
	}
	mod.RegisterRoutes(router)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/lms/classes", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestLMSRoutesRejectWrongPortal(t *testing.T) {
	router, validator, _ := testLMSRouter(t)
	token := lmsToken(t, validator, &auth.JWTClaims{
		UserID:      "teacher-1",
		Portal:      "elearning",
		Permissions: []string{"lms.class.read"},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/lms/classes", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestLMSRoutesRejectMissingPermission(t *testing.T) {
	router, validator, _ := testLMSRouter(t)
	token := lmsToken(t, validator, &auth.JWTClaims{
		UserID: "teacher-1",
		Portal: "lms",
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/lms/classes", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestLMSRoutesAllowPortalWithPermission(t *testing.T) {
	router, _, token := testLMSRouter(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/lms/classes", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

func TestLMSBulkStudentAccountsRequiresImportPermission(t *testing.T) {
	router, validator, _ := testLMSRouter(t)
	token := lmsToken(t, validator, &auth.JWTClaims{
		UserID:      "teacher-1",
		Portal:      "lms",
		Permissions: []string{"lms.class.update"},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/lms/students/bulk-accounts", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

func TestLMSBulkStudentAccountsAllowsImportPermission(t *testing.T) {
	router, validator, _ := testLMSRouter(t)
	token := lmsToken(t, validator, &auth.JWTClaims{
		UserID:      "teacher-1",
		Portal:      "lms",
		Permissions: []string{"lms.student.import"},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/lms/students/bulk-accounts", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(rec, req)

	if rec.Code == http.StatusForbidden {
		t.Fatalf("status = %d, route permission should allow lms.student.import", rec.Code)
	}
}

func testLMSRouter(t *testing.T) (*gin.Engine, *auth.JWTValidator, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	validator, err := auth.NewHS256Validator("test-secret")
	if err != nil {
		t.Fatal(err)
	}
	token := lmsToken(t, validator, &auth.JWTClaims{
		UserID:      "teacher-1",
		Portal:      "lms",
		Permissions: []string{"lms.class.read"},
	})
	router := gin.New()
	mod := &Module{
		deps: Deps{JWTValidator: validator},
		ctrl: NewController(NewService(newMemoryRepository(), nil)),
	}
	mod.RegisterRoutes(router)
	return router, validator, token
}

func lmsToken(t *testing.T, validator *auth.JWTValidator, claims *auth.JWTClaims) string {
	t.Helper()
	token, err := validator.GenerateHS256(claims, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return token
}
