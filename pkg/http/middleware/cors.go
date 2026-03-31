package middleware

import (
	"net/http"
	"strings"

	"github.com/go-chi/cors"
)

// CORSOptions holds CORS configuration.
type CORSOptions struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           int
	Debug            bool
}

// CORSMiddleware returns a chi-compatible CORS middleware.
// If no origins are provided, it defaults to allowing all origins (useful for development).
func CORSMiddleware(opts CORSOptions) func(http.Handler) http.Handler {
	corsOpts := cors.Options{
		AllowedOrigins:   opts.AllowedOrigins,
		AllowedMethods:   opts.AllowedMethods,
		AllowedHeaders:   opts.AllowedHeaders,
		ExposedHeaders:   opts.ExposedHeaders,
		AllowCredentials: opts.AllowCredentials,
		MaxAge:           opts.MaxAge,
		Debug:            opts.Debug,
	}
	return cors.Handler(corsOpts)
}

// AllowAllCORSMiddleware is a permissive CORS handler for development.
func AllowAllCORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Only set CORS headers when an Origin header is present.
		if origin != "" && isAllowedOrigin(origin, []string{"*"}) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-Request-ID")
			w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
			w.Header().Set("Access-Control-Max-Age", "86400")
		}

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// isAllowedOrigin checks if an origin is in the allowed list.
// An empty allowed list or "*" means all origins are allowed.
func isAllowedOrigin(origin string, allowed []string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, a := range allowed {
		if a == "*" {
			return true
		}
		if strings.EqualFold(a, origin) {
			return true
		}
		// Support wildcard subdomains: *.example.com.
		if strings.HasPrefix(a, "*.") {
			domain := strings.TrimPrefix(a, "*.")
			host := strings.TrimPrefix(origin, "https://")
			host = strings.TrimPrefix(host, "http://")
			if strings.HasSuffix(host, domain) {
				return true
			}
		}
	}
	return false
}
