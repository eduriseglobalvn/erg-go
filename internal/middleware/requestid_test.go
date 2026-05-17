package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	platformctx "erg.ninja/internal/platform/context"
)

func TestRequestIDStoresValueInGinAndStdContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID())
	router.GET("/ping", func(c *gin.Context) {
		if got := GetRequestID(c); got != "req-1" {
			t.Fatalf("GetRequestID() = %q", got)
		}
		if got := platformctx.RequestID(c.Request.Context()); got != "req-1" {
			t.Fatalf("platform request id = %q", got)
		}
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("X-Request-ID", "req-1")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}
}
