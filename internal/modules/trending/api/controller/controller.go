// Package trending
//
// @title ERG Trending Service API
// @version 1.0
// @description REST API for the ERG Trending Service — trending topics, news discovery, and feed management.
// @host localhost:8080
// @BasePath /
package controller

import (
	"context"
	"erg.ninja/internal/dto/response"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/middleware"
	trendingdto "erg.ninja/internal/modules/trending/api/dto"
	trendingservice "erg.ninja/internal/modules/trending/application/service"
	"erg.ninja/pkg/auth"
	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
)

type Controller struct {
	svc    *trendingservice.Service
	mongo  *database.MongoClient
	redis  *cache.RedisClient
	log    *logger.Logger
	jwtVal *auth.JWTValidator
}

func NewController(svc *trendingservice.Service, mongo *database.MongoClient, redis *cache.RedisClient, log *logger.Logger, jwtVal *auth.JWTValidator) *Controller {
	return &Controller{svc: svc, mongo: mongo, redis: redis, log: log, jwtVal: jwtVal}
}

func (c *Controller) RegisterRoutes(r *gin.Engine) {
	// Public health/ready — no auth required.
	r.GET("/api/trending/healthz", c.Healthz)
	r.GET("/api/trending/ready", c.Ready)

	api := r.Group("/api/trending")
	api.Use(middleware.JWTMiddleware(c.jwtVal))
	api.GET("/topics", c.ListTopics)
	api.GET("/topics/:topic", c.GetTopic)
	api.GET("/news", c.ListNews)
	api.GET("/snapshots", c.History)
	api.GET("/feeds", c.Feeds)
	api.GET("/discovery-feed", c.Feeds)
	api.GET("/history", c.History)
	api.GET("/sources", c.Sources)
	api.POST("/refresh", c.Refresh)
	api.POST("/discover", c.Refresh) // FE alias
	api.GET("/stats", c.Stats)       // Added for FE
}

// RegisterPublicRoutes registers public endpoints (healthz, ready).
func (c *Controller) RegisterPublicRoutes(r *gin.Engine) {
	r.GET("/api/trending/healthz", c.Healthz)
	r.GET("/api/trending/ready", c.Ready)
}

// RegisterProtectedRoutes registers authenticated endpoints under /api/trending/*.
func (c *Controller) RegisterProtectedRoutes(r *gin.RouterGroup) {
	r.GET("/topics", c.ListTopics)
	r.GET("/topics/:topic", c.GetTopic)
	r.GET("/news", c.ListNews)
	r.GET("/snapshots", c.History)
	r.GET("/feeds", c.Feeds)
	r.GET("/discovery-feed", c.Feeds)
	r.GET("/history", c.History)
	r.GET("/sources", c.Sources)
	r.POST("/refresh", c.Refresh)
}

// Healthz handles GET /api/trending/healthz.
// @Summary Trending health check
// @Description Returns trending module health status.
// @Tags Trending
// @Produce json
// @Success 200 {object} map[string]any
// @Router /api/trending/healthz [get]
func (c *Controller) Healthz(ctx *gin.Context) {
	lastRefresh, err := c.svc.LatestSnapshotTime(ctx.Request.Context())
	if err != nil {
		c.writeError(ctx, http.StatusInternalServerError, "HEALTH_FAILED", err.Error())
		return
	}
	c.writeJSON(ctx, http.StatusOK, map[string]any{
		"status":       "ok",
		"module":       "trending",
		"last_refresh": lastRefresh,
	})
}

// Ready handles GET /api/trending/ready.
// @Summary Trending readiness check
// @Description Returns trending module readiness with dep checks.
// @Tags Trending
// @Produce json
// @Success 200 {object} map[string]any
// @Router /api/trending/ready [get]
func (c *Controller) Ready(ctx *gin.Context) {
	reqCtx, cancel := context.WithTimeout(ctx.Request.Context(), 5*time.Second)
	defer cancel()

	body := map[string]any{"status": "ready", "module": "trending"}
	if err := c.mongo.Ping(reqCtx); err != nil {
		body["status"] = "not_ready"
		body["mongo_error"] = err.Error()
	}
	if err := c.redis.Ping(reqCtx); err != nil {
		body["status"] = "not_ready"
		body["redis_error"] = err.Error()
	}
	if ready, err := c.svc.Ready(reqCtx); err == nil {
		body["details"] = ready
	}
	status := http.StatusOK
	if body["status"] != "ready" {
		status = http.StatusServiceUnavailable
	}
	c.writeJSON(ctx, status, body)
}

// ListTopics handles GET /api/trending/topics.
// @Summary List trending topics
// @Description Returns current trending topics.
// @Tags Trending
// @Produce json
// @Param limit query int false "Limit"
// @Success 200 {object} map[string]any
// @Router /api/trending/topics [get]
func (c *Controller) ListTopics(ctx *gin.Context) {
	limit := int64(parseIntDefault(ctx.Query("limit"), 20))
	topics, err := c.svc.ListTopics(ctx.Request.Context(), limit)
	if err != nil {
		c.writeError(ctx, http.StatusInternalServerError, "LIST_TOPICS_FAILED", err.Error())
		return
	}
	c.writeJSON(ctx, http.StatusOK, map[string]any{"data": topics, "total": len(topics)})
}

// GetTopic handles GET /api/trending/topics/{topic}.
// @Summary Get trending topic detail
// @Description Returns details for a specific trending topic.
// @Tags Trending
// @Produce json
// @Param topic path string true "Topic slug"
// @Success 200 {object} map[string]any
// @Router /api/trending/topics/{topic} [get]
func (c *Controller) GetTopic(ctx *gin.Context) {
	topic, err := c.svc.GetTopic(ctx.Request.Context(), ctx.Param("topic"))
	if err != nil {
		c.writeError(ctx, http.StatusInternalServerError, "GET_TOPIC_FAILED", err.Error())
		return
	}
	if topic == nil {
		c.writeError(ctx, http.StatusNotFound, "TOPIC_NOT_FOUND", "topic not found")
		return
	}
	c.writeJSON(ctx, http.StatusOK, topic)
}

// ListNews handles GET /api/trending/news.
// @Summary List trending news
// @Description Returns latest trending news articles.
// @Tags Trending
// @Produce json
// @Param limit query int false "Limit"
// @Success 200 {object} map[string]any
// @Router /api/trending/news [get]
func (c *Controller) ListNews(ctx *gin.Context) {
	limit := int64(parseIntDefault(ctx.Query("limit"), 20))
	articles, err := c.svc.ListNews(ctx.Request.Context(), limit)
	if err != nil {
		c.writeError(ctx, http.StatusInternalServerError, "LIST_NEWS_FAILED", err.Error())
		return
	}
	c.writeJSON(ctx, http.StatusOK, map[string]any{"data": articles, "total": len(articles)})
}

// Feeds handles GET /api/trending/feeds.
// @Summary Get discovery feeds
// @Description Returns URLs discovered from trending topics.
// @Tags Trending
// @Produce json
// @Param limit query int false "Limit"
// @Success 200 {object} map[string]any
// @Router /api/trending/feeds [get]
func (c *Controller) Feeds(ctx *gin.Context) {
	limit := int64(parseIntDefault(ctx.Query("limit"), 100))
	urls, err := c.svc.DiscoveryFeed(ctx.Request.Context(), limit)
	if err != nil {
		c.writeError(ctx, http.StatusInternalServerError, "FEEDS_FAILED", err.Error())
		return
	}
	c.writeJSON(ctx, http.StatusOK, map[string]any{"data": urls, "total": len(urls)})
}

// History handles GET /api/trending/history.
// @Summary Get trending history
// @Description Returns historical trending data.
// @Tags Trending
// @Produce json
// @Param limit query int false "Limit"
// @Success 200 {object} map[string]any
// @Router /api/trending/history [get]
func (c *Controller) History(ctx *gin.Context) {
	limit := int64(parseIntDefault(ctx.Query("limit"), 20))
	history, err := c.svc.ListHistory(ctx.Request.Context(), limit)
	if err != nil {
		c.writeError(ctx, http.StatusInternalServerError, "HISTORY_FAILED", err.Error())
		return
	}
	c.writeJSON(ctx, http.StatusOK, map[string]any{"data": history, "total": len(history)})
}

// Sources handles GET /api/trending/sources.
// @Summary List trending sources
// @Description Returns configured trending data sources.
// @Tags Trending
// @Produce json
// @Success 200 {object} map[string]any
// @Router /api/trending/sources [get]
func (c *Controller) Sources(ctx *gin.Context) {
	body, err := c.svc.Sources(ctx.Request.Context())
	if err != nil {
		c.writeError(ctx, http.StatusInternalServerError, "SOURCES_FAILED", err.Error())
		return
	}
	c.writeJSON(ctx, http.StatusOK, body)
}

// Refresh handles POST /api/trending/refresh.
// @Summary Refresh trending data
// @Description Triggers a manual refresh of trending data.
// @Tags Trending
// @Produce json
// @Security BearerAuth
// @Success 202 {object} map[string]any
// @Router /api/trending/refresh [post]
func (c *Controller) Refresh(ctx *gin.Context) {
	result, err := c.svc.Refresh(ctx.Request.Context())
	if err != nil {
		c.writeError(ctx, http.StatusInternalServerError, "REFRESH_FAILED", err.Error())
		return
	}
	c.writeJSON(ctx, http.StatusAccepted, trendingdto.RefreshResponse{
		Status:      "refreshed",
		TopicCount:  len(result.Topics),
		NewsCount:   len(result.News),
		GeneratedAt: result.GeneratedAt.Format(time.RFC3339),
	})
}

// Stats handles GET /api/trending/stats.
// @Summary Get trending statistics
// @Description Returns discovery statistics for trending module.
// @Tags Trending
// @Produce json
// @Success 200 {object} dto.TrendingStats
// @Router /api/trending/stats [get]
func (c *Controller) Stats(ctx *gin.Context) {
	stats, err := c.svc.GetStats(ctx.Request.Context())
	if err != nil {
		c.writeError(ctx, http.StatusInternalServerError, "STATS_FAILED", err.Error())
		return
	}
	c.writeJSON(ctx, http.StatusOK, stats)
}

func (c *Controller) writeJSON(ctx *gin.Context, status int, v any) {
	response.WriteGin(ctx, status, v, nil, nil)
}

func (c *Controller) writeError(ctx *gin.Context, status int, code, message string) {
	response.ErrorGin(ctx, status, code, message)
}

func parseIntDefault(raw string, def int) int {
	if raw == "" {
		return def
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return def
	}
	return value
}
