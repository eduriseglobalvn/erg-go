// Package http provides chi router server setup, middleware, and an HTTP client
// with retry, circuit breaker, and structured logging.
package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/pprof"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/telemetry"

	mw "erg.ninja/pkg/http/middleware"
)

// Server wraps a chi.Router with lifecycle management and dependency injection.
type Server struct {
	router   *chi.Mux
	httpSrv  *http.Server
	log      *logger.Logger
	cfg      config.HTTPConfig
	mongo    *database.MongoClient
	redis    *cache.RedisClient
	registry *telemetry.PrometheusRegistry
}

// ServerOption configures a Server.
type ServerOption func(*Server)

// WithMongo attaches the MongoDB client for health checks.
func WithMongo(m *database.MongoClient) ServerOption {
	return func(s *Server) {
		s.mongo = m
	}
}

// WithRedis attaches the Redis client for health checks.
func WithRedis(r *cache.RedisClient) ServerOption {
	return func(s *Server) {
		s.redis = r
	}
}

// WithPrometheus attaches a Prometheus registry for /metrics endpoint.
func WithPrometheus(r *telemetry.PrometheusRegistry) ServerOption {
	return func(s *Server) {
		s.registry = r
	}
}

// NewServer creates a new HTTP server with the standard middleware chain.
// Standard chain: Recovery → RequestID → RealIP → Logger → CORS → RateLimit.
func NewServer(cfg config.HTTPConfig, log *logger.Logger, opts ...ServerOption) *Server {
	s := &Server{
		cfg: cfg,
		log: log,
	}
	for _, o := range opts {
		o(s)
	}

	s.router = chi.NewRouter()
	s.applyMiddleware(cfg, log)

	return s
}

// applyMiddleware installs the standard middleware stack.
func (s *Server) applyMiddleware(cfg config.HTTPConfig, log *logger.Logger) {
	// Recovery — panic recovery with request ID.
	s.router.Use(chiMiddleware.Recoverer)

	// Request ID — inject/generate X-Request-ID.
	s.router.Use(mw.RequestIDMiddleware)

	// Real IP — extract real client IP from X-Forwarded-For / X-Real-IP.
	s.router.Use(mw.RealIPMiddleware)

	// Structured request logging via chi's built-in logger.
	s.router.Use(chiMiddleware.Logger)

	// CORS — Cross-Origin Resource Sharing.
	corsOpts := cors.Options{
		AllowedOrigins:   cfg.CORS.AllowedOrigins,
		AllowedMethods:   cfg.CORS.AllowedMethods,
		AllowedHeaders:   cfg.CORS.AllowedHeaders,
		ExposedHeaders:   cfg.CORS.ExposedHeaders,
		AllowCredentials: cfg.CORS.AllowCredentials,
		MaxAge:           cfg.CORS.MaxAge,
	}
	s.router.Use(cors.Handler(corsOpts))

	// Rate limiting — in-memory token bucket per IP.
	if cfg.RateLimit.Enabled {
		s.router.Use(mw.RateLimitMiddleware(
			mw.WithRateLimitRequests(cfg.RateLimit.RequestsPerMinute),
			mw.WithRateLimitBurst(cfg.RateLimit.Burst),
		))
	}

	// Compress responses (gzip).
	s.router.Use(chiMiddleware.Compress(5))
}

// Mount mounts a sub-router at the given path.
func (s *Server) Mount(path string, handler http.Handler) {
	s.router.Mount(path, handler)
}

// MountFunc mounts a chi router handler function at the given path.
func (s *Server) MountFunc(path string, fn http.HandlerFunc) {
	s.router.Mount(path, http.HandlerFunc(fn))
}

// Handle registers an HTTP handler.
func (s *Server) Handle(pattern string, handler http.Handler) {
	s.router.Handle(pattern, handler)
}

// HandleFunc registers an HTTP handler function.
func (s *Server) HandleFunc(pattern string, fn http.HandlerFunc) {
	s.router.HandleFunc(pattern, fn)
}

// Method registers a handler for an HTTP method at a path.
func (s *Server) Method(method, pattern string, handler http.Handler) {
	s.router.Method(method, pattern, handler)
}

// MethodFunc registers a handler function for an HTTP method at a path.
func (s *Server) MethodFunc(method, pattern string, fn http.HandlerFunc) {
	s.router.MethodFunc(method, pattern, fn)
}

// Get registers a GET handler.
func (s *Server) Get(pattern string, fn http.HandlerFunc) {
	s.router.Get(pattern, fn)
}

// Post registers a POST handler.
func (s *Server) Post(pattern string, fn http.HandlerFunc) {
	s.router.Post(pattern, fn)
}

// Put registers a PUT handler.
func (s *Server) Put(pattern string, fn http.HandlerFunc) {
	s.router.Put(pattern, fn)
}

// Delete registers a DELETE handler.
func (s *Server) Delete(pattern string, fn http.HandlerFunc) {
	s.router.Delete(pattern, fn)
}

// Patch registers a PATCH handler.
func (s *Server) Patch(pattern string, fn http.HandlerFunc) {
	s.router.Patch(pattern, fn)
}

// MountHealthRoutes attaches /healthz and /ready endpoints.
func (s *Server) MountHealthRoutes() {
	s.router.Get("/healthz", s.handleHealthz)
	s.router.Get("/ready", s.handleReady)
}

// MountDebugRoutes attaches pprof routes in development mode.
func (s *Server) MountDebugRoutes() {
	s.router.HandleFunc("/debug/pprof/", pprof.Index)
	s.router.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	s.router.HandleFunc("/debug/pprof/profile", pprof.Profile)
	s.router.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	s.router.HandleFunc("/debug/pprof/trace", pprof.Trace)
	s.router.HandleFunc("/debug/pprof/allocs", pprof.Handler("allocs").ServeHTTP)
	s.router.HandleFunc("/debug/pprof/heap", pprof.Handler("heap").ServeHTTP)
	s.router.HandleFunc("/debug/pprof/block", pprof.Handler("block").ServeHTTP)
	s.router.HandleFunc("/debug/pprof/mutex", pprof.Handler("mutex").ServeHTTP)
}

// MountMetrics attaches the Prometheus metrics endpoint.
func (s *Server) MountMetrics() {
	if s.registry != nil {
		s.router.Handle("/metrics", s.registry.Handler())
	}
}

// handleHealthz is a liveness probe: returns 200 if the server is running.
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// handleReady checks all dependencies: MongoDB ping + Redis ping.
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	type dependency struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		Error  string `json:"error,omitempty"`
	}

	deps := []dependency{}

	// Check MongoDB.
	if s.mongo != nil {
		if err := s.mongo.Ping(ctx); err != nil {
			deps = append(deps, dependency{Name: "mongodb", Status: "down", Error: err.Error()})
		} else {
			deps = append(deps, dependency{Name: "mongodb", Status: "up"})
		}
	}

	// Check Redis.
	if s.redis != nil {
		if err := s.redis.Ping(ctx); err != nil {
			deps = append(deps, dependency{Name: "redis", Status: "down", Error: err.Error()})
		} else {
			deps = append(deps, dependency{Name: "redis", Status: "up"})
		}
	}

	// Determine overall status.
	status := "ready"
	httpStatus := http.StatusOK
	for _, d := range deps {
		if d.Status == "down" {
			status = "not_ready"
			httpStatus = http.StatusServiceUnavailable
			break
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)

	resp := map[string]interface{}{"status": status, "dependencies": deps}
	_ = writeJSON(w, resp)
}

// ServeHTTP makes the Server implement http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// Router returns the underlying chi router.
func (s *Server) Router() *chi.Mux {
	return s.router
}

// Start begins listening on the configured address. It blocks.
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	s.httpSrv = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  s.cfg.ReadTimeout,
		WriteTimeout: s.cfg.WriteTimeout,
		IdleTimeout:  s.cfg.IdleTimeout,
	}

	s.log.Info().
		Str("addr", addr).
		Dur("read_timeout", s.cfg.ReadTimeout).
		Dur("write_timeout", s.cfg.WriteTimeout).
		Msg("http server starting")

	return s.httpSrv.ListenAndServe()
}

// StartTLS begins listening with TLS on the configured address.
func (s *Server) StartTLS(certFile, keyFile string) error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	s.httpSrv = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  s.cfg.ReadTimeout,
		WriteTimeout: s.cfg.WriteTimeout,
		IdleTimeout:  s.cfg.IdleTimeout,
	}

	return s.httpSrv.ListenAndServeTLS(certFile, keyFile)
}

// Shutdown gracefully shuts down the server with a timeout.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpSrv == nil {
		return nil
	}
	s.log.Info().Msg("http server shutting down")
	return s.httpSrv.Shutdown(ctx)
}

// writeJSON writes a JSON response with consistent formatting.
func writeJSON(w http.ResponseWriter, v interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
