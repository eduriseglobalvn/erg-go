package handlers

import (
	"context"
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/modules/bot/domain/entity"
	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
)

// HealthHandler handles health check and readiness endpoints.
type HealthHandler struct {
	mongo     *database.MongoClient
	redis     *cache.RedisClient
	log       *logger.Logger
	startTime time.Time
}

// NewHealthHandler creates a HealthHandler.
func NewHealthHandler(mongo *database.MongoClient, redis *cache.RedisClient, log *logger.Logger) *HealthHandler {
	return &HealthHandler{
		mongo:     mongo,
		redis:     redis,
		log:       log,
		startTime: time.Now(),
	}
}

// RegisterRoutes mounts health routes onto the given router.
func (h *HealthHandler) RegisterRoutes(r *gin.Engine) {
	r.GET("/healthz", h.Healthz)
	r.GET("/ready", h.Ready)
}

// Healthz handles GET /healthz — liveness probe.
// @Summary Bot health check
// @Description Returns bot service liveness status.
// @Tags Bot Health
// @Produce json
// @Success 200 {object} map[string]any
// @Router /healthz [get]
func (h *HealthHandler) Healthz(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "bot-service",
	})
}

// Ready handles GET /ready — readiness probe with dependency checks.
// @Summary Bot readiness check
// @Description Returns bot service readiness with dependency checks.
// @Tags Bot Health
// @Produce json
// @Success 200 {object} map[string]any
// @Router /ready [get]
func (h *HealthHandler) Ready(ctx *gin.Context) {
	reqCtx, cancel := context.WithTimeout(ctx.Request.Context(), 5*time.Second)
	defer cancel()

	type dependency struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		Error  string `json:"error,omitempty"`
	}

	var deps []dependency
	allUp := true

	// Check MongoDB.
	if h.mongo != nil {
		if err := h.mongo.Ping(reqCtx); err != nil {
			deps = append(deps, dependency{Name: "mongodb", Status: "down", Error: err.Error()})
			allUp = false
		} else {
			deps = append(deps, dependency{Name: "mongodb", Status: "up"})
		}
	}

	// Check Redis.
	if h.redis != nil {
		if err := h.redis.Ping(reqCtx); err != nil {
			deps = append(deps, dependency{Name: "redis", Status: "down", Error: err.Error()})
			allUp = false
		} else {
			deps = append(deps, dependency{Name: "redis", Status: "up"})
		}
	}

	status := "ready"
	httpStatus := http.StatusOK
	if !allUp {
		status = "not_ready"
		httpStatus = http.StatusServiceUnavailable
	}

	uptime := time.Since(h.startTime)
	convCount, _ := h.getActiveConversationCount(reqCtx)

	body := map[string]any{
		"status":       status,
		"service":      "bot-service",
		"uptime":       uptime.String(),
		"dependencies": deps,
		"runtime": map[string]string{
			"go_version":  runtime.Version(),
			"go_routines": itoa(runtime.NumGoroutine()),
			"memory_mb":   itoa(int(getMemMB())),
		},
		"conversations": map[string]any{
			"active": convCount,
		},
	}

	ctx.JSON(httpStatus, body)
}

// getActiveConversationCount returns the count of active conversations.
func (h *HealthHandler) getActiveConversationCount(ctx context.Context) (int64, error) {
	if h.mongo == nil {
		return 0, nil
	}
	coll := h.mongo.Collection(models.BotConversationCollection)
	return coll.CountDocuments(ctx, map[string]any{
		"state": "active",
	})
}

// getMemMB returns the current memory usage in MB.
func getMemMB() int64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	const maxInt64AsUint64 = uint64(1<<63 - 1)
	mb := m.Alloc / (1024 * 1024)
	if mb > maxInt64AsUint64 {
		return int64(maxInt64AsUint64)
	}
	return int64(mb)
}

// itoa converts an int to a string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if negative {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}
