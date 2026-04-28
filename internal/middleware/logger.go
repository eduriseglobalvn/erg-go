// Package middleware provides Gin-compatible HTTP middleware for erg-go.
package middleware

import (
	"time"

	"github.com/gin-gonic/gin"

	"erg.ninja/pkg/logger"
)

// Logger returns a Gin middleware that logs HTTP requests in JSON format.
func Logger(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method

		// Build log event.
		evt := log.Info()
		if status >= 500 {
			evt = log.Error()
		} else if status >= 400 {
			evt = log.Warn()
		}

		evt.
			Str("request_id", GetRequestID(c)).
			Str("method", method).
			Str("path", path).
			Str("query", query).
			Int("status", status).
			Dur("latency", latency).
			Str("ip", clientIP).
			Str("user_agent", c.Request.UserAgent()).
			Int("body_size", c.Writer.Size()).
			Msg("HTTP request")
	}
}
