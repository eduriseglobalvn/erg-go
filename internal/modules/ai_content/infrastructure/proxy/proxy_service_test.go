package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
)

func TestProxyTargetURLForKeepsConfiguredHost(t *testing.T) {
	svc := NewProxyService(zerolog.Nop(), "https://ai.internal:8443/base")
	req := httptest.NewRequest(http.MethodGet, "/api/ai-content/generate?topic=math", nil)

	target, err := svc.targetURLFor(req)
	if err != nil {
		t.Fatalf("targetURLFor returned error: %v", err)
	}
	if target.Scheme != "https" || target.Host != "ai.internal:8443" {
		t.Fatalf("target host changed: %s", target.String())
	}
	if target.Path != "/base/api/ai-content/generate" {
		t.Fatalf("unexpected target path: %s", target.Path)
	}
	if target.RawQuery != "topic=math" {
		t.Fatalf("unexpected query: %s", target.RawQuery)
	}
}

func TestProxyTargetURLForRejectsAbsoluteURL(t *testing.T) {
	svc := NewProxyService(zerolog.Nop(), "https://ai.internal")
	req := httptest.NewRequest(http.MethodGet, "https://evil.example/api/ai-content", nil)

	if _, err := svc.targetURLFor(req); err == nil {
		t.Fatal("expected absolute URL to be rejected")
	}
}

func TestProxySanitizesForwardedHeaders(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Forwarded-For"); got != "203.0.113.8" {
			t.Fatalf("unexpected X-Forwarded-For: %q", got)
		}
		if got := r.Header.Get("X-Forwarded-Host"); got != "app.example" {
			t.Fatalf("unexpected X-Forwarded-Host: %q", got)
		}
		if got := r.Header.Get("X-Forwarded-Proto"); got != "http" {
			t.Fatalf("unexpected X-Forwarded-Proto: %q", got)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer upstream.Close()

	svc := NewProxyService(zerolog.Nop(), upstream.URL)
	req := httptest.NewRequest(http.MethodPost, "/api/ai-content/generate", nil)
	req.RemoteAddr = "203.0.113.8:4567"
	req.Host = "app.example"
	req.Header.Set("X-Forwarded-For", "10.0.0.1")
	req.Header.Set("Connection", "upgrade")

	resp, err := svc.Proxy(req)
	if err != nil {
		t.Fatalf("Proxy returned error: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
}
