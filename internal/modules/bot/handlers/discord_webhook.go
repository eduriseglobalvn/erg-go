// Package handlers implements HTTP handlers for the bot-service.
package handlers

import (
	"context"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/modules/bot/models"
	"erg.ninja/internal/modules/bot/platform"
	"erg.ninja/internal/modules/bot/services"
	"erg.ninja/pkg/logger"
)

// DiscordWebhookHandler handles incoming Discord interaction webhooks.
type DiscordWebhookHandler struct {
	cmdHandler *services.CommandHandler
	linkSvc    *services.LinkService
	convSvc    *services.ConversationService
	discord    *platform.DiscordClient
	secret     string
	log        *logger.Logger
}

// NewDiscordWebhookHandler creates a DiscordWebhookHandler.
func NewDiscordWebhookHandler(
	cmdHandler *services.CommandHandler,
	linkSvc *services.LinkService,
	convSvc *services.ConversationService,
	discord *platform.DiscordClient,
	secret string,
	log *logger.Logger,
) *DiscordWebhookHandler {
	return &DiscordWebhookHandler{
		cmdHandler: cmdHandler,
		linkSvc:    linkSvc,
		convSvc:    convSvc,
		discord:    discord,
		secret:     secret,
		log:        log,
	}
}

// RegisterRoutes mounts the Discord webhook routes onto the given router.
func (h *DiscordWebhookHandler) RegisterRoutes(r *gin.Engine) {
	r.POST("/webhooks/discord", h.HandleDiscordWebhook)
}

// HandleDiscordWebhook handles POST /webhooks/discord
// Discord sends interaction webhooks that need to be acknowledged within 3 seconds.
// Signature verification: X-Signature-Ed25519 (Ed25519) or X-Hub-Signature-256 (HMAC).
// @Summary Handle Discord webhook
// @Description Receives and processes Discord interaction webhooks.
// @Tags Bot Webhooks
// @Accept json
// @Produce json
// @Success 200 {object} map[string]any
// @Router /webhooks/discord [post]
func (h *DiscordWebhookHandler) HandleDiscordWebhook(ctx *gin.Context) {
	// Read body.
	body, err := io.ReadAll(io.LimitReader(ctx.Request.Body, 1<<20)) // 1 MB max
	if err != nil {
		h.log.Error().Err(err).Msg("discord: read body")
		ctx.String(http.StatusBadRequest, "bad request")
		return
	}

	// Verify signature.
	if !h.verifyDiscordRequest(ctx.Request, body) {
		h.log.Warn().Msg("discord: invalid signature")
		ctx.String(http.StatusUnauthorized, "unauthorized")
		return
	}

	// Parse interaction payload.
	var interaction DiscordInteraction
	if err := json.Unmarshal(body, &interaction); err != nil {
		h.log.Error().Err(err).Msg("discord: unmarshal interaction")
		ctx.String(http.StatusBadRequest, "bad request")
		return
	}

	// Handle different interaction types.
	switch interaction.Type {
	case DiscordInteractionTypePing:
		// Respond with Pong (acknowledgement).
		ctx.JSON(http.StatusOK, DiscordPingResponse{Type: DiscordInteractionTypePing})

	case DiscordInteractionTypeApplicationCommand,
		DiscordInteractionTypeMessageComponent,
		DiscordInteractionTypeApplicationCommandAutocomplete:
		// Respond immediately with acknowledgement, then process asynchronously.
		// Create a fresh context with timeout so the goroutine doesn't outlive the request.
		asyncCtx, asyncCancel := context.WithTimeout(context.Background(), 30*time.Second)
		go func() {
			defer asyncCancel()
			h.processInteraction(asyncCtx, interaction, string(body))
		}()

		// Respond with ACK to Discord (within 3 seconds).
		ctx.JSON(http.StatusOK, DiscordPingResponse{Type: DiscordInteractionTypeDeferredChannelMessage})

	default:
		h.log.Warn().Int("type", interaction.Type).Msg("discord: unknown interaction type")
	}
}

// processInteraction handles the Discord interaction asynchronously (after ACK).
func (h *DiscordWebhookHandler) processInteraction(ctx context.Context, interaction DiscordInteraction, rawBody string) {
	// Note: caller already provides a context with 30s timeout; do not double-wrap.

	// Build PlatformUpdate from interaction.
	update := h.buildPlatformUpdate(interaction)
	if update == nil {
		h.log.Error().Msg("discord: could not build platform update")
		return
	}

	// Handle via command handler.
	response := h.cmdHandler.Handle(ctx, update)
	if response == "" {
		return
	}

	// Send response via Discord followup webhook.
	if interaction.Token != "" {
		if err := h.discord.RespondToInteraction(ctx, interaction.Token, response); err != nil {
			h.log.Error().Err(err).Msg("discord: respond to interaction")
		}
	}
}

// buildPlatformUpdate converts a Discord interaction into a PlatformUpdate.
func (h *DiscordWebhookHandler) buildPlatformUpdate(interaction DiscordInteraction) *models.PlatformUpdate {
	platformUserID := ""
	username := ""
	convID := ""
	command := ""
	var args []string
	rawText := ""

	switch data := interaction.Data.(type) {
	case DiscordApplicationCommandData:
		command = data.Name
		args = ToArgs(data.Options)
		rawText = "/" + command + " " + strings.Join(args, " ")

		// Extract user from resolved data map.
		if len(data.Resolved.Users) > 0 {
			for _, u := range data.Resolved.Users {
				platformUserID = u.ID
				username = u.Username
				break // use first resolved user
			}
		}
		convID = interaction.ChannelID

	case DiscordMessageComponentData:
		// Button/select menu interactions.
		rawText = data.CustomID
		convID = interaction.ChannelID
		platformUserID = interaction.Member.User.ID
		username = interaction.Member.User.Username

	case nil:
		return nil
	}

	if platformUserID == "" && interaction.User != (DiscordUser{}) {
		platformUserID = interaction.User.ID
		username = interaction.User.Username
	}

	return &models.PlatformUpdate{
		Platform:       "discord",
		UserID:         platformUserID,
		Username:       username,
		ConversationID: convID,
		MessageID:      interaction.Message.MessageID,
		Command:        command,
		Args:           args,
		RawText:        rawText,
		IsCommand:      command != "",
		Timestamp:      time.Unix(int64(interaction.Timestamp), 0),
	}
}

// verifyDiscordRequest verifies the Discord webhook request signature.
func (h *DiscordWebhookHandler) verifyDiscordRequest(r *http.Request, body []byte) bool {
	if h.secret == "" {
		// Secret not configured — reject in production, allow in development.
		// Caller should set DISCORD_WEBHOOK_SECRET env var.
		return false
	}

	// Try Ed25519 signature first (preferred by Discord).
	sigEd25519 := r.Header.Get("X-Signature-Ed25519")
	timestamp := r.Header.Get("X-Signature-Timestamp")

	if sigEd25519 != "" && timestamp != "" {
		decodedSig, err := hex.DecodeString(sigEd25519)
		if err == nil && len(decodedSig) == ed25519.SignatureSize {
			msg := []byte(timestamp + string(body))
			pubKey := []byte(h.secret) // In production: store actual Discord public key.
			if len(pubKey) == ed25519.PublicKeySize {
				return ed25519.Verify(pubKey, msg, decodedSig)
			}
		}
	}

	// Fallback: HMAC-SHA256 (X-Hub-Signature-256).
	sigHMAC := r.Header.Get("X-Hub-Signature-256")
	if sigHMAC == "" {
		return false
	}

	// Parse "sha256=<hex>" format.
	sigHex := sigHMAC
	if len(sigHMAC) > 7 && sigHMAC[:7] == "sha256=" {
		sigHex = sigHMAC[7:]
	}

	mac := hmac.New(sha256.New, []byte(h.secret))
	mac.Write([]byte(timestamp))
	mac.Write(body)
	expectedMAC := mac.Sum(nil)

	decodedSig, err := hex.DecodeString(sigHex)
	if err != nil {
		return false
	}

	return subtle.ConstantTimeCompare(decodedSig, expectedMAC) == 1
}

// --- Discord API types ---

// Discord interaction types.
const (
	DiscordInteractionTypePing                           = 1
	DiscordInteractionTypeApplicationCommand             = 2
	DiscordInteractionTypeMessageComponent               = 3
	DiscordInteractionTypeApplicationCommandAutocomplete = 4
	DiscordInteractionTypeModalSubmit                    = 5
	DiscordInteractionTypeDeferredChannelMessage         = 5 // ACK without showing a message (same value, Discord ignores duplicate)
	DiscordInteractionTypeDeferredUpdateMessage          = 6 // ACK for message updates
)

// DiscordInteraction represents a Discord interaction webhook payload.
type DiscordInteraction struct {
	ID            string         `json:"id"`
	ApplicationID string         `json:"application_id"`
	Type          int            `json:"type"`
	Data          any            `json:"data,omitempty"`
	GuildID       string         `json:"guild_id,omitempty"`
	ChannelID     string         `json:"channel_id,omitempty"`
	Member        *DiscordMember `json:"member,omitempty"`
	User          DiscordUser    `json:"user,omitempty"`
	Token         string         `json:"token"`
	Version       int            `json:"version,omitempty"`
	Message       DiscordMessage `json:"message,omitempty"`
	Timestamp     int64          `json:"timestamp,omitempty"`
}

// DiscordMember represents a Discord guild member.
type DiscordMember struct {
	User     DiscordUser `json:"user"`
	Nick     string      `json:"nick,omitempty"`
	Roles    []string    `json:"roles,omitempty"`
	JoinedAt string      `json:"joined_at,omitempty"`
}

// DiscordUser represents a Discord user.
type DiscordUser struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	Avatar        string `json:"avatar,omitempty"`
	Discriminator string `json:"discriminator,omitempty"`
	PublicFlags   int    `json:"public_flags,omitempty"`
}

// DiscordApplicationCommandData represents a slash command interaction data.
type DiscordApplicationCommandData struct {
	ID       string                     `json:"id"`
	Name     string                     `json:"name"`
	Type     int                        `json:"type"`
	Resolved DiscordResolvedData        `json:"resolved,omitempty"`
	Options  []DiscordApplicationOption `json:"options,omitempty"`
	CustomID string                     `json:"custom_id,omitempty"`
}

// ToArgs converts application options to command arguments.
func ToArgs(o []DiscordApplicationOption) []string {
	var args []string
	for _, opt := range o {
		if opt.Value != "" {
			args = append(args, opt.Value)
		}
	}
	return args
}

// DiscordApplicationOption represents a command option.
type DiscordApplicationOption struct {
	Name    string `json:"name"`
	Type    int    `json:"type"`
	Value   string `json:"value,omitempty"`
	Focused bool   `json:"focused,omitempty"`
}

// DiscordMessageComponentData represents a button/select menu interaction.
type DiscordMessageComponentData struct {
	CustomID    string `json:"custom_id"`
	CustomIDInt int    `json:"custom_id_int,omitempty"` // for numeric IDs
}

// DiscordResolvedData contains resolved resources from a command.
type DiscordResolvedData struct {
	Users    map[string]DiscordResolvedUser `json:"users,omitempty"`
	Roles    map[string]any                 `json:"roles,omitempty"`
	Channels map[string]any                 `json:"channels,omitempty"`
	Messages map[string]any                 `json:"messages,omitempty"`
}

// DiscordResolvedUser represents a resolved Discord user.
type DiscordResolvedUser struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	Avatar        string `json:"avatar,omitempty"`
	Discriminator string `json:"discriminator,omitempty"`
}

// DiscordMessage represents a Discord message.
type DiscordMessage struct {
	MessageID string      `json:"id"`
	ChannelID string      `json:"channel_id"`
	GuildID   string      `json:"guild_id,omitempty"`
	Content   string      `json:"content,omitempty"`
	Author    DiscordUser `json:"author,omitempty"`
	Timestamp string      `json:"timestamp,omitempty"`
}

// DiscordPingResponse is the response to a Discord interaction.
type DiscordPingResponse struct {
	Type int `json:"type"`
}
