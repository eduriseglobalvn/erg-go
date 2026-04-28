// Package middleware provides Gin-compatible HTTP middleware for erg-go.
package middleware

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestID generates and attaches a unique request ID to each request.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

// GetRequestID extracts the request ID from the Gin context.
func GetRequestID(c *gin.Context) string {
	if v, exists := c.Get("request_id"); exists {
		return v.(string)
	}
	return GetRequestIDFromContext(c.Request.Context())
}

// GetRequestIDFromContext extracts the request ID from a standard context.
func GetRequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value("request_id").(string); ok {
		return v
	}
	return ""
}
