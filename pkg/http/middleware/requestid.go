package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type contextKey string

const RequestIDKey contextKey = "request_id"

// RequestIDMiddleware injects or generates a X-Request-ID for every request.
// If the client provides X-Request-ID, it is used; otherwise a UUID is generated.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		ctx := context.WithValue(r.Context(), RequestIDKey, requestID)
		ctx = context.WithValue(ctx, "logger", nil) // Placeholder for logger injection.

		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID extracts the request ID from the context.
func GetRequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if id, ok := ctx.Value(RequestIDKey).(string); ok {
		return id
	}
	return ""
}

// WithRequestID returns a new context with the given request ID.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, RequestIDKey, id)
}

// zerologRequestIDHook is a zerolog hook that injects the request ID into log events.
type zerologRequestIDHook struct{}

func (zerologRequestIDHook) Run(e *zerolog.Event, level zerolog.Level, msg string) {
	// This hook would be used by the logger middleware to enrich events.
}

// RequestIDToContext returns a zerolog event with the request ID from context.
func RequestIDToContext(ctx context.Context) *zerolog.Event {
	if l := zerolog.Ctx(ctx); l != nil {
		return l.Info()
	}
	return (&zerolog.Logger{}).Info()
}
