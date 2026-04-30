// Package ai_content provides a transparent HTTP reverse proxy to the erg-backend
// NestJS AI Content module. All /api/ai-content/* requests are forwarded to
// http://localhost:3003/api/ai-content/* preserving headers, method, and body.
package ai_content

import (
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// ProxyService performs transparent HTTP reverse proxying.
type ProxyService struct {
	target *url.URL
	client *http.Client
	log    zerolog.Logger
}

// NewProxyService creates a proxy service that forwards requests to the target base URL.
func NewProxyService(log zerolog.Logger, target string) *ProxyService {
	parsed, err := parseProxyTarget(target)
	if err != nil {
		log.Error().Err(err).Str("target", target).Msg("ai_content proxy target disabled")
	}

	svc := &ProxyService{target: parsed, log: log}
	svc.client = &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			if svc.target == nil || !svc.sameTarget(req.URL) {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	return svc
}

// Proxy forwards the incoming HTTP request to the target server and returns the response.
// It preserves the request method, headers (except Host), and body.
func (s *ProxyService) Proxy(r *http.Request) (*http.Response, error) {
	targetURL, err := s.targetURLFor(r)
	if err != nil {
		return nil, err
	}

	targetReq, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL.String(), r.Body) // #nosec G107,G704 -- targetURL is rebuilt from a configured base URL and validated by targetURLFor.
	if err != nil {
		return nil, err
	}

	targetReq.Header = make(http.Header)
	for k, v := range r.Header {
		if isProxyHopHeader(k) {
			continue
		}
		targetReq.Header[k] = v
	}
	targetReq.Header.Set("X-Forwarded-Host", r.Host)
	targetReq.Header.Set("X-Forwarded-For", clientIP(r.RemoteAddr))
	targetReq.Header.Set("X-Forwarded-Proto", forwardedProto(r))
	if rid := r.Header.Get("X-Request-ID"); rid != "" {
		targetReq.Header.Set("X-Request-ID", rid)
	}

	return s.client.Do(targetReq) // #nosec G704 -- requests are constrained to the configured proxy target host.
}

func parseProxyTarget(raw string) (*url.URL, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("ai_content proxy target is empty")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, errors.New("ai_content proxy target must use http or https")
	}
	if parsed.Hostname() == "" {
		return nil, errors.New("ai_content proxy target host is required")
	}
	parsed.Fragment = ""
	return parsed, nil
}

func (s *ProxyService) targetURLFor(r *http.Request) (*url.URL, error) {
	if s == nil || s.target == nil {
		return nil, errors.New("ai_content proxy target is not configured")
	}
	if r == nil || r.URL == nil {
		return nil, errors.New("request URL is required")
	}
	if r.URL.IsAbs() || r.URL.Host != "" || r.URL.Scheme != "" {
		return nil, errors.New("absolute upstream URLs are not allowed")
	}

	target := *s.target
	requestPath := path.Clean("/" + strings.TrimPrefix(r.URL.Path, "/"))
	if requestPath == "/" {
		requestPath = ""
	}
	target.Path = path.Join(s.target.Path, requestPath)
	if strings.HasSuffix(r.URL.Path, "/") && !strings.HasSuffix(target.Path, "/") {
		target.Path += "/"
	}
	target.RawQuery = r.URL.RawQuery
	target.Fragment = ""
	if !s.sameTarget(&target) {
		return nil, errors.New("proxy target host changed during URL build")
	}
	return &target, nil
}

func (s *ProxyService) sameTarget(candidate *url.URL) bool {
	if s == nil || s.target == nil || candidate == nil {
		return false
	}
	return strings.EqualFold(candidate.Scheme, s.target.Scheme) &&
		strings.EqualFold(candidate.Hostname(), s.target.Hostname()) &&
		normalizedPort(candidate) == normalizedPort(s.target)
}

func normalizedPort(u *url.URL) string {
	if port := u.Port(); port != "" {
		return port
	}
	switch strings.ToLower(u.Scheme) {
	case "http":
		return "80"
	case "https":
		return "443"
	default:
		return ""
	}
}

func isProxyHopHeader(key string) bool {
	switch http.CanonicalHeaderKey(key) {
	case "Host",
		"Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Te",
		"Trailer",
		"Transfer-Encoding",
		"Upgrade",
		"X-Forwarded-For",
		"X-Forwarded-Host",
		"X-Forwarded-Proto":
		return true
	default:
		return false
	}
}

func clientIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

func forwardedProto(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

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
