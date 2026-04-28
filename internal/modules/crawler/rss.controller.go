// Package crawler provides RSS feed CRUD controllers.
package crawler

import (
	"erg.ninja/internal/dto/response"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mmcdole/gofeed"

	"erg.ninja/internal/modules/crawler/dto"
	"erg.ninja/internal/modules/crawler/entities"
	"erg.ninja/internal/modules/crawler/repository"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/queue"
)

const refreshFeedJobType = "crawler:refresh_feed"

// RSSController handles HTTP requests for RSS feed management.
type RSSController struct {
	repo  *repository.Repository
	queue *queue.AsynqClient
	log   *logger.Logger
}

// NewRSSController creates a new RSS controller.
func NewRSSController(repo *repository.Repository, queueClient *queue.AsynqClient, log *logger.Logger) *RSSController {
	return &RSSController{repo: repo, queue: queueClient, log: log}
}

// RegisterRoutes mounts the RSS feed REST API routes.
func (c *RSSController) RegisterRoutes(r *gin.Engine) {
	api := r.Group("/api/rss/feeds")
	api.POST("/", c.CreateFeed)
	api.GET("/", c.ListFeeds)
	api.GET("/peek", c.PeekFeed)
	api.POST("/preview", c.PreviewFeed)
	api.POST("/trigger", c.TriggerFeed)
	api.POST("/create-selective", c.CreateSelective)
	api.GET("/:id", c.GetFeed)
	api.PUT("/:id", c.UpdateFeed)
	api.DELETE("/:id", c.DeleteFeed)
	api.POST("/:id/refresh", c.RefreshFeed)

	// Frontend-compatible legacy route aliases used by the Next.js admin UI.
	legacy := r.Group("/api/crawler/rss")
	legacy.GET("", c.ListFeedsCompat)
	legacy.POST("", c.CreateFeed)
	legacy.PATCH("/:id", c.UpdateFeed)
	legacy.DELETE("/:id", c.DeleteFeed)
	legacy.POST("/preview", c.PreviewFeed)
	legacy.POST("/create-selective", c.CreateSelective)
	legacy.GET("/peek/:id", c.PeekFeedByIDCompat)
	legacy.POST("/trigger", c.TriggerFeed)
	legacy.POST("/sync/:id", c.RefreshFeed)
}

// CreateFeed handles POST /api/rss/feeds.
// @Summary Create a new RSS feed
// @Description Adds a new RSS feed source to the crawler system.
// @Tags Crawler RSS
// @Accept json
// @Produce json
// @Param payload body dto.CreateFeedRequest true "Feed parameters"
// @Success 201 {object} dto.FeedResponse
// @Router /api/rss/feeds [post]
func (c *RSSController) CreateFeed(ctx *gin.Context) {
	var req struct {
		dto.CreateFeedRequest
		TargetCategoryID string `json:"targetCategoryId"`
		CronExpression   string `json:"cronExpression"`
		IsActive         *bool  `json:"isActive"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		c.writeError(ctx, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}

	if req.Name == "" || req.URL == "" {
		c.writeError(ctx, http.StatusBadRequest, "MISSING_FIELDS", "name and url are required")
		return
	}

	feedType := req.CreateFeedRequest.Type
	if feedType == "" || feedType == "auto" {
		feedType = "rss"
	}
	category := req.CreateFeedRequest.Category
	if category == "" {
		category = req.TargetCategoryID
	}
	frequency := req.CreateFeedRequest.Frequency
	if frequency == "" {
		frequency = req.CronExpression
	}
	enabled := true
	if req.IsActive != nil {
		enabled = *req.IsActive
	}

	feed := &entities.RSSFeed{
		Name:      req.CreateFeedRequest.Name,
		URL:       req.CreateFeedRequest.URL,
		Type:      feedType,
		Category:  category,
		Language:  req.CreateFeedRequest.Language,
		Frequency: frequency,
		Enabled:   enabled,
	}

	if err := c.repo.CreateFeed(ctx.Request.Context(), feed); err != nil {
		c.writeError(ctx, http.StatusInternalServerError, "CREATE_FAILED", err.Error())
		return
	}

	c.writeJSON(ctx, http.StatusCreated, dto.FeedToResponse(feed))
}

// ListFeeds handles GET /api/rss/feeds.
// @Summary List all RSS feeds
// @Description Returns a list of configured RSS feeds with optional filtering.
// @Tags Crawler RSS
// @Produce json
// @Param enabled query bool false "Filter by enabled status"
// @Param category query string false "Filter by category"
// @Success 200 {object} map[string]any
// @Router /api/rss/feeds [get]
func (c *RSSController) ListFeeds(ctx *gin.Context) {
	var enabled *bool
	if s := ctx.Query("enabled"); s != "" {
		b := s == "true"
		enabled = &b
	}
	category := ctx.Query("category")

	feeds, err := c.repo.ListFeeds(ctx.Request.Context(), enabled, category)
	if err != nil {
		c.writeError(ctx, http.StatusInternalServerError, "LIST_FAILED", err.Error())
		return
	}

	c.writeJSON(ctx, http.StatusOK, map[string]any{
		"items": dto.FeedsToResponses(feeds),
		"total": len(feeds),
	})
}

// ListFeedsCompat returns the RSS source list in the legacy crawler-admin shape.
func (c *RSSController) ListFeedsCompat(ctx *gin.Context) {
	var enabled *bool
	if s := ctx.Query("enabled"); s != "" {
		b := s == "true"
		enabled = &b
	}
	category := ctx.Query("category")

	feeds, err := c.repo.ListFeeds(ctx.Request.Context(), enabled, category)
	if err != nil {
		c.writeError(ctx, http.StatusInternalServerError, "LIST_FAILED", err.Error())
		return
	}

	items := make([]rssSourceResponse, 0, len(feeds))
	for _, feed := range feeds {
		items = append(items, newRSSSourceResponse(feed))
	}
	c.writeJSON(ctx, http.StatusOK, items)
}

// GetFeed handles GET /api/rss/feeds/:id.
// @Summary Get RSS feed
// @Description Returns a single RSS feed by ID.
// @Tags Crawler RSS
// @Produce json
// @Security BearerAuth
// @Param id path string true "Feed ID"
// @Success 200 {object} map[string]any
// @Router /api/rss/feeds/{id} [get]
func (c *RSSController) GetFeed(ctx *gin.Context) {
	id := ctx.Param("id")

	feed, err := c.repo.GetFeed(ctx.Request.Context(), id)
	if err != nil {
		c.writeError(ctx, http.StatusInternalServerError, "GET_FAILED", err.Error())
		return
	}
	if feed == nil {
		c.writeError(ctx, http.StatusNotFound, "NOT_FOUND", "feed not found")
		return
	}

	c.writeJSON(ctx, http.StatusOK, dto.FeedToResponse(feed))
}

// UpdateFeed handles PUT /api/rss/feeds/:id.
// @Summary Update RSS feed
// @Description Updates an RSS feed configuration.
// @Tags Crawler RSS
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Feed ID"
// @Success 200 {object} map[string]any
// @Router /api/rss/feeds/{id} [put]
func (c *RSSController) UpdateFeed(ctx *gin.Context) {
	id := ctx.Param("id")

	var req struct {
		dto.UpdateFeedRequest
		TargetCategoryID string `json:"targetCategoryId"`
		CronExpression   string `json:"cronExpression"`
		IsActive         *bool  `json:"isActive"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		c.writeError(ctx, http.StatusBadRequest, "INVALID_BODY", err.Error())
		return
	}

	update := make(map[string]interface{})
	if req.UpdateFeedRequest.Name != "" {
		update["name"] = req.UpdateFeedRequest.Name
	}
	category := req.UpdateFeedRequest.Category
	if category == "" {
		category = req.TargetCategoryID
	}
	if category != "" {
		update["category"] = category
	}
	if req.UpdateFeedRequest.Language != "" {
		update["language"] = req.UpdateFeedRequest.Language
	}
	frequency := req.UpdateFeedRequest.Frequency
	if frequency == "" {
		frequency = req.CronExpression
	}
	if frequency != "" {
		update["frequency"] = frequency
	}
	if req.UpdateFeedRequest.Enabled != nil {
		update["enabled"] = *req.UpdateFeedRequest.Enabled
	}
	if req.IsActive != nil {
		update["enabled"] = *req.IsActive
	}

	if len(update) == 0 {
		c.writeError(ctx, http.StatusBadRequest, "NO_UPDATE", "no fields to update")
		return
	}

	if err := c.repo.UpdateFeed(ctx.Request.Context(), id, update); err != nil {
		c.writeError(ctx, http.StatusInternalServerError, "UPDATE_FAILED", err.Error())
		return
	}

	feed, _ := c.repo.GetFeed(ctx.Request.Context(), id)
	if feed == nil {
		c.writeError(ctx, http.StatusNotFound, "NOT_FOUND", "feed not found after update")
		return
	}

	c.writeJSON(ctx, http.StatusOK, dto.FeedToResponse(feed))
}

// DeleteFeed handles DELETE /api/rss/feeds/:id.
// @Summary Delete RSS feed
// @Description Removes an RSS feed.
// @Tags Crawler RSS
// @Produce json
// @Security BearerAuth
// @Param id path string true "Feed ID"
// @Success 200 {object} map[string]any
// @Router /api/rss/feeds/{id} [delete]
func (c *RSSController) DeleteFeed(ctx *gin.Context) {
	id := ctx.Param("id")

	if err := c.repo.DeleteFeed(ctx.Request.Context(), id); err != nil {
		c.writeError(ctx, http.StatusInternalServerError, "DELETE_FAILED", err.Error())
		return
	}

	c.writeJSON(ctx, http.StatusOK, map[string]string{
		"status":  "deleted",
		"feed_id": id,
	})
}

// RefreshFeed handles POST /api/rss/feeds/:id/refresh.
// @Summary Force refresh an RSS feed
// @Description Enqueues a high-priority job to fetch and parse an RSS feed immediately.
// @Tags Crawler RSS
// @Produce json
// @Param id path string true "Feed ID"
// @Param force query bool false "Force refresh ignoring cache"
// @Success 202 {object} map[string]string
// @Router /api/rss/feeds/{id}/refresh [post]
func (c *RSSController) RefreshFeed(ctx *gin.Context) {
	id := ctx.Param("id")

	feed, err := c.repo.GetFeed(ctx.Request.Context(), id)
	if err != nil {
		c.writeError(ctx, http.StatusInternalServerError, "GET_FAILED", err.Error())
		return
	}
	if feed == nil {
		c.writeError(ctx, http.StatusNotFound, "NOT_FOUND", "feed not found")
		return
	}

	if c.queue == nil {
		c.writeError(ctx, http.StatusServiceUnavailable, "QUEUE_UNAVAILABLE", "background queue is not configured")
		return
	}

	payload := entities.RefreshFeedPayload{
		FeedID: id,
		Force:  ctx.Query("force") == "true",
	}
	jobID, err := c.queue.Enqueue(
		ctx.Request.Context(),
		refreshFeedJobType,
		payload,
		queue.WithQueue(queue.PriorityHigh),
	)
	if err != nil {
		c.log.Error().Err(err).Str("feed_id", id).Msg("rss: failed to enqueue refresh job")
		c.writeError(ctx, http.StatusInternalServerError, "ENQUEUE_FAILED", "failed to enqueue refresh job")
		return
	}

	c.writeJSON(ctx, http.StatusAccepted, map[string]string{
		"status":  "refresh_enqueued",
		"feed_id": id,
		"feed":    feed.URL,
		"job_id":  jobID,
	})
}

// PeekFeed handles GET /api/rss/feeds/peek.
// @Summary Peek into an RSS feed URL
// @Description Fetches and parses an RSS feed URL without saving it to the database.
// @Tags Crawler RSS
// @Produce json
// @Param url query string true "Feed URL"
// @Success 200 {object} map[string]any
// @Router /api/rss/feeds/peek [get]
func (c *RSSController) PeekFeed(ctx *gin.Context) {
	url := ctx.Query("url")
	if url == "" {
		c.writeError(ctx, http.StatusBadRequest, "MISSING_URL", "url is required")
		return
	}

	fp := gofeed.NewParser()
	feed, err := fp.ParseURL(url)
	if err != nil {
		c.writeError(ctx, http.StatusInternalServerError, "PARSE_FAILED", err.Error())
		return
	}

	limit := 5
	if len(feed.Items) < limit {
		limit = len(feed.Items)
	}

	var items []map[string]any
	for _, item := range feed.Items[:limit] {
		pubDate := ""
		if item.PublishedParsed != nil {
			pubDate = item.PublishedParsed.Format("2006-01-02T15:04:05Z")
		}
		thumbnail := ""
		if item.Image != nil {
			thumbnail = item.Image.URL
		}
		items = append(items, map[string]any{
			"title":       item.Title,
			"link":        item.Link,
			"description": item.Description,
			"pubDate":     pubDate,
			"thumbnail":   thumbnail,
		})
	}

	c.writeJSON(ctx, http.StatusOK, map[string]any{
		"title":       feed.Title,
		"description": feed.Description,
		"link":        feed.Link,
		"items":       items,
	})
}

// ─── Additional RSS endpoints ported from NestJS ─────────────────────────

// TriggerFeed handles POST /api/rss/feeds/trigger.
// @Summary Trigger RSS sync
// @Description Triggers an immediate sync for a given feed by ID in body.
// @Tags Crawler RSS
// @Accept json
// @Produce json
// @Param payload body map[string]any true "Feed ID"
// @Success 202 {object} map[string]string
// @Router /api/rss/feeds/trigger [post]
func (c *RSSController) TriggerFeed(ctx *gin.Context) {
	var req struct {
		RssID string `json:"rssId"`
		ID    string `json:"id"`
	}
	_ = ctx.ShouldBindJSON(&req)

	targetID := req.RssID
	if targetID == "" {
		targetID = req.ID
	}
	if targetID == "" {
		c.writeError(ctx, 400, "MISSING_ID", "rssId or id is required")
		return
	}

	feed, err := c.repo.GetFeed(ctx.Request.Context(), targetID)
	if err != nil || feed == nil {
		c.writeError(ctx, 404, "NOT_FOUND", "feed not found")
		return
	}

	if c.queue != nil {
		_, _ = c.queue.Enqueue(ctx.Request.Context(), refreshFeedJobType, map[string]any{"feed_id": targetID, "force": true})
	}

	c.writeJSON(ctx, 202, map[string]string{"status": "triggered", "feed_id": targetID})
}

// PreviewFeed handles POST /api/rss/feeds/preview.
// @Summary Preview a custom RSS URL
// @Description Fetches and parses a custom RSS URL for preview.
// @Tags Crawler RSS
// @Accept json
// @Produce json
// @Param payload body map[string]any true "RSS URL"
// @Success 200 {object} map[string]any
// @Router /api/rss/feeds/preview [post]
func (c *RSSController) PreviewFeed(ctx *gin.Context) {
	var req struct {
		URL string `json:"url" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		c.writeError(ctx, 400, "INVALID_BODY", err.Error())
		return
	}

	fp := gofeed.NewParser()
	feed, err := fp.ParseURL(req.URL)
	if err != nil {
		c.writeError(ctx, 500, "PARSE_FAILED", err.Error())
		return
	}

	limit := 10
	if len(feed.Items) < limit {
		limit = len(feed.Items)
	}

	var items []map[string]any
	for _, item := range feed.Items[:limit] {
		pubDate := ""
		if item.PublishedParsed != nil {
			pubDate = item.PublishedParsed.Format("2006-01-02T15:04:05Z")
		}
		thumbnail := ""
		if item.Image != nil {
			thumbnail = item.Image.URL
		}
		items = append(items, map[string]any{
			"title":       item.Title,
			"link":        item.Link,
			"description": item.Description,
			"pubDate":     pubDate,
			"thumbnail":   thumbnail,
		})
	}

	c.writeJSON(ctx, 200, map[string]any{
		"title":       feed.Title,
		"description": feed.Description,
		"link":        feed.Link,
		"items":       items,
		"total":       len(feed.Items),
	})
}

// CreateSelective handles POST /api/rss/feeds/create-selective.
// @Summary Create RSS feed with selected items
// @Description Creates a feed and immediately crawls only the selected links.
// @Tags Crawler RSS
// @Accept json
// @Produce json
// @Success 201 {object} map[string]any
// @Router /api/rss/feeds/create-selective [post]
func (c *RSSController) CreateSelective(ctx *gin.Context) {
	var req struct {
		Feed          map[string]any `json:"feed"`
		SelectedLinks []string       `json:"selectedLinks"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		c.writeError(ctx, 400, "INVALID_BODY", err.Error())
		return
	}

	// Create the feed
	name, _ := req.Feed["name"].(string)
	url, _ := req.Feed["url"].(string)
	if name == "" || url == "" {
		c.writeError(ctx, 400, "MISSING_FIELDS", "feed.name and feed.url are required")
		return
	}

	feed := &entities.RSSFeed{
		Name:    name,
		URL:     url,
		Type:    "rss",
		Enabled: true,
	}
	if category, _ := req.Feed["targetCategoryId"].(string); category != "" {
		feed.Category = category
	}
	if isActive, ok := req.Feed["isActive"].(bool); ok {
		feed.Enabled = isActive
	}
	if err := c.repo.CreateFeed(ctx.Request.Context(), feed); err != nil {
		c.writeError(ctx, 500, "CREATE_FAILED", err.Error())
		return
	}

	c.writeJSON(ctx, 201, map[string]any{
		"feed":           feed,
		"selected_links": req.SelectedLinks,
		"status":         "created",
	})
}

// PeekFeedByIDCompat returns feed items in the legacy `/api/crawler/rss/peek/:id` shape.
func (c *RSSController) PeekFeedByIDCompat(ctx *gin.Context) {
	id := ctx.Param("id")
	feed, err := c.repo.GetFeed(ctx.Request.Context(), id)
	if err != nil {
		c.writeError(ctx, http.StatusInternalServerError, "GET_FAILED", err.Error())
		return
	}
	if feed == nil {
		c.writeError(ctx, http.StatusNotFound, "NOT_FOUND", "feed not found")
		return
	}

	fp := gofeed.NewParser()
	parsed, err := fp.ParseURL(feed.URL)
	if err != nil {
		c.writeError(ctx, http.StatusInternalServerError, "PARSE_FAILED", err.Error())
		return
	}

	limit := 10
	if len(parsed.Items) < limit {
		limit = len(parsed.Items)
	}

	items := make([]map[string]any, 0, limit)
	for _, item := range parsed.Items[:limit] {
		pubDate := ""
		if item.PublishedParsed != nil {
			pubDate = item.PublishedParsed.Format(time.RFC3339)
		}
		isCrawled, _ := c.repo.URLExists(ctx.Request.Context(), item.Link)
		thumbnail := ""
		if item.Image != nil {
			thumbnail = item.Image.URL
		}
		items = append(items, map[string]any{
			"title":     item.Title,
			"link":      item.Link,
			"thumbnail": firstNonEmpty(thumbnail),
			"pubDate":   pubDate,
			"isCrawled": isCrawled,
		})
	}

	c.writeJSON(ctx, http.StatusOK, map[string]any{
		"feedName": parsed.Title,
		"feedUrl":  feed.URL,
		"items":    items,
	})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// ─── Helpers ─────────────────────────────────────────────────────────────

func (c *RSSController) writeJSON(ctx *gin.Context, status int, v any) {
	response.WriteGin(ctx, status, v, nil, nil)
}

func (c *RSSController) writeError(ctx *gin.Context, status int, code, message string) {
	response.ErrorGin(ctx, status, code, message)
}
