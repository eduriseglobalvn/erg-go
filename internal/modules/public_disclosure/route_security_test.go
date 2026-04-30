package public_disclosure

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"erg.ninja/pkg/auth"
	"github.com/gin-gonic/gin"
)

func TestPublicDisclosureWriteRoutesRequireAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	validator, err := auth.NewHS256Validator("test-secret")
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	NewController(nil, nil, validator).RegisterRoutes(router)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/public-disclosure/", nil)

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}
