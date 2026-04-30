package middleware

import (
	"context"
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type realIPKey struct{}

var RealIPKey = realIPKey{}

// RealIP is a Gin middleware that extracts the real client IP from headers.
func RealIP() gin.HandlerFunc {
	return RealIPWithTrustedProxies(nil)
}

// RealIPWithTrustedProxies extracts proxy headers only when the direct peer is trusted.
func RealIPWithTrustedProxies(trustedProxyCIDRs []string) gin.HandlerFunc {
	trustedProxies := parseTrustedProxyCIDRs(trustedProxyCIDRs)
	return func(c *gin.Context) {
		rip := realIP(c.Request, trustedProxies)

		// Inject into context for raw Go handlers if needed.
		ctx := context.WithValue(c.Request.Context(), RealIPKey, rip)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// realIP extracts the real client IP from common proxy headers.
func realIP(req *http.Request, trustedProxies []*net.IPNet) string {
	peerIP := remoteIP(req.RemoteAddr)
	if isTrustedProxy(peerIP, trustedProxies) {
		if ip := firstForwardedIP(req.Header.Get("X-Forwarded-For")); ip != "" {
			return ip
		}
		if ip := validIP(req.Header.Get("X-Real-IP")); ip != "" {
			return ip
		}
	}

	if peerIP != nil {
		return peerIP.String()
	}
	return ""
}

func parseTrustedProxyCIDRs(cidrs []string) []*net.IPNet {
	trusted := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}
		if ip := net.ParseIP(cidr); ip != nil {
			if ip.To4() != nil {
				cidr = ip.String() + "/32"
			} else {
				cidr = ip.String() + "/128"
			}
		}
		_, ipNet, err := net.ParseCIDR(cidr)
		if err == nil {
			trusted = append(trusted, ipNet)
		}
	}
	return trusted
}

func isTrustedProxy(ip net.IP, trustedProxies []*net.IPNet) bool {
	if ip == nil || len(trustedProxies) == 0 {
		return false
	}
	for _, trusted := range trustedProxies {
		if trusted.Contains(ip) {
			return true
		}
	}
	return false
}

func remoteIP(remoteAddr string) net.IP {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	return net.ParseIP(strings.TrimSpace(host))
}

func firstForwardedIP(header string) string {
	for _, candidate := range strings.Split(header, ",") {
		if ip := validIP(candidate); ip != "" {
			return ip
		}
	}
	return ""
}

func validIP(candidate string) string {
	ip := net.ParseIP(strings.TrimSpace(candidate))
	if ip == nil {
		return ""
	}
	return ip.String()
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
