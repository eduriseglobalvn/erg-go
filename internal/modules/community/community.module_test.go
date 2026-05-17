package community

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"erg.ninja/pkg/auth"
)

func TestCommunityWriteRequiresLogin(t *testing.T) {
	router, _ := testCommunityRouter(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/hoclieu/community/posts", strings.NewReader(`{"content":"hello"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401: %s", rec.Code, rec.Body.String())
	}
}

func TestCommunityPublicTopicsFallbackWhenStoreMissing(t *testing.T) {
	router, _ := testCommunityRouter(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/hoclieu/community/topics", nil)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 fallback topics: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Bảng tin giáo viên") {
		t.Fatalf("fallback topics should be usable Vietnamese forum topics: %s", rec.Body.String())
	}
}

func TestCommunityPublicFeedFallbackWhenStoreMissing(t *testing.T) {
	router, _ := testCommunityRouter(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/hoclieu/community/feed?limit=30", nil)
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 fallback feed: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"data":[]`) {
		t.Fatalf("fallback feed should be an empty page: %s", rec.Body.String())
	}
}

func TestCommunityWriteDoesNotRequireHocLieuPortal(t *testing.T) {
	router, validator := testCommunityRouter(t)
	token, err := validator.GenerateHS256(&auth.JWTClaims{
		UserID: "665000000000000000000001",
		Roles:  []string{"user"},
	}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/hoclieu/community/posts", strings.NewReader(`{"topicId":"665000000000000000000101","content":"hello"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)
	if rec.Code == http.StatusForbidden {
		t.Fatalf("community route must not require hoclieu portal: %s", rec.Body.String())
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 from missing test store after auth passes: %s", rec.Code, rec.Body.String())
	}
}

func testCommunityRouter(t *testing.T) (*gin.Engine, *auth.JWTValidator) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	validator, err := auth.NewHS256Validator("test-secret")
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	mod := NewModule(Deps{JWTValidator: validator})
	if err := mod.Setup(); err != nil {
		t.Fatal(err)
	}
	mod.RegisterRoutes(router)
	return router, validator
}
