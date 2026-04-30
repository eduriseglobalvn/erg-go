package documents

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"erg.ninja/pkg/auth"
	"github.com/gin-gonic/gin"
)

func TestDocumentRoutesRejectAnonymous(t *testing.T) {
	gin.SetMode(gin.TestMode)
	validator, err := auth.NewHS256Validator("test-secret")
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	NewController(nil, nil, validator).RegisterRoutes(router)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/documents/", nil)

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestDocumentRoutesRejectInvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	validator, err := auth.NewHS256Validator("test-secret")
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	NewController(nil, nil, validator).RegisterRoutes(router)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/documents/", nil)
	req.Header.Set("Authorization", "Bearer not-a-valid-token")

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}
