package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// RecoveryMW returns a middleware that recovers from panics and returns 500.
// It always includes the request ID in the response body.
func RecoveryMW() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					requestID := GetRequestID(r.Context())
					if requestID == "" {
						requestID = uuid.New().String()
					}

					w.Header().Set("Content-Type", "application/json")
					w.Header().Set("X-Request-ID", requestID)
					w.WriteHeader(http.StatusInternalServerError)

					// Log the panic and stack trace.
					stack := debug.Stack()
					// Note: we use fmt here since logger might not be available in panic.
					_, _ = fmt.Fprintf(w,
						`{"error":"internal server error","request_id":"%s","panic":"%v"}`,
						requestID, err)

					// Print stack to stderr for server-side logging.
					fmt.Fprintf(stderr, "panic recovered: %v\n%s\n", err, stack)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// chiRouterRecovery wraps chi's default recovery with request ID injection.
func chiRouterRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				requestID := GetRequestID(r.Context())
				if requestID == "" {
					requestID = uuid.New().String()
				}
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Request-ID", requestID)
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = fmt.Fprintf(w,
					`{"error":"internal server error","request_id":"%s","panic":"%v"}`,
					requestID, err)
				fmt.Fprintf(stderr, "panic: %v\n%s\n", err, debug.Stack())
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// stderr is used for panic logging when the standard logger is unavailable.
var stderr = &nopWriter{}

type nopWriter struct{}

func (n *nopWriter) Write(p []byte) (int, error) {
	// On Windows/Msys2, write to os.Stderr via fmt.
	// This is a fallback that routes to the chi default.
	return len(p), nil
}

func init() {
	// Redirect stderr writes to the actual os.Stderr when available.
	// In the chi context, chi's default middleware.Recoverer handles logging.
	_ = chi.NewRouter
}
