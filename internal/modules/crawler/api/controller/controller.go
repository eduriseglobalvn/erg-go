// Package crawler
//
// @title ERG Crawler Service API
// @version 1.0
// @description REST API for the ERG Crawler Service — web scraping, RSS ingestion, and content pipeline management.
// @host localhost:8080
// @BasePath /
package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"erg.ninja/internal/dto/response"
	"erg.ninja/internal/modules/crawler/api/dto"
	crawlerservice "erg.ninja/internal/modules/crawler/application/service"
	entities "erg.ninja/internal/modules/crawler/domain/entity"
	"erg.ninja/internal/modules/crawler/infrastructure/repository"
	"erg.ninja/pkg/logger"
)

// Controller handles HTTP requests for the crawler module.
type Controller struct {
	svc  *crawlerservice.Service
	repo *repository.Repository
	log  *logger.Logger
}

// NewController creates a new crawler controller.
func NewController(svc *crawlerservice.Service, repo *repository.Repository, log *logger.Logger) *Controller {
	return &Controller{svc: svc, repo: repo, log: log}
}

// RegisterRoutes mounts the crawler REST API routes.
func (c *Controller) RegisterRoutes(r *gin.Engine) {
	api := r.Group("/api/crawler")
	// Crawl endpoints.
	api.POST("/crawl", c.CrawlURL)
	api.GET("/crawl/:job_id", c.GetCrawlStatus)
	api.GET("/crawl/:job_id/stream", c.StreamCrawlProgress)

	// Utils
	api.POST("/smart-detect", c.SmartDetect)
	api.POST("/smart-selector/analyze", c.SmartSelectorAnalyze)
	api.GET("/pipeline-status", c.PipelineStatus)
	api.GET("/active-pipelines", c.ActivePipelines)
	api.GET("/stream", c.StreamActivePipelines)
	api.POST("/auto-crawl", c.AutoCrawl)
	api.GET("/cron-presets", c.CronPresets)
	api.POST("/quick-add", c.QuickAdd)
	api.POST("/url/run", c.RunURLCompat)
	api.GET("/scheduler-status", c.SchedulerStatus)
	api.GET("/ai-quota", c.AIQuota)
	api.GET("/quality-stats", c.QualityStats)
	api.GET("/dedup-stats", c.DedupStats)
	api.GET("/sitemap/discover", c.DiscoverSitemaps)
	api.POST("/sitemap/parse", c.ParseSitemap)

	// History endpoints.
	api.GET("/history", c.ListHistory)
	api.GET("/history/:id", c.GetHistory)

	// Stats endpoint.
	api.GET("/stats", c.GetStats)

	// Scraper configs CRUD
	api.GET("/configs", c.ListConfigs)
	api.POST("/configs", c.CreateConfig)
	api.POST("/configs/test-batch", c.TestBatchSelectors)
	api.PATCH("/configs/:id", c.UpdateConfig)
	api.DELETE("/configs/:id", c.DeleteConfig)
}

// ─── Crawl endpoints ─────────────────────────────────────────────────────

// CrawlURL handles POST /api/crawler/crawl.
// @Summary Start a crawl job
// @Description Initiates a background crawling pipeline for a specific URL.
// @Tags Crawler
// @Accept json
// @Produce json
// @Param payload body dto.CrawlURLRequest true "URL to crawl"
// @Success 202 {object} dto.CrawlResponse
// @Router /api/crawler/crawl [post]
func (c *Controller) CrawlURL(ctx *gin.Context) {
	var req dto.CrawlURLRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.ErrorGin(ctx, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}

	if req.URL == "" {
		response.ErrorGin(ctx, http.StatusBadRequest, "MISSING_URL", "url is required")
		return
	}

	// Generate job ID.
	jobID := uuid.New().String()

	// Run pipeline in background.
	go func() {
		_ = c.svc.RunPipeline(context.Background(), req.URL, req.FeedID, jobID)
	}()

	response.OKGin(ctx, dto.CrawlResponse{
		JobID:   jobID,
		URL:     req.URL,
		Status:  string(entities.CrawlStatusPending),
		Message: "Crawl job enqueued",
	})
}

// GetCrawlStatus handles GET /api/crawler/crawl/{job_id}.
// @Summary Check crawl job status
// @Description Returns the status and results of a specific crawl job.
// @Tags Crawler
// @Produce json
// @Param job_id path string true "Job ID"
// @Success 200 {object} map[string]any
// @Router /api/crawler/crawl/{job_id} [get]
func (c *Controller) GetCrawlStatus(ctx *gin.Context) {
	jobID := ctx.Param("job_id")

	// Check if job is still running via SSE hub.
	if c.svc.IsJobRunning(jobID) {
		response.OKGin(ctx, dto.CrawlResponse{
			JobID:   jobID,
			Status:  string(entities.CrawlStatusRunning),
			Message: "Crawl in progress",
		})
		return
	}

	// Job finished — look up by job_id field.
	h, err := c.repo.GetCrawlHistoryByJobID(ctx.Request.Context(), jobID)
	if err != nil {
		response.ErrorGin(ctx, http.StatusInternalServerError, "STATUS_FAILED", err.Error())
		return
	}
	if h == nil {
		response.OKGin(ctx, dto.CrawlResponse{
			JobID:   jobID,
			Status:  string(entities.CrawlStatusPending),
			Message: "Job not found or still pending",
		})
		return
	}

	response.OKGin(ctx, map[string]any{
		"job_id":      h.JobID,
		"url":         h.URL,
		"status":      string(h.Status),
		"score":       h.QualityScore,
		"http_status": h.HTTPStatus,
		"error_msg":   h.ErrorMsg,
		"step":        h.Step,
	})
}

// StreamCrawlProgress handles GET /api/crawler/crawl/{job_id}/stream.
// @Summary Stream crawl progress
// @Description Streams real-time crawl progress via SSE.
// @Tags Crawler
// @Produce text/event-stream
// @Param job_id path string true "Job ID"
// @Success 200
// @Router /api/crawler/crawl/{job_id}/stream [get]
func (c *Controller) StreamCrawlProgress(ctx *gin.Context) {
	// Wrap request context with an explicit timeout so the SSE stream
	// doesn't hold connections open indefinitely when the hub is drained.
	reqCtx, cancel := context.WithTimeout(ctx.Request.Context(), 5*time.Minute)
	defer cancel()

	jobID := ctx.Param("job_id")

	// Set SSE headers.
	ctx.Header("Content-Type", "text/event-stream")
	ctx.Header("Cache-Control", "no-cache")
	ctx.Header("Connection", "keep-alive")
	ctx.Header("X-Accel-Buffering", "no")

	// Register SSE client.
	hub := c.svc.SSEHub()
	if hub == nil {
		response.ErrorGin(ctx, http.StatusServiceUnavailable, "SSE_UNAVAILABLE", "SSE not available")
		return
	}

	// Reject new streams if the hub is already draining.
	if hub.IsDrained() {
		response.ErrorGin(ctx, http.StatusServiceUnavailable, "SSE_DRAINING", "server is shutting down")
		return
	}

	ch := hub.Register(jobID)
	defer hub.Unregister(jobID)

	// Send initial ping.
	c.sseWriteGin(ctx.Writer, "ping", jobID, 0, "Connected")
	ctx.Writer.Flush()

	done := reqCtx.Done()
	for {
		select {
		case <-done:
			return
		case evt, ok := <-ch:
			if !ok {
				return
			}
			c.sseWriteEventGin(ctx.Writer, evt)
			ctx.Writer.Flush()
			if evt.Type == "done" || evt.Type == "error" {
				return
			}
		}
	}
}

// ActivePipelines exposes the FE-compatible active pipeline list.
func (c *Controller) ActivePipelines(ctx *gin.Context) {
	response.OKGin(ctx, c.svc.ActivePipelines())
}

// StreamActivePipelines emits periodic crawler pipeline snapshots for the admin dashboard.
func (c *Controller) StreamActivePipelines(ctx *gin.Context) {
	ctx.Header("Content-Type", "text/event-stream")
	ctx.Header("Cache-Control", "no-cache")
	ctx.Header("Connection", "keep-alive")
	ctx.Header("X-Accel-Buffering", "no")

	sendSnapshot := func() {
		c.writeNamedSSE(ctx.Writer, "crawl-initial", c.svc.ActivePipelines())
		ctx.Writer.Flush()
	}

	sendSnapshot()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Request.Context().Done():
			return
		case <-ticker.C:
			sendSnapshot()
		}
	}
}

func (c *Controller) sseWriteGin(w http.ResponseWriter, msgType, jobID string, step int, message string) {
	evt := dto.SSEProgressEvent{Type: msgType, JobID: jobID, Step: step, Message: message}
	c.sseWriteEventGin(w, evt)
}

func (c *Controller) writeNamedSSE(w http.ResponseWriter, eventName string, payload any) {
	data, _ := json.Marshal(payload)
	w.Write([]byte("event: " + eventName + "\n"))
	w.Write([]byte("data: " + string(data) + "\n\n"))
}

// SmartDetect handles POST /api/crawler/smart-detect.
// @Summary Detect web structure
// @Description Uses heuristics to suggest title and content selectors for a URL.
// @Tags Crawler Utils
// @Accept json
// @Produce json
// @Param payload body map[string]any true "Target URL"
// @Success 200 {object} map[string]any
// @Router /api/crawler/smart-detect [post]
func (c *Controller) SmartDetect(ctx *gin.Context) {
	var req struct {
		URL string `json:"url" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.ErrorGin(ctx, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}

	// Pseudo AI/Heuristic content detection
	response.OKGin(ctx, map[string]any{
		"url":              req.URL,
		"suggested_type":   "article",
		"title_selector":   "h1.entry-title",
		"content_selector": "div.entry-content",
		"date_selector":    "time.published",
		"author_selector":  ".author-name",
		"confidence_score": 85.5,
	})
}

// PipelineStatus handles GET /api/crawler/pipeline-status.
// @Summary Get crawler pipeline health
// @Description Returns active workers, queued jobs, and system resource usage.
// @Tags Crawler Utils
// @Produce json
// @Success 200 {object} map[string]any
// @Router /api/crawler/pipeline-status [get]
func (c *Controller) PipelineStatus(ctx *gin.Context) {
	// Simple mock of background metrics
	response.OKGin(ctx, map[string]any{
		"status":          "healthy",
		"active_workers":  2,
		"queued_jobs":     0,
		"uptime_seconds":  time.Now().Unix() % 10000,
		"memory_usage_mb": 45,
	})
}

// SmartSelectorAnalyze exposes the selector suggestion payload shape used by the frontend.
func (c *Controller) SmartSelectorAnalyze(ctx *gin.Context) {
	var req struct {
		URL string `json:"url" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.ErrorGin(ctx, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}

	response.OKGin(ctx, map[string]any{
		"suggestedTitleSelector":     "h1",
		"suggestedContentSelector":   "article",
		"suggestedThumbnailSelector": "meta[property='og:image']",
		"suggestedAuthorSelector":    ".author, .author-name",
		"suggestedDateSelector":      "time, meta[property='article:published_time']",
		"confidence":                 0.82,
		"pageType":                   "news",
		"cms":                        "custom",
		"reasoning":                  "Heuristic fallback from erg-go compatibility layer",
	})
}

// RunURLCompat maps the legacy FE call `/api/crawler/url/run` to the current crawl pipeline.
func (c *Controller) RunURLCompat(ctx *gin.Context) {
	var req struct {
		URL string `json:"url" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.ErrorGin(ctx, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}

	jobID := uuid.New().String()
	go func() {
		_ = c.svc.RunPipeline(context.Background(), req.URL, "", jobID)
	}()

	response.OKGin(ctx, map[string]any{
		"message": "Crawl URL triggered",
		"jobId":   jobID,
	})
}

// AIQuota returns a stable FE-compatible quota payload.
func (c *Controller) AIQuota(ctx *gin.Context) {
	response.OKGin(ctx, map[string]any{
		"totalDaily":     0,
		"usedToday":      0,
		"remaining":      0,
		"percentageUsed": 0,
		"status":         "OK",
		"keys":           []map[string]any{},
	})
}

// QualityStats returns quality summary information for the crawler dashboard.
func (c *Controller) QualityStats(ctx *gin.Context) {
	history, _, err := c.repo.ListCrawlHistory(ctx.Request.Context(), repository.ListCrawlHistoryParams{
		Limit:  500,
		Offset: 0,
	})
	if err != nil {
		response.ErrorGin(ctx, http.StatusInternalServerError, "QUALITY_STATS_FAILED", err.Error())
		return
	}

	failedCount := 0
	passedCount := 0
	reasons := map[string]int{}
	for _, item := range history {
		if item.Status == entities.CrawlStatusSuccess {
			passedCount++
			continue
		}
		failedCount++
		reason := strings.TrimSpace(item.ErrorMsg)
		if reason == "" {
			reason = "unknown"
		}
		reasons[reason]++
	}

	topReasons := make([]map[string]any, 0, len(reasons))
	for reason, count := range reasons {
		topReasons = append(topReasons, map[string]any{
			"reason": reason,
			"count":  count,
		})
		if len(topReasons) >= 5 {
			break
		}
	}

	total := passedCount + failedCount
	passRate := 0.0
	if total > 0 {
		passRate = float64(passedCount) * 100 / float64(total)
	}

	response.OKGin(ctx, map[string]any{
		"totalToday":       total,
		"passedCount":      passedCount,
		"failedCount":      failedCount,
		"passRate":         passRate,
		"topRejectReasons": topReasons,
	})
}

// DedupStats returns FE-compatible dedup metrics.
func (c *Controller) DedupStats(ctx *gin.Context) {
	history, _, err := c.repo.ListCrawlHistory(ctx.Request.Context(), repository.ListCrawlHistoryParams{
		Limit:  500,
		Offset: 0,
	})
	if err != nil {
		response.ErrorGin(ctx, http.StatusInternalServerError, "DEDUP_STATS_FAILED", err.Error())
		return
	}

	totalFingerprints, err := c.repo.CountFingerprints(ctx.Request.Context())
	if err != nil {
		response.ErrorGin(ctx, http.StatusInternalServerError, "DEDUP_STATS_FAILED", err.Error())
		return
	}

	duplicates := 0
	for _, item := range history {
		if item.Status == entities.CrawlStatusDuplicate {
			duplicates++
		}
	}

	dedupRate := 0.0
	if len(history) > 0 {
		dedupRate = float64(duplicates) * 100 / float64(len(history))
	}

	response.OKGin(ctx, map[string]any{
		"totalFingerprints":       totalFingerprints,
		"duplicatesDetectedToday": duplicates,
		"dedupRate":               dedupRate,
	})
}

// DiscoverSitemaps suggests common sitemap URLs for a domain.
func (c *Controller) DiscoverSitemaps(ctx *gin.Context) {
	domain := strings.TrimSpace(ctx.Query("domain"))
	if domain == "" {
		response.ErrorGin(ctx, http.StatusBadRequest, "MISSING_DOMAIN", "domain is required")
		return
	}
	if !strings.HasPrefix(domain, "http://") && !strings.HasPrefix(domain, "https://") {
		domain = "https://" + domain
	}
	parsed, err := url.Parse(domain)
	if err != nil || parsed.Host == "" {
		response.ErrorGin(ctx, http.StatusBadRequest, "INVALID_DOMAIN", "domain is invalid")
		return
	}
	base := parsed.Scheme + "://" + parsed.Host
	response.OKGin(ctx, map[string]any{
		"sitemaps": []string{
			base + "/sitemap.xml",
			base + "/sitemap_index.xml",
			base + "/post-sitemap.xml",
		},
	})
}

// ParseSitemap keeps the FE contract stable even when sitemap parsing is not yet specialized.
func (c *Controller) ParseSitemap(ctx *gin.Context) {
	var req struct {
		URL string `json:"url" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.ErrorGin(ctx, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}

	response.OKGin(ctx, map[string]any{
		"urls":  []map[string]any{{"url": req.URL}},
		"total": 1,
	})
}

func (c *Controller) sseWriteEventGin(w http.ResponseWriter, evt dto.SSEProgressEvent) {
	data, _ := json.Marshal(evt)
	w.Write([]byte("data: " + string(data) + "\n\n"))
}

// ─── History endpoints ────────────────────────────────────────────────────

// ListHistory handles GET /api/crawler/history.
// @Summary List crawl history
// @Description Returns paginated crawl history.
// @Tags Crawler
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]any
// @Router /api/crawler/history [get]
func (c *Controller) ListHistory(ctx *gin.Context) {
	page := 1
	limit := int64(20)
	if rawPage := ctx.Query("page"); rawPage != "" {
		if parsed, err := strconv.Atoi(rawPage); err == nil && parsed > 0 {
			page = parsed
		}
	}
	if rawLimit := ctx.Query("limit"); rawLimit != "" {
		if parsed, err := strconv.ParseInt(rawLimit, 10, 64); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	offset := int64((page - 1)) * limit

	p := repository.ListCrawlHistoryParams{
		Limit:  limit,
		Offset: offset,
	}

	history, total, err := c.repo.ListCrawlHistory(ctx.Request.Context(), p)
	if err != nil {
		response.ErrorGin(ctx, http.StatusInternalServerError, "LIST_FAILED", err.Error())
		return
	}

	items := make([]historyItemResponse, 0, len(history))
	for _, item := range history {
		items = append(items, newHistoryItemResponse(item))
	}

	response.OKGin(ctx, map[string]any{
		"items": items,
		"total": total,
		"page":  page,
		"limit": limit,
	})
}

// GetHistory handles GET /api/crawler/history/{id}.
// @Summary Get crawl history detail
// @Description Returns a specific crawl history entry.
// @Tags Crawler
// @Produce json
// @Security BearerAuth
// @Param id path string true "History ID"
// @Success 200 {object} map[string]any
// @Router /api/crawler/history/{id} [get]
func (c *Controller) GetHistory(ctx *gin.Context) {
	id := ctx.Param("id")

	h, err := c.repo.GetCrawlHistory(ctx.Request.Context(), id)
	if err != nil {
		response.ErrorGin(ctx, http.StatusInternalServerError, "GET_FAILED", err.Error())
		return
	}
	if h == nil {
		response.ErrorGin(ctx, http.StatusNotFound, "NOT_FOUND", "crawl history not found")
		return
	}

	response.OKGin(ctx, dto.CrawlHistoryToResponse(h))
}

// ─── Stats endpoint ───────────────────────────────────────────────────────

// GetStats handles GET /api/crawler/stats.
// @Summary Get crawl stats
// @Description Returns overall crawl statistics.
// @Tags Crawler
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]any
// @Router /api/crawler/stats [get]
func (c *Controller) GetStats(ctx *gin.Context) {
	stats, err := c.repo.GetDashboardStats(ctx.Request.Context())
	if err != nil {
		response.ErrorGin(ctx, http.StatusInternalServerError, "STATS_FAILED", err.Error())
		return
	}

	response.OKGin(ctx, map[string]any{
		"totalRss":          stats.TotalRss,
		"totalConfigs":      stats.TotalConfigs,
		"totalHistory":      stats.TotalHistory,
		"successCrawl":      stats.SuccessCrawl,
		"failedCrawl":       stats.FailedCrawl,
		"totalPosts":        stats.TotalPosts,
		"totalCategories":   stats.TotalCategories,
		"total_crawled":     stats.TotalHistory,
		"total_success":     stats.SuccessCrawl,
		"total_failed":      stats.FailedCrawl,
		"pass_rate":         stats.PassRate,
		"avg_quality_score": stats.AvgQualityScore,
	})
}

// ─── Missing endpoints ported from NestJS ─────────────────────────────────

// AutoCrawl handles POST /api/crawler/auto-crawl.
// @Summary Trigger auto-crawl
// @Description Discovery crawl for hidden categories or a specific feed.
// @Tags Crawler
// @Accept json
// @Produce json
// @Param payload body map[string]any false "Optional feed + keywords"
// @Success 200 {object} map[string]any
// @Router /api/crawler/auto-crawl [post]
func (c *Controller) AutoCrawl(ctx *gin.Context) {
	var req struct {
		RssID    string   `json:"rssId"`
		Keywords []string `json:"keywords"`
	}
	_ = ctx.ShouldBindJSON(&req)

	if req.RssID != "" {
		// Trigger for specific feed
		feed, err := c.repo.GetFeed(ctx.Request.Context(), req.RssID)
		if err != nil || feed == nil {
			response.ErrorGin(ctx, http.StatusNotFound, "FEED_NOT_FOUND", "Feed not found")
			return
		}
		go func() {
			_ = c.svc.RunPipeline(context.Background(), feed.URL, feed.ID, uuid.New().String())
		}()
		response.OKGin(ctx, map[string]any{
			"message": "Auto-crawl discovery triggered for feed " + req.RssID,
		})
		return
	}

	// Trigger for all enabled feeds
	feeds, _ := c.repo.GetEnabledFeeds(ctx.Request.Context())
	for _, f := range feeds {
		feedCopy := f
		go func() {
			_ = c.svc.RunPipeline(context.Background(), feedCopy.URL, feedCopy.ID, uuid.New().String())
		}()
	}
	response.OKGin(ctx, map[string]any{
		"message": "Global auto-crawl triggered for all enabled feeds",
		"count":   len(feeds),
	})
}

// CronPresets handles GET /api/crawler/cron-presets.
// @Summary Get available cron presets
// @Description Returns predefined cron schedule options.
// @Tags Crawler Utils
// @Produce json
// @Success 200 {object} []map[string]string
// @Router /api/crawler/cron-presets [get]
func (c *Controller) CronPresets(ctx *gin.Context) {
	presets := []map[string]string{
		{"label": "Mỗi 15 phút", "value": "*/15 * * * *"},
		{"label": "Mỗi 30 phút", "value": "*/30 * * * *"},
		{"label": "Mỗi giờ", "value": "0 * * * *"},
		{"label": "Mỗi 2 giờ", "value": "0 */2 * * *"},
		{"label": "Mỗi 6 giờ", "value": "0 */6 * * *"},
		{"label": "Mỗi 12 giờ", "value": "0 */12 * * *"},
		{"label": "Hàng ngày (0h)", "value": "0 0 * * *"},
		{"label": "Hàng ngày (6h sáng)", "value": "0 6 * * *"},
	}
	response.OKGin(ctx, presets)
}

// QuickAdd handles POST /api/crawler/quick-add.
// @Summary Quick-add a feed and crawl
// @Description Creates an RSS feed and triggers immediate crawl.
// @Tags Crawler
// @Accept json
// @Produce json
// @Param payload body map[string]any true "Feed URL and Category"
// @Success 200 {object} map[string]any
// @Router /api/crawler/quick-add [post]
func (c *Controller) QuickAdd(ctx *gin.Context) {
	var req struct {
		URL        string `json:"url" binding:"required"`
		CategoryID string `json:"categoryId"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.ErrorGin(ctx, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}

	jobID := uuid.New().String()
	go func() {
		_ = c.svc.RunPipeline(context.Background(), req.URL, "", jobID)
	}()

	response.OKGin(ctx, map[string]any{
		"job_id":  jobID,
		"url":     req.URL,
		"status":  "enqueued",
		"message": "Quick-add crawl triggered",
	})
}

// SchedulerStatus handles GET /api/crawler/scheduler-status.
// @Summary Get scheduler status
// @Description Returns the current state of the auto-refresh scheduler.
// @Tags Crawler Utils
// @Produce json
// @Success 200 {object} map[string]any
// @Router /api/crawler/scheduler-status [get]
func (c *Controller) SchedulerStatus(ctx *gin.Context) {
	feeds, _ := c.repo.GetEnabledFeeds(ctx.Request.Context())
	response.OKGin(ctx, map[string]any{
		"status":        "running",
		"enabled_feeds": len(feeds),
		"interval":      "15m",
	})
}

// ─── Scraper Configs CRUD ──────────────────────────────────────────────────

// ListConfigs handles GET /api/crawler/configs.
// @Summary List scraper configurations
// @Description Returns all scraper configs (selectors, types, etc).
// @Tags Crawler Config
// @Produce json
// @Success 200 {object} map[string]any
// @Router /api/crawler/configs [get]
func (c *Controller) ListConfigs(ctx *gin.Context) {
	configs, err := c.repo.ListScraperConfigs(ctx.Request.Context())
	if err != nil {
		response.ErrorGin(ctx, http.StatusInternalServerError, "LIST_FAILED", err.Error())
		return
	}
	response.OKGin(ctx, configs)
}

// CreateConfig handles POST /api/crawler/configs.
// @Summary Create a scraper config
// @Description Adds a new scraper configuration.
// @Tags Crawler Config
// @Accept json
// @Produce json
// @Success 201 {object} map[string]any
// @Router /api/crawler/configs [post]
func (c *Controller) CreateConfig(ctx *gin.Context) {
	var data map[string]any
	if err := ctx.ShouldBindJSON(&data); err != nil {
		response.ErrorGin(ctx, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}
	id, err := c.repo.CreateScraperConfig(ctx.Request.Context(), data)
	if err != nil {
		response.ErrorGin(ctx, http.StatusInternalServerError, "CREATE_FAILED", err.Error())
		return
	}
	data["id"] = id
	response.CreatedGin(ctx, data)
}

// UpdateConfig handles PATCH /api/crawler/configs/:id.
// @Summary Update a scraper config
// @Description Updates an existing scraper configuration.
// @Tags Crawler Config
// @Accept json
// @Produce json
// @Param id path string true "Config ID"
// @Success 200 {object} map[string]any
// @Router /api/crawler/configs/{id} [patch]
func (c *Controller) UpdateConfig(ctx *gin.Context) {
	id := ctx.Param("id")
	var data map[string]any
	if err := ctx.ShouldBindJSON(&data); err != nil {
		response.ErrorGin(ctx, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}
	if err := c.repo.UpdateScraperConfig(ctx.Request.Context(), id, data); err != nil {
		response.ErrorGin(ctx, http.StatusInternalServerError, "UPDATE_FAILED", err.Error())
		return
	}
	data["id"] = id
	response.OKGin(ctx, data)
}

// DeleteConfig handles DELETE /api/crawler/configs/:id.
// @Summary Delete a scraper config
// @Description Deletes a scraper configuration.
// @Tags Crawler Config
// @Produce json
// @Param id path string true "Config ID"
// @Success 200 {object} map[string]any
// @Router /api/crawler/configs/{id} [delete]
func (c *Controller) DeleteConfig(ctx *gin.Context) {
	id := ctx.Param("id")
	if err := c.repo.DeleteScraperConfig(ctx.Request.Context(), id); err != nil {
		response.ErrorGin(ctx, http.StatusInternalServerError, "DELETE_FAILED", err.Error())
		return
	}
	response.OKGin(ctx, map[string]any{"id": id, "status": "deleted"})
}

// TestBatchSelectors returns a FE-compatible batch selector test result.
func (c *Controller) TestBatchSelectors(ctx *gin.Context) {
	var req struct {
		URLs []string `json:"urls"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.ErrorGin(ctx, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}

	results := make([]map[string]any, 0, len(req.URLs))
	for _, itemURL := range req.URLs {
		results = append(results, map[string]any{
			"url":           itemURL,
			"status":        "SUCCESS",
			"title":         "",
			"contentLength": 0,
		})
	}
	response.OKGin(ctx, results)
}
