// Package interceptors provides Gin-native HTTP middleware for the erg.ninja http package.
package interceptors

import (
	"context"
	"encoding/json"
	"net/http"
	"runtime/debug"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"erg.ninja/internal/middleware"
	ergerr "erg.ninja/pkg/errors"
)

// contextKey is a dedicated context value type to avoid collisions.
type contextKey string

const (
	// TraceIDKey is the context key for the OpenTelemetry trace ID.
	TraceIDKey contextKey = "trace_id"
)

type statusCaptureWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusCaptureWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// AddTraceID copies the incoming X-Trace-ID header into the request context so
// downstream handlers can include it in responses.
func AddTraceID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			traceID := r.Header.Get("X-Trace-ID")
			ctx := context.WithValue(r.Context(), TraceIDKey, traceID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ErrorInterceptor recovers panics from standard net/http handlers and returns
// the shared structured JSON error format used across the codebase.
func ErrorInterceptor(logger *zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					requestID := requestIDFromRequest(r)
					traceID, _ := r.Context().Value(TraceIDKey).(string)

					if logger != nil {
						logger.Error().
							Str("request_id", requestID).
							Str("trace_id", traceID).
							Str("method", r.Method).
							Str("path", r.URL.Path).
							Interface("panic", rec).
							Str("stack", string(debug.Stack())).
							Msg("http: panic recovered")
					}

					resp := ergerr.NewResponseWithRequestID(
						ergerr.ErrInternal,
						"An unexpected error occurred",
						requestID,
					)
					if traceID != "" {
						resp = resp.WithTraceID(traceID)
					}

					writeError(w, resp, http.StatusInternalServerError)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// GinErrorInterceptor is a gin-compatible middleware for error handling and recovery.
// It recovers from panics, logs them with stack traces, and returns structured JSON errors.
func GinErrorInterceptor(logger *zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Recovery from panics.
		defer func() {
			if rec := recover(); rec != nil {
				requestID := middleware.GetRequestID(c)
				if requestID == "" {
					requestID = "unknown"
				}
				traceID, _ := c.Get(string(TraceIDKey))
				stack := debug.Stack()

				if logger != nil {
					logger.Error().
						Str("request_id", requestID).
						Interface("trace_id", traceID).
						Str("method", c.Request.Method).
						Str("path", c.Request.URL.Path).
						Interface("panic", rec).
						Str("stack", string(stack)).
						Msg("http: panic recovered (gin)")
				}

				resp := ergerr.NewResponseWithRequestID(
					ergerr.ErrInternal,
					"An unexpected error occurred",
					requestID,
				)
				if tid, ok := traceID.(string); ok {
					resp = resp.WithTraceID(tid)
				}

				c.AbortWithStatusJSON(http.StatusInternalServerError, resp)
			}
		}()

		c.Next()
	}
}

// WrapHandler adapts error-returning handlers to standard net/http handlers.
func WrapHandler(handler func(http.ResponseWriter, *http.Request) error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				resp := ergerr.NewResponseWithRequestID(
					ergerr.ErrInternal,
					"An unexpected error occurred",
					requestIDFromRequest(r),
				)
				if traceID, _ := r.Context().Value(TraceIDKey).(string); traceID != "" {
					resp = resp.WithTraceID(traceID)
				}
				writeError(w, resp, http.StatusInternalServerError)
			}
		}()

		if err := handler(w, r); err != nil {
			resp := ergerr.FromError(err)
			if resp == nil {
				resp = ergerr.NewResponse(ergerr.ErrInternal, "An unexpected error occurred")
			}
			if resp.RequestID == "" {
				resp.RequestID = requestIDFromRequest(r)
			}
			if resp.TraceID == "" {
				if traceID, _ := r.Context().Value(TraceIDKey).(string); traceID != "" {
					resp.TraceID = traceID
				}
			}
			writeError(w, resp, resp.Code.ToHTTPStatus())
		}
	})
}

func writeError(w http.ResponseWriter, resp *ergerr.ErrorResponse, status int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if resp != nil {
		if resp.RequestID != "" {
			w.Header().Set("X-Request-ID", resp.RequestID)
		}
		if resp.TraceID != "" {
			w.Header().Set("X-Trace-ID", resp.TraceID)
		}
		if resp.RetryAfter > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(resp.RetryAfter))
		}
	}
	w.WriteHeader(status)
	if resp == nil {
		resp = ergerr.NewResponse(ergerr.ErrInternal, "An unexpected error occurred")
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func requestIDFromRequest(r *http.Request) string {
	if r == nil {
		return uuid.NewString()
	}
	if requestID := r.Header.Get("X-Request-ID"); requestID != "" {
		return requestID
	}
	if requestID := middleware.GetRequestIDFromContext(r.Context()); requestID != "" {
		return requestID
	}
	return uuid.NewString()
}
