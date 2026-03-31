package middleware

import (
	"context"
	"net"
	"net/http"
	"strings"
)

// RealIPMiddleware extracts the real client IP address from X-Forwarded-For,
// X-Real-IP, or the remote address when behind a proxy or load balancer.
func RealIPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rip := realIP(r)
		ctx := context.WithValue(r.Context(), RealIPKey, rip)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RealIPKey is the context key for the real IP address.
type RealIPContextKey struct{}

var RealIPKey = RealIPContextKey{}

// realIP extracts the real client IP from common proxy headers.
func realIP(r *http.Request) string {
	// Check X-Forwarded-For header (may contain multiple IPs: client, proxy1, proxy2).
	// The first IP is typically the original client.
	fwd := r.Header.Get("X-Forwarded-For")
	if fwd != "" {
		ips := strings.Split(fwd, ",")
		if len(ips) > 0 {
			ip := strings.TrimSpace(ips[0])
			if parsed := net.ParseIP(ip); parsed != nil {
				return ip
			}
		}
	}

	// Check X-Real-IP header (set by nginx).
	xrip := r.Header.Get("X-Real-IP")
	if xrip != "" {
		if parsed := net.ParseIP(xrip); parsed != nil {
			return xrip
		}
	}

	// Fall back to the remote address from the connection.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	if parsed := net.ParseIP(host); parsed != nil {
		return host
	}

	return r.RemoteAddr
}

// GetRealIP extracts the real IP from the context.
func GetRealIP(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if ip, ok := ctx.Value(RealIPKey).(string); ok {
		return ip
	}
	return ""
}
