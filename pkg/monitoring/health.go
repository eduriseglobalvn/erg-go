// Package monitoring provides background health monitoring for database connectivity.
package monitoring

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
)

// HealthMonitor periodically checks MongoDB and Redis connectivity.
// Logs warnings when degraded and recovers automatically when connections restore.
type HealthMonitor struct {
	mongo       *database.MongoClient
	redis       *cache.RedisClient
	log         *logger.Logger
	interval    time.Duration
	threshold   int // consecutive failures before logging warning
	consecutive int // current consecutive failure count
	stop        chan struct{}
}

// NewHealthMonitor creates a new health monitor.
// Default interval: 30s, threshold: 3 consecutive failures.
func NewHealthMonitor(
	mongo *database.MongoClient,
	redis *cache.RedisClient,
	log *logger.Logger,
	opts ...HealthMonitorOption,
) *HealthMonitor {
	h := &HealthMonitor{
		mongo:     mongo,
		redis:     redis,
		log:       log,
		interval:  30 * time.Second,
		threshold: 3,
		stop:      make(chan struct{}),
	}
	for _, o := range opts {
		o(h)
	}
	return h
}

// HealthMonitorOption configures the HealthMonitor.
type HealthMonitorOption func(*HealthMonitor)

// WithInterval sets the check interval.
func WithInterval(d time.Duration) HealthMonitorOption {
	return func(h *HealthMonitor) { h.interval = d }
}

// WithThreshold sets the consecutive failure threshold before warning.
func WithThreshold(n int) HealthMonitorOption {
	return func(h *HealthMonitor) { h.threshold = n }
}

// Start begins the background health check loop.
// Returns immediately; checks run in a goroutine.
func (h *HealthMonitor) Start(ctx context.Context) {
	go func() {
		// Run an immediate check on startup.
		h.check(ctx)

		ticker := time.NewTicker(h.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				h.log.Info().Msg("health-monitor: context cancelled, stopping")
				return
			case <-h.stop:
				h.log.Info().Msg("health-monitor: stopped")
				return
			case <-ticker.C:
				h.check(ctx)
			}
		}
	}()
	h.log.Info().Dur("interval", h.interval).Msg("health-monitor: started")
}

// Stop halts the health check loop.
func (h *HealthMonitor) Stop() {
	close(h.stop)
}

// check performs a single health check on MongoDB and Redis.
func (h *HealthMonitor) check(ctx context.Context) {
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var issues []string

	// Check MongoDB.
	if err := h.mongo.Ping(checkCtx); err != nil {
		issues = append(issues, fmt.Sprintf("mongo: %v", err))
		healthMetric.WithLabelValues("mongo").Set(0)
	} else {
		healthMetric.WithLabelValues("mongo").Set(1)
	}

	// Check Redis.
	if err := h.redis.Ping(checkCtx); err != nil {
		issues = append(issues, fmt.Sprintf("redis: %v", err))
		healthMetric.WithLabelValues("redis").Set(0)
	} else {
		healthMetric.WithLabelValues("redis").Set(1)
	}

	if len(issues) > 0 {
		h.consecutive++
		if h.consecutive >= h.threshold {
			h.log.Warn().
				Int("consecutive_failures", h.consecutive).
				Strs("issues", issues).
				Msg("health-monitor: DB connectivity degraded")
		}
	} else {
		if h.consecutive >= h.threshold {
			h.log.Info().
				Int("previous_failures", h.consecutive).
				Msg("health-monitor: DB connectivity restored")
		}
		h.consecutive = 0
	}
}

// ─── Prometheus metrics ────────────────────────────────────────────────────────

var healthMetric = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "db_health",
	Help: "Database health: 1 = healthy, 0 = degraded",
}, []string{"db"}) // labels: "mongo", "redis"
