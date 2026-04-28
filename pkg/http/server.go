// Package http provides gin router server setup, middleware, and an HTTP client
// with retry, circuit breaker, and structured logging.
package http

import (
	"context"
	"fmt"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"

	"github.com/rs/zerolog"

	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/telemetry"

	"erg.ninja/internal/middleware"
	"erg.ninja/pkg/http/interceptors"
)

// Server wraps a gin.Engine with lifecycle management and dependency injection.
type Server struct {
	router   *gin.Engine
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

	s.router = gin.New()
	s.applyMiddleware(cfg, log)

	return s
}

// applyMiddleware installs the standard middleware stack.
func (s *Server) applyMiddleware(cfg config.HTTPConfig, log *logger.Logger) {
	// Recovery — panic recovery.
	s.router.Use(gin.Recovery())

	// Error interceptor — converts panics and plain errors to structured JSON.
	s.router.Use(interceptors.GinErrorInterceptor(log.Zerolog()))

	// Real IP — extract real client IP from X-Forwarded-For / X-Real-IP.
	s.router.Use(middleware.RealIP())

	// Request ID — inject/generate X-Request-ID.
	s.router.Use(middleware.RequestID())

	// Structured request logging via zerolog.
	s.router.Use(requestLogger(log.Zerolog()))

	// CORS — Cross-Origin Resource Sharing.
	s.router.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.CORS.AllowedOrigins,
		AllowMethods:     cfg.CORS.AllowedMethods,
		AllowHeaders:     cfg.CORS.AllowedHeaders,
		ExposeHeaders:    cfg.CORS.ExposedHeaders,
		AllowCredentials: cfg.CORS.AllowCredentials,
		MaxAge:           time.Duration(cfg.CORS.MaxAge) * time.Second,
	}))

	// Rate limiting — in-memory token bucket per IP.
	if cfg.RateLimit.Enabled {
		s.router.Use(middleware.RateLimit(s.redis, middleware.RateLimitConfig{
			RequestsPerMinute: cfg.RateLimit.RequestsPerMinute,
			Burst:             cfg.RateLimit.Burst,
		}))
	}

	// Compress responses (gzip).
	s.router.Use(gzip.Gzip(gzip.BestCompression))
}

// MountFunc mounts a gin router handler function at the given path.
// Use this to register sub-routers or grouped routes.
func (s *Server) MountFunc(path string, fn func(r *gin.RouterGroup)) {
	fn(s.router.Group(path))
}



// MountHealthRoutes attaches /healthz and /ready endpoints using Gin-native handlers.
func (s *Server) MountHealthRoutes() {
	s.router.GET("/healthz", s.handleHealthz)
	s.router.GET("/ready", s.handleReady)
}

// MountDebugRoutes attaches pprof routes in development mode.
func (s *Server) MountDebugRoutes() {
	s.router.Any("/debug/pprof/", gin.WrapF(pprof.Index))
	s.router.Any("/debug/pprof/cmdline", gin.WrapF(pprof.Cmdline))
	s.router.Any("/debug/pprof/profile", gin.WrapF(pprof.Profile))
	s.router.Any("/debug/pprof/symbol", gin.WrapF(pprof.Symbol))
	s.router.Any("/debug/pprof/trace", gin.WrapF(pprof.Trace))
	s.router.Any("/debug/pprof/allocs", gin.WrapH(pprof.Handler("allocs")))
	s.router.Any("/debug/pprof/heap", gin.WrapH(pprof.Handler("heap")))
	s.router.Any("/debug/pprof/block", gin.WrapH(pprof.Handler("block")))
	s.router.Any("/debug/pprof/mutex", gin.WrapH(pprof.Handler("mutex")))
}

// MountMetrics attaches the Prometheus metrics endpoint.
func (s *Server) MountMetrics() {
	if s.registry != nil {
		s.router.GET("/metrics", gin.WrapH(s.registry.Handler()))
	}
}

// handleHealthz is a liveness probe: returns 200 if the server is running.
func (s *Server) handleHealthz(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// handleReady checks all dependencies: MongoDB ping + Redis ping.
func (s *Server) handleReady(ctx *gin.Context) {
	reqCtx := ctx.Request.Context()
	type dependency struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		Error  string `json:"error,omitempty"`
	}

	deps := []dependency{}

	// Check MongoDB.
	if s.mongo != nil {
		if err := s.mongo.Ping(reqCtx); err != nil {
			deps = append(deps, dependency{Name: "mongodb", Status: "down", Error: err.Error()})
		} else {
			deps = append(deps, dependency{Name: "mongodb", Status: "up"})
		}
	}

	// Check Redis.
	if s.redis != nil {
		if err := s.redis.Ping(reqCtx); err != nil {
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

	ctx.JSON(httpStatus, gin.H{
		"status":       status,
		"dependencies": deps,
	})
}

// ServeHTTP makes the Server implement http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// Router returns the underlying gin engine.
func (s *Server) Router() *gin.Engine {
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



// requestLogger is a zerolog-based request logger compatible with Gin.
func requestLogger(log *zerolog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Log after request is processed.
		c.Next()

		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = "unknown"
		}
		log.Info().
			Str("request_id", requestID).
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Int("status", c.Writer.Status()).
			Str("client_ip", c.ClientIP()).
			Msg("http request")
	}
}
