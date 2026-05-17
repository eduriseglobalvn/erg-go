// Package handlers
//
// @title ERG Bot Service API
// @version 1.0
// @description REST API for the ERG Bot Service — conversation management, account linking, and workflow execution.
// @host localhost:8080
// @BasePath /
// @securityDefinitions.apikey Bearer
// @in header
// @name Authorization
// @description JWT Bearer token (Bearer <token>)
package handlers

import (
	"erg.ninja/internal/dto/response"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"erg.ninja/internal/modules/bot/application/service"
	"erg.ninja/internal/modules/bot/domain/entity"
	"erg.ninja/internal/modules/bot/infrastructure/platform"
	"erg.ninja/pkg/logger"
)

// BotController handles REST API endpoints for the bot-service.
type BotController struct {
	convSvc     *services.ConversationService
	linkSvc     *services.LinkService
	workflowSvc *services.WorkflowEngine
	discord     *platform.DiscordClient
	telegram    *platform.TelegramClient
	coll        *mongo.Collection
	log         *logger.Logger
}

// NewBotController creates a BotController.
func NewBotController(
	convSvc *services.ConversationService,
	linkSvc *services.LinkService,
	workflowSvc *services.WorkflowEngine,
	discord *platform.DiscordClient,
	telegram *platform.TelegramClient,
	coll *mongo.Collection,
	log *logger.Logger,
) *BotController {
	return &BotController{
		convSvc:     convSvc,
		linkSvc:     linkSvc,
		workflowSvc: workflowSvc,
		discord:     discord,
		telegram:    telegram,
		coll:        coll,
		log:         log,
	}
}

// RegisterRoutes mounts all REST API routes onto the given router group.
func (c *BotController) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/conversations", c.ListConversations)
	r.GET("/conversations/:id", c.GetConversation)
	r.POST("/conversations/:id/send", c.SendToConversation)
	r.GET("/conversations/:id/wizard", c.GetWizardState)

	r.POST("/link", c.CreateLinkCode)
	r.GET("/link/:code", c.VerifyLinkCode)
	r.GET("/accounts", c.ListLinkedAccounts)
	r.DELETE("/accounts/:id", c.UnlinkAccount)
}

// --- Conversation endpoints ---

// ListConversations handles GET /api/bot/conversations.
// @Summary List conversations
// @Description Returns paginated bot conversations.
// @Tags Bot
// @Produce json
// @Security BearerAuth
// @Param platform query string false "Platform filter"
// @Param limit query int false "Limit"
// @Param skip query int false "Skip"
// @Success 200 {object} map[string]any
// @Router /api/bot/conversations [get]
func (c *BotController) ListConversations(ctx *gin.Context) {
	filter := bson.M{"state": bson.M{"$in": []any{"active", "pending"}}}
	if platform := ctx.Query("platform"); platform != "" {
		filter["platform"] = platform
	}
	if userID := ctx.Query("user_id"); userID != "" {
		filter["user_id"] = userID
	}

	limit := int64(20)
	if l := ctx.Query("limit"); l != "" {
		if v, err := strconv.ParseInt(l, 10, 64); err == nil && v > 0 && v <= 100 {
			limit = v
		}
	}

	skip := int64(0)
	if s := ctx.Query("skip"); s != "" {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil && v >= 0 {
			skip = v
		}
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "updated_at", Value: -1}}).
		SetLimit(limit).
		SetSkip(skip)

	cursor, err := c.coll.Find(ctx.Request.Context(), filter, opts)
	if err != nil {
		c.writeError(ctx, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	defer cursor.Close(ctx.Request.Context())

	var convs []models.BotConversation
	if err := cursor.All(ctx.Request.Context(), &convs); err != nil {
		c.writeError(ctx, http.StatusInternalServerError, "DECODE_ERROR", err.Error())
		return
	}

	c.writeJSON(ctx, http.StatusOK, map[string]any{
		"conversations": convs,
		"count":         len(convs),
		"limit":         limit,
		"skip":          skip,
	})
}

// GetConversation handles GET /api/bot/conversations/{id}.
// @Summary Get conversation
// @Description Returns a single conversation by ID.
// @Tags Bot
// @Produce json
// @Security BearerAuth
// @Param id path string true "Conversation ID"
// @Success 200 {object} map[string]any
// @Router /api/bot/conversations/{id} [get]
func (c *BotController) GetConversation(ctx *gin.Context) {
	convID := ctx.Param("id")

	var conv models.BotConversation
	err := c.coll.FindOne(ctx.Request.Context(), bson.M{"platform_conv_id": convID}).Decode(&conv)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.writeError(ctx, http.StatusNotFound, "NOT_FOUND", "conversation not found")
			return
		}
		c.writeError(ctx, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	c.writeJSON(ctx, http.StatusOK, conv)
}

// SendToConversation handles POST /api/bot/conversations/{id}/send.
// @Summary Send message to conversation
// @Description Sends a message to a conversation via platform.
// @Tags Bot
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Conversation ID"
// @Success 200 {object} map[string]any
// @Router /api/bot/conversations/{id}/send [post]
func (c *BotController) SendToConversation(ctx *gin.Context) {
	convID := ctx.Param("id")

	var req SendMessageRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		c.writeError(ctx, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	if req.Message == "" {
		c.writeError(ctx, http.StatusBadRequest, "EMPTY_MESSAGE", "message is required")
		return
	}

	// Look up conversation.
	var conv models.BotConversation
	err := c.coll.FindOne(ctx.Request.Context(), bson.M{"platform_conv_id": convID}).Decode(&conv)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.writeError(ctx, http.StatusNotFound, "NOT_FOUND", "conversation not found")
			return
		}
		c.writeError(ctx, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	switch conv.Platform {
	case "discord":
		if c.discord == nil {
			c.writeError(ctx, http.StatusServiceUnavailable, "DISCORD_UNAVAILABLE", "discord client is not configured")
			return
		}
		if err := c.discord.SendChannelMessage(ctx.Request.Context(), conv.PlatformConvID, req.Message); err != nil {
			if conv.UserID != "" {
				if dmErr := c.discord.SendDM(ctx.Request.Context(), conv.UserID, req.Message); dmErr == nil {
					break
				} else {
					c.log.WarnContext(ctx.Request.Context()).Err(dmErr).Str("conversation_id", convID).Msg("bot: discord DM fallback failed")
				}
			}
			c.writeError(ctx, http.StatusBadGateway, "SEND_FAILED", err.Error())
			return
		}
	case "telegram":
		if c.telegram == nil {
			c.writeError(ctx, http.StatusServiceUnavailable, "TELEGRAM_UNAVAILABLE", "telegram client is not configured")
			return
		}
		chatID, err := strconv.ParseInt(conv.PlatformConvID, 10, 64)
		if err != nil {
			c.writeError(ctx, http.StatusInternalServerError, "INVALID_CONVERSATION_ID", fmt.Sprintf("invalid telegram chat id: %v", err))
			return
		}
		if _, err := c.telegram.SendMessage(ctx.Request.Context(), chatID, req.Message); err != nil {
			c.writeError(ctx, http.StatusBadGateway, "SEND_FAILED", err.Error())
			return
		}
	default:
		c.writeError(ctx, http.StatusBadRequest, "UNSUPPORTED_PLATFORM", "conversation platform is not supported")
		return
	}

	c.writeJSON(ctx, http.StatusOK, map[string]string{
		"status":  "sent",
		"conv_id": convID,
	})
}

// GetWizardState handles GET /api/bot/conversations/{id}/wizard.
// @Summary Get wizard state
// @Description Returns the active wizard state for a conversation.
// @Tags Bot
// @Produce json
// @Security BearerAuth
// @Param id path string true "Conversation ID"
// @Success 200 {object} map[string]any
// @Router /api/bot/conversations/{id}/wizard [get]
func (c *BotController) GetWizardState(ctx *gin.Context) {
	convID := ctx.Param("id")

	wiz, active := c.convSvc.GetActiveWizard(ctx.Request.Context(), convID)
	if !active {
		c.writeJSON(ctx, http.StatusOK, map[string]any{
			"active": false,
		})
		return
	}

	c.writeJSON(ctx, http.StatusOK, map[string]any{
		"active":     true,
		"step":       wiz.Step,
		"data":       wiz.Data,
		"expires_at": wiz.ExpiresAt,
		"started_at": wiz.StartedAt,
	})
}

// SendMessageRequest is the request body for SendToConversation.
type SendMessageRequest struct {
	Message string `json:"message"`
}

// --- Link endpoints ---

// CreateLinkCode handles POST /api/bot/link.
// @Summary Create link code
// @Description Generates a link code for account linking.
// @Tags Bot
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 201 {object} map[string]any
// @Router /api/bot/link [post]
func (c *BotController) CreateLinkCode(ctx *gin.Context) {
	var req LinkCodeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		c.writeError(ctx, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}

	if req.UserID == "" {
		c.writeError(ctx, http.StatusBadRequest, "MISSING_USER_ID", "user_id is required")
		return
	}

	code, err := c.linkSvc.CreateLinkCode(ctx.Request.Context(), req.UserID)
	if err != nil {
		c.writeError(ctx, http.StatusInternalServerError, "CREATE_CODE_ERROR", err.Error())
		return
	}

	c.writeJSON(ctx, http.StatusCreated, map[string]string{
		"code":       code,
		"expires_in": "5 minutes",
		"user_id":    req.UserID,
	})
}

// VerifyLinkCode handles GET /api/bot/link/{code}.
// @Summary Verify link code
// @Description Verifies a link code for account linking.
// @Tags Bot
// @Produce json
// @Param code path string true "Link code"
// @Success 200 {object} map[string]any
// @Router /api/bot/link/{code} [get]
func (c *BotController) VerifyLinkCode(ctx *gin.Context) {
	code := ctx.Param("code")

	if code == "" {
		c.writeError(ctx, http.StatusBadRequest, "MISSING_CODE", "code is required")
		return
	}

	// Look up the code in Redis to get the user ID.
	// Verification is done via webhook from the platform client.
	c.writeJSON(ctx, http.StatusOK, map[string]string{
		"code":   code,
		"status": "pending",
	})
}

// ListLinkedAccounts handles GET /api/bot/accounts.
// @Summary List linked accounts
// @Description Returns linked platform accounts for a user.
// @Tags Bot
// @Produce json
// @Security BearerAuth
// @Param user_id query string true "User ID"
// @Success 200 {object} map[string]any
// @Router /api/bot/accounts [get]
func (c *BotController) ListLinkedAccounts(ctx *gin.Context) {
	userID := ctx.Query("user_id")

	if userID == "" {
		c.writeError(ctx, http.StatusBadRequest, "MISSING_USER_ID", "user_id query param is required")
		return
	}

	accounts, err := c.linkSvc.GetLinkedAccounts(ctx.Request.Context(), userID)
	if err != nil {
		c.writeError(ctx, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	c.writeJSON(ctx, http.StatusOK, map[string]any{
		"accounts": accounts,
		"count":    len(accounts),
	})
}

// UnlinkAccount handles DELETE /api/bot/accounts/{id}.
// @Summary Unlink account
// @Description Removes a linked platform account.
// @Tags Bot
// @Produce json
// @Security BearerAuth
// @Param id path string true "Platform User ID"
// @Param platform query string true "Platform"
// @Success 200 {object} map[string]any
// @Router /api/bot/accounts/{id} [delete]
func (c *BotController) UnlinkAccount(ctx *gin.Context) {
	platform := ctx.Query("platform")
	platformUserID := ctx.Param("id")

	if platform == "" || platformUserID == "" {
		c.writeError(ctx, http.StatusBadRequest, "MISSING_PARAMS", "platform and platform_user_id are required")
		return
	}

	if err := c.linkSvc.UnlinkAccount(ctx.Request.Context(), platform, platformUserID); err != nil {
		c.writeError(ctx, http.StatusInternalServerError, "UNLINK_ERROR", err.Error())
		return
	}

	c.writeJSON(ctx, http.StatusOK, map[string]string{
		"status": "unlinked",
	})
}

// LinkCodeRequest is the request body for CreateLinkCode.
type LinkCodeRequest struct {
	UserID string `json:"user_id"`
}

// writeJSON writes a JSON response with consistent formatting.
func (c *BotController) writeJSON(ctx *gin.Context, status int, v any) {
	response.WriteGin(ctx, status, v, nil, nil)
}

// writeError writes a structured API error response.
func (c *BotController) writeError(ctx *gin.Context, status int, code, message string) {
	response.ErrorGin(ctx, status, code, message)
}
