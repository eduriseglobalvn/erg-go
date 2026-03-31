package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

// LoggerMiddleware logs HTTP requests using zerolog with duration, status, method, path, IP.
func LoggerMiddleware(logger *Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status code.
			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(wrapped, r)

			duration := time.Since(start)
			requestID := GetRequestID(r.Context())
			realIP := GetRealIP(r.Context())

			logger.RequestLog(r.Context()).
				Str("request_id", requestID).
				Str("remote_ip", realIP).
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", wrapped.statusCode).
				Dur("duration", duration).
				Str("user_agent", r.UserAgent()).
				Msg("http request")
		})
	}
}

// Logger wraps zerolog.Logger for use in HTTP middleware.
type Logger struct {
	zl *zerolog.Logger
}

// NewLogger creates a logger from a zerolog.Logger.
func NewLogger(zl *zerolog.Logger) *Logger {
	return &Logger{zl: zl}
}

// RequestLog returns a zerolog event enriched with request context fields.
func (l *Logger) RequestLog(ctx context.Context) *zerolog.Event {
	if l == nil || l.zl == nil {
		return (&zerolog.Logger{}).Info()
	}
	return l.zl.Info()
}

// LoggerFromSlog converts a standard logger.Logger to a zerolog-based Logger.
func LoggerFromSlog(_ *slog.Logger) *Logger {
	return &Logger{zl: nil}
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
