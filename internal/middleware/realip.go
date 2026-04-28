package middleware

import (
	"context"
	"net"
	"strings"

	"github.com/gin-gonic/gin"
)

type realIPKey struct{}

var RealIPKey = realIPKey{}

// RealIP is a Gin middleware that extracts the real client IP from headers.
func RealIP() gin.HandlerFunc {
	return func(c *gin.Context) {
		rip := realIP(c)

		// Inject into context for raw Go handlers if needed.
		ctx := context.WithValue(c.Request.Context(), RealIPKey, rip)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// realIP extracts the real client IP from common proxy headers.
func realIP(c *gin.Context) string {
	fwd := c.GetHeader("X-Forwarded-For")
	if fwd != "" {
		ips := strings.Split(fwd, ",")
		if len(ips) > 0 {
			ip := strings.TrimSpace(ips[0])
			if parsed := net.ParseIP(ip); parsed != nil {
				return ip
			}
		}
	}

	xrip := c.GetHeader("X-Real-IP")
	if xrip != "" {
		if parsed := net.ParseIP(xrip); parsed != nil {
			return xrip
		}
	}

	// Fallback to Gin's ClientIP() which has its own trusted proxy logic.
	return c.ClientIP()
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
