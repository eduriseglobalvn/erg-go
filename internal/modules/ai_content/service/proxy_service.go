// Package ai_content provides a transparent HTTP reverse proxy to the erg-backend
// NestJS AI Content module. All /api/ai-content/* requests are forwarded to
// http://localhost:3003/api/ai-content/* preserving headers, method, and body.
package ai_content

import (
	"bytes"
	"io"
	"net/http"

	"github.com/rs/zerolog"
)

// ProxyService performs transparent HTTP reverse proxying.
type ProxyService struct {
	target string
	log    zerolog.Logger
}

// NewProxyService creates a proxy service that forwards requests to the target base URL.
func NewProxyService(log zerolog.Logger, target string) *ProxyService {
	return &ProxyService{target: target, log: log}
}

// Proxy forwards the incoming HTTP request to the target server and returns the response.
// It preserves the request method, headers (except Host), and body.
func (s *ProxyService) Proxy(r *http.Request) (*http.Response, error) {
	// Build the target URL: target base + incoming URL path + query string
	targetURL := s.target + r.URL.Path
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}

	targetReq, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, r.Body)
	if err != nil {
		return nil, err
	}

	// Copy headers (Host is excluded — the transport sets it to target host).
	targetReq.Header = make(http.Header)
	for k, v := range r.Header {
		if k == "Host" {
			continue
		}
		targetReq.Header[k] = v
	}
	targetReq.Header.Set("X-Forwarded-Host", r.Host)
	targetReq.Header.Set("X-Forwarded-For", r.RemoteAddr)
	targetReq.Header.Set("X-Forwarded-Proto", "http")
	if rid := r.Header.Get("X-Request-ID"); rid != "" {
		targetReq.Header.Set("X-Request-ID", rid)
	}

	client := &http.Client{}
	return client.Do(targetReq)
}

// ─── Convenience helpers ───────────────────────────────────────────────────────

// CopyResponse copies the proxied response back to the original caller.
func CopyResponse(w http.ResponseWriter, r *http.Request, resp *http.Response) error {
	for k, v := range resp.Header {
		for _, vv := range v {
			w.Header().Add(k, vv)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, err := io.Copy(w, resp.Body)
	return err
}

// ReadBody reads the entire request body and returns a new readable copy.
func ReadBody(r *http.Request) ([]byte, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	r.Body = io.NopCloser(bytes.NewBuffer(body))
	return body, nil
}
