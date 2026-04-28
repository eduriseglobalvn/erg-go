// Package middleware provides Gin-compatible HTTP middleware for erg-go.
package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/dto/response"
	"erg.ninja/pkg/logger"
)

// Recovery returns a Gin middleware that recovers from panics and returns a 500 error.
func Recovery(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				// Log the panic with stack trace.
				stack := debug.Stack()
				log.Error().
					Interface("panic", r).
					Str("stack", string(stack)).
					Str("request_id", GetRequestID(c)).
					Str("method", c.Request.Method).
					Str("path", c.Request.URL.Path).
					Msg("panic recovered in handler")

				response.ErrorGin(c, http.StatusInternalServerError, "ERR_INTERNAL", fmt.Sprintf("Internal server error: %v", r))
				c.Abort()
			}
		}()
		c.Next()
	}
}
