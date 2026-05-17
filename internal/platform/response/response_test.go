package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	platformctx "erg.ninja/internal/platform/context"
	"erg.ninja/internal/platform/exception"
)

func TestOKWritesCanonicalEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/ping", nil)
	ctx.Request = ctx.Request.WithContext(platformctx.WithRequestID(ctx.Request.Context(), "req-1"))

	OK(ctx, gin.H{"pong": true})

	var got Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !got.Success || got.Code != "OK" || got.Message != "Success" {
		t.Fatalf("unexpected envelope: %+v", got)
	}
	if got.Meta.RequestID != "req-1" {
		t.Fatalf("request id = %q", got.Meta.RequestID)
	}
}

func TestErrorWritesCanonicalEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/login", nil)

	WriteError(ctx, exception.New("AUTH_INVALID_CREDENTIALS", "invalid credentials", http.StatusUnauthorized))

	var got Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.Success {
		t.Fatal("expected error envelope")
	}
	if got.Code != "AUTH_INVALID_CREDENTIALS" || got.Message != "invalid credentials" {
		t.Fatalf("unexpected envelope: %+v", got)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
}
