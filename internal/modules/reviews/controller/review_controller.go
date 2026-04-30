// Package controller handles HTTP requests for the reviews module.
package controller

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/dto/response"
	"erg.ninja/internal/middleware"
	"erg.ninja/internal/modules/reviews/dto"
	"erg.ninja/internal/modules/reviews/service"
	"erg.ninja/pkg/auth"
	"erg.ninja/pkg/logger"
)

// Controller handles HTTP requests for reviews.
type Controller struct {
	svc  *service.Service
	log  *logger.Logger
	auth *auth.JWTValidator
}

// NewController creates a new reviews controller.
func NewController(svc *service.Service, log *logger.Logger, jwtValidator *auth.JWTValidator) *Controller {
	return &Controller{
		svc:  svc,
		log:  log,
		auth: jwtValidator,
	}
}

// RegisterRoutes mounts the reviews REST API routes.
func (c *Controller) RegisterRoutes(r *gin.Engine) {
	api := r.Group("/api/reviews")
	// Public routes — no auth required.
	api.GET("/", c.ListReviews)
	api.GET("/stats", c.GetStats)
	api.POST("/", c.CreateReview)
	api.POST("/:reviewId/helpful", c.MarkHelpful)

	// Admin routes — JWT auth required.
	admin := api.Group("")
	admin.Use(middleware.JWTMiddleware(c.auth), middleware.RequireRoles("admin"))
	admin.GET("/admin", c.ListAdminReviews)
	admin.GET("/admin/all", c.ListAdminReviews)
	admin.PATCH("/:reviewId/status", c.UpdateStatus)
	admin.POST("/batch/status", c.BulkUpdateStatus)
	admin.PATCH("/:reviewId/approve", c.ApproveReview)
	admin.PATCH("/:reviewId/reject", c.RejectReview)
	admin.POST("/:reviewId/reply", c.ReplyReview)
	admin.PATCH("/:reviewId/feature", c.FeatureReview)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// adminUserID extracts the admin user ID from the JWT claims.
func (c *Controller) adminUserID(ctx *gin.Context) string {
	claims := middleware.GetClaims(ctx.Request.Context())
	if claims == nil {
		return ""
	}
	if claims.UserID != "" {
		return claims.UserID
	}
	return claims.Subject
}

// parseQueryInt parses a query param as int, returns default if absent or invalid.
func parseQueryInt(ctx *gin.Context, key string, deflt int) int {
	s := ctx.Query(key)
	if s == "" {
		return deflt
	}
	i, err := strconv.Atoi(s)
	if err != nil || i < 1 {
		return deflt
	}
	return i
}

// isNotFound reports whether err is a repository not-found sentinel.
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "not found")
}

// ─── Public Handlers ──────────────────────────────────────────────────────────

// ListReviews handles GET /api/reviews.
// @Summary List reviews
// @Description Fetch reviews filtered by target ID and type (e.g. course, post).
// @Tags Reviews
// @Accept json
// @Produce json
// @Param targetId query string false "Filter by target ID"
// @Param targetType query string false "Filter by target type"
// @Param page query int false "Page number (default 1)"
// @Param limit query int false "Limit per page (default 10)"
// @Param sort query string false "Sort order (newest|helpful)"
// @Success 200 {object} dto.ReviewListResponse
// @Router /api/reviews [get]
func (c *Controller) ListReviews(ctx *gin.Context) {
	params := dto.ReviewQueryParams{
		TargetID:   ctx.Query("targetId"),
		TargetType: ctx.Query("targetType"),
		Page:       parseQueryInt(ctx, "page", 1),
		Limit:      parseQueryInt(ctx, "limit", 10),
		Sort:       ctx.Query("sort"),
	}
	if params.Sort == "" {
		params.Sort = "newest"
	}

	result, err := c.svc.ListReviews(ctx.Request.Context(), params)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("reviews: ListReviews failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, result)
}

// CreateReview handles POST /api/reviews.
// @Summary Submit a review
// @Description Submit a new review for a target.
// @Tags Reviews
// @Accept json
// @Produce json
// @Param payload body dto.CreateReviewRequest true "Review Data"
// @Success 201 {object} dto.ReviewResponse
// @Failure 400 {object} response.Response
// @Router /api/reviews [post]
func (c *Controller) CreateReview(ctx *gin.Context) {
	var req dto.CreateReviewRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	if err := dto.Validate(&req); err != nil {
		response.ValidationErrorGin(ctx, err)
		return
	}

	// Optionally capture IP and User-Agent for spam/fraud detection.
	ip := getClientIP(ctx)
	if ip != "" {
		_ = ip
	}

	result, err := c.svc.CreateReview(ctx.Request.Context(), &req)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("reviews: CreateReview failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	response.CreatedGin(ctx, result)
}

// MarkHelpful handles POST /api/reviews/{reviewId}/helpful.
// @Summary Mark review as helpful
// @Description Increment the helpful counter for a review.
// @Tags Reviews
// @Accept json
// @Produce json
// @Param reviewId path string true "Review ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} response.Response
// @Router /api/reviews/{reviewId}/helpful [post]
func (c *Controller) MarkHelpful(ctx *gin.Context) {
	reviewID := ctx.Param("reviewId")
	if reviewID == "" {
		response.BadRequestGin(ctx, fmt.Errorf("reviewId is required"))
		return
	}

	if err := c.svc.MarkHelpful(ctx.Request.Context(), reviewID); err != nil {
		if isNotFound(err) {
			response.NotFoundGin(ctx, "review not found")
			return
		}
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Str("review_id", reviewID).Msg("reviews: MarkHelpful failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, map[string]string{"message": "marked as helpful"})
}

// GetStats handles GET /api/reviews/stats.
func (c *Controller) GetStats(ctx *gin.Context) {
	stats, err := c.svc.GetStats(ctx.Request.Context(), ctx.Query("targetId"), ctx.Query("targetType"))
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("reviews: GetStats failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, stats)
}

// ─── Admin Handlers ──────────────────────────────────────────────────────────

// ListAdminReviews handles GET /api/reviews/admin/all.
// @Summary Admin: List all reviews
// @Description Fetch all reviews for management.
// @Tags Reviews Admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param status query string false "Filter by status"
// @Success 200 {object} dto.AdminReviewListResponse
// @Router /api/reviews/admin/all [get]
func (c *Controller) ListAdminReviews(ctx *gin.Context) {
	params := dto.AdminReviewQueryParams{
		Status:     ctx.Query("status"),
		TargetType: ctx.Query("targetType"),
		TargetID:   ctx.Query("targetId"),
		Page:       parseQueryInt(ctx, "page", 1),
		Limit:      parseQueryInt(ctx, "limit", 10),
		Sort:       ctx.Query("sort"),
	}
	if params.Sort == "" {
		params.Sort = "newest"
	}

	result, err := c.svc.ListAdminReviews(ctx.Request.Context(), params)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("reviews: ListAdminReviews failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, result)
}

// ApproveReview handles PATCH /api/reviews/{reviewId}/approve.
// @Summary Admin: Approve review
// @Description Mark a review as approved/published.
// @Tags Reviews Admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param reviewId path string true "Review ID"
// @Success 200 {object} dto.ReviewResponse
// @Router /api/reviews/{reviewId}/approve [patch]
func (c *Controller) ApproveReview(ctx *gin.Context) {
	reviewID := ctx.Param("reviewId")
	if reviewID == "" {
		response.BadRequestGin(ctx, fmt.Errorf("reviewId is required"))
		return
	}

	adminID := c.adminUserID(ctx)
	result, err := c.svc.ApproveReview(ctx.Request.Context(), reviewID, adminID)
	if err != nil {
		if isNotFound(err) {
			response.NotFoundGin(ctx, "review not found")
			return
		}
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Str("review_id", reviewID).Msg("reviews: ApproveReview failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, result)
}

// RejectReview handles PATCH /api/reviews/{reviewId}/reject.
// @Summary Admin: Reject review
// @Description Mark a review as rejected.
// @Tags Reviews Admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param reviewId path string true "Review ID"
// @Success 200 {object} map[string]any
// @Router /api/reviews/{reviewId}/reject [patch]
func (c *Controller) RejectReview(ctx *gin.Context) {
	reviewID := ctx.Param("reviewId")
	if reviewID == "" {
		response.BadRequestGin(ctx, fmt.Errorf("reviewId is required"))
		return
	}

	var req dto.RejectReviewRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	if err := dto.Validate(&req); err != nil {
		response.ValidationErrorGin(ctx, err)
		return
	}

	adminID := c.adminUserID(ctx)
	result, err := c.svc.RejectReview(ctx.Request.Context(), reviewID, adminID, &req)
	if err != nil {
		if isNotFound(err) {
			response.NotFoundGin(ctx, "review not found")
			return
		}
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Str("review_id", reviewID).Msg("reviews: RejectReview failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, result)
}

// UpdateStatus handles PATCH /api/reviews/{reviewId}/status.
func (c *Controller) UpdateStatus(ctx *gin.Context) {
	reviewID := ctx.Param("reviewId")
	if reviewID == "" {
		response.BadRequestGin(ctx, fmt.Errorf("reviewId is required"))
		return
	}

	var req dto.UpdateReviewStatusRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	if err := dto.Validate(&req); err != nil {
		response.ValidationErrorGin(ctx, err)
		return
	}

	result, err := c.svc.UpdateStatus(ctx.Request.Context(), reviewID, c.adminUserID(ctx), req.Status, req.AdminNote)
	if err != nil {
		if isNotFound(err) {
			response.NotFoundGin(ctx, "review not found")
			return
		}
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Str("review_id", reviewID).Msg("reviews: UpdateStatus failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, result)
}

// BulkUpdateStatus handles POST /api/reviews/batch/status.
func (c *Controller) BulkUpdateStatus(ctx *gin.Context) {
	var req dto.BulkUpdateStatusRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	if err := dto.Validate(&req); err != nil {
		response.ValidationErrorGin(ctx, err)
		return
	}

	result, err := c.svc.BulkUpdateStatus(ctx.Request.Context(), req.IDs, c.adminUserID(ctx), req.Status)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("reviews: BulkUpdateStatus failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, result)
}

// ReplyReview handles POST /api/reviews/{reviewId}/reply.
// @Summary Admin: Reply to review
// @Description Post an admin reply to a review.
// @Tags Reviews Admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param reviewId path string true "Review ID"
// @Success 200 {object} map[string]any
// @Router /api/reviews/{reviewId}/reply [post]
func (c *Controller) ReplyReview(ctx *gin.Context) {
	reviewID := ctx.Param("reviewId")
	if reviewID == "" {
		response.BadRequestGin(ctx, fmt.Errorf("reviewId is required"))
		return
	}

	var req dto.ReplyReviewRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	if req.Reply == "" {
		req.Reply = req.ReplyContent
	}
	if strings.TrimSpace(req.Reply) == "" {
		response.BadRequestGin(ctx, fmt.Errorf("reply or replyContent is required"))
		return
	}
	if err := dto.Validate(&req); err != nil {
		response.ValidationErrorGin(ctx, err)
		return
	}

	adminID := c.adminUserID(ctx)
	result, err := c.svc.ReplyReview(ctx.Request.Context(), reviewID, adminID, &req)
	if err != nil {
		if isNotFound(err) {
			response.NotFoundGin(ctx, "review not found")
			return
		}
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Str("review_id", reviewID).Msg("reviews: ReplyReview failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, result)
}

// FeatureReview handles PATCH /api/reviews/{reviewId}/feature.
// @Summary Admin: Toggle featured review
// @Description Toggle the featured status of a review.
// @Tags Reviews Admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param reviewId path string true "Review ID"
// @Success 200 {object} map[string]any
// @Router /api/reviews/{reviewId}/feature [patch]
func (c *Controller) FeatureReview(ctx *gin.Context) {
	reviewID := ctx.Param("reviewId")
	if reviewID == "" {
		response.BadRequestGin(ctx, fmt.Errorf("reviewId is required"))
		return
	}

	var req dto.FeatureReviewRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}

	result, err := c.svc.ToggleFeatured(ctx.Request.Context(), reviewID, req.IsFeatured)
	if err != nil {
		if isNotFound(err) {
			response.NotFoundGin(ctx, "review not found")
			return
		}
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Str("review_id", reviewID).Msg("reviews: FeatureReview failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, result)
}

// getClientIP extracts the real client IP, handling X-Forwarded-For.
func getClientIP(ctx *gin.Context) string {
	if fwd := ctx.GetHeader("X-Forwarded-For"); fwd != "" {
		parts := strings.SplitN(fwd, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	return ctx.ClientIP()
}
