package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/modules/bot/application/service"
	"erg.ninja/internal/modules/bot/domain/entity"
	"erg.ninja/internal/modules/bot/infrastructure/platform"
	"erg.ninja/pkg/logger"
)

// TelegramWebhookHandler handles incoming Telegram webhook requests.
type TelegramWebhookHandler struct {
	cmdHandler *services.CommandHandler
	linkSvc    *services.LinkService
	convSvc    *services.ConversationService
	telegram   *platform.TelegramClient
	botToken   string
	log        *logger.Logger
}

// NewTelegramWebhookHandler creates a TelegramWebhookHandler.
func NewTelegramWebhookHandler(
	cmdHandler *services.CommandHandler,
	linkSvc *services.LinkService,
	convSvc *services.ConversationService,
	telegram *platform.TelegramClient,
	botToken string,
	log *logger.Logger,
) *TelegramWebhookHandler {
	return &TelegramWebhookHandler{
		cmdHandler: cmdHandler,
		linkSvc:    linkSvc,
		convSvc:    convSvc,
		telegram:   telegram,
		botToken:   botToken,
		log:        log,
	}
}

// RegisterRoutes mounts the Telegram webhook routes onto the given router.
func (h *TelegramWebhookHandler) RegisterRoutes(r *gin.Engine) {
	r.POST("/webhooks/telegram", h.HandleTelegramWebhook)
}

// HandleTelegramWebhook handles POST /webhooks/telegram.
// Telegram verifies webhooks by sending a GET request to your server first to confirm
// the token, then sends POST with JSON updates.
// @Summary Handle Telegram webhook
// @Description Receives and processes Telegram webhook updates.
// @Tags Bot Webhooks
// @Accept json
// @Produce json
// @Success 200 {object} map[string]any
// @Router /webhooks/telegram [post]
func (h *TelegramWebhookHandler) HandleTelegramWebhook(ctx *gin.Context) {
	// Read body.
	body, err := io.ReadAll(io.LimitReader(ctx.Request.Body, 1<<20)) // 1 MB max
	if err != nil {
		h.log.Error().Err(err).Msg("telegram: read body")
		ctx.String(http.StatusBadRequest, "bad request")
		return
	}

	// Verify Telegram request.
	if !h.verifyTelegramRequest(ctx.Request, body) {
		h.log.Warn().Msg("telegram: verification failed")
		ctx.String(http.StatusUnauthorized, "unauthorized")
		return
	}

	// Parse the update.
	update, err := platform.ParseTelegramUpdate(body)
	if err != nil {
		h.log.Error().Err(err).Msg("telegram: parse update")
		ctx.String(http.StatusBadRequest, "bad request")
		return
	}

	// Process the update asynchronously with a fresh context.
	asyncCtx, asyncCancel := context.WithTimeout(context.Background(), 30*time.Second)
	go func() {
		defer asyncCancel()
		h.processUpdate(asyncCtx, update)
	}()

	// Respond with 200 OK immediately (Telegram expects fast response).
	ctx.JSON(http.StatusOK, map[string]bool{"ok": true})
}

// processUpdate handles a Telegram update asynchronously.
// Note: caller already provides a context with 30s timeout; do not double-wrap.
func (h *TelegramWebhookHandler) processUpdate(ctx context.Context, tgUpdate *platform.TelegramUpdate) {

	if tgUpdate == nil {
		return
	}

	var platformUpdate *models.PlatformUpdate

	if tgUpdate.Message != nil {
		platformUpdate = h.messageToPlatformUpdate(tgUpdate.Message)

		// Check for commands (start with /).
		cmd, args := platform.ExtractCommandFromMessage(tgUpdate.Message)
		platformUpdate.Command = cmd
		platformUpdate.Args = args
		platformUpdate.IsCommand = cmd != ""
		if cmd != "" {
			platformUpdate.RawText = "/" + cmd
			if len(args) > 0 {
				platformUpdate.RawText += " " + joinStrings(args, " ")
			}
		} else {
			platformUpdate.RawText = tgUpdate.Message.Text
		}
	} else if tgUpdate.CallbackQuery != nil {
		platformUpdate = h.callbackToPlatformUpdate(tgUpdate.CallbackQuery)
	} else {
		h.log.Debug().Int64("update_id", tgUpdate.UpdateID).Msg("telegram: unhandled update type")
		return
	}

	if platformUpdate == nil {
		return
	}

	// Handle via command handler.
	response := h.cmdHandler.Handle(ctx, platformUpdate)
	if response == "" {
		return
	}

	// Send response.
	chatID, err := strconv.ParseInt(platformUpdate.ConversationID, 10, 64)
	if err != nil {
		h.log.Error().Err(err).Msg("telegram: invalid chat_id")
		return
	}

	// Use HTML mode for better formatting.
	if _, err := h.telegram.SendHTMLMessage(ctx, chatID, response); err != nil {
		h.log.Error().Err(err).Msg("telegram: send message")
	}
}

// messageToPlatformUpdate converts a Telegram message to a PlatformUpdate.
func (h *TelegramWebhookHandler) messageToPlatformUpdate(msg *platform.TelegramMessage) *models.PlatformUpdate {
	userID := ""
	username := ""
	displayName := ""

	if msg.From != nil {
		userID = strconv.FormatInt(msg.From.ID, 10)
		username = msg.From.Username
		if msg.From.LastName != "" {
			displayName = msg.From.FirstName + " " + msg.From.LastName
		} else {
			displayName = msg.From.FirstName
		}
	}

	convID := strconv.FormatInt(msg.Chat.ID, 10)
	replyToID := ""

	return &models.PlatformUpdate{
		Platform:       "telegram",
		UserID:         userID,
		Username:       username,
		DisplayName:    displayName,
		ConversationID: convID,
		MessageID:      strconv.FormatInt(msg.MessageID, 10),
		RawText:        msg.Text,
		ReplyToID:      replyToID,
		Timestamp:      time.Unix(msg.Date, 0),
		Metadata: map[string]string{
			"chat_type":  msg.Chat.Type,
			"chat_title": msg.Chat.Title,
		},
	}
}

// callbackToPlatformUpdate converts a Telegram callback query to a PlatformUpdate.
func (h *TelegramWebhookHandler) callbackToPlatformUpdate(cq *platform.TelegramCallbackQuery) *models.PlatformUpdate {
	userID := ""
	username := ""

	if cq.From != nil {
		userID = strconv.FormatInt(cq.From.ID, 10)
		username = cq.From.Username
	}

	convID := ""
	if cq.Message != nil {
		convID = strconv.FormatInt(cq.Message.Chat.ID, 10)
	}

	return &models.PlatformUpdate{
		Platform:       "telegram",
		UserID:         userID,
		Username:       username,
		ConversationID: convID,
		MessageID:      strconv.FormatInt(cq.Message.MessageID, 10),
		IsCallback:     true,
		CallbackData:   cq.Data,
		RawText:        cq.Data,
		Timestamp:      time.Now(),
	}
}

// verifyTelegramRequest verifies the Telegram webhook request using HMAC-SHA256.
func (h *TelegramWebhookHandler) verifyTelegramRequest(r *http.Request, body []byte) bool {
	if h.botToken == "" {
		// Skip in development.
		return true
	}

	// Check secret token header (preferred method in recent Telegram Bot API).
	secret := r.Header.Get("X-Telegram-Bot-Api-Secret-Token")
	if secret != "" {
		// Constant-time comparison.
		if subtleCompare(secret, h.botToken) {
			return true
		}
	}

	// Also verify HMAC of the body using the bot token.
	hashHeader := r.Header.Get("X-Telegram-Bot-Api-Signature")
	if hashHeader == "" {
		return false // No HMAC header — reject unknown requests.
	}

	mac := hmac.New(sha256.New, []byte(h.botToken))
	mac.Write(body)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	return subtleCompare(hashHeader, expectedMAC)
}

// subtleCompare performs constant-time string comparison.
func subtleCompare(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var result byte
	for i := 0; i < len(a); i++ {
		result |= a[i] ^ b[i]
	}
	return result == 0
}

// joinStrings joins a slice of strings with a separator.
func joinStrings(ss []string, sep string) string {
	if len(ss) == 0 {
		return ""
	}
	result := ss[0]
	for i := 1; i < len(ss); i++ {
		result += sep + ss[i]
	}
	return result
}

// serveLinkCommand handles the /link Telegram command.
func (h *TelegramWebhookHandler) serveLinkCommand(ctx context.Context, msg *platform.TelegramMessage) {
	chatID := msg.Chat.ID

	// Check if the message contains a link code.
	text := msg.Text
	if text == "" {
		return
	}

	// Format: /link <code> or just the 6-char code.
	code := text
	if len(code) > 5 && code[:5] == "/link" {
		code = trimSpace(text[5:])
	}

	code = trimSpace(code)
	if len(code) == 6 {
		// Verify the link code.
		platformUserID := strconv.FormatInt(msg.From.ID, 10)
		_, err := h.linkSvc.VerifyLinkCode(ctx, "telegram", platformUserID, code)
		if err != nil {
			h.telegram.SendMessage(ctx, chatID, fmt.Sprintf("Mã không hợp lệ hoặc đã hết hạn: %v", err))
			return
		}
		h.telegram.SendHTMLMessage(ctx, chatID, "Tài khoản Telegram của bạn đã được liên kết thành công!")
	}
}

// trimSpace trims leading and trailing whitespace.
func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n' || s[0] == '\r') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t' || s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
