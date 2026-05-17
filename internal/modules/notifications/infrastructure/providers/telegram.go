package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	entities "erg.ninja/internal/modules/notifications/domain/entity"
	httppkg "erg.ninja/pkg/http"
	"erg.ninja/pkg/logger"
)

// TelegramProvider delivers notifications via the Telegram Bot API.
type TelegramProvider struct {
	log       *logger.Logger
	client    *httppkg.Client
	apiBase   string
	rateLimit time.Duration
}

// TelegramProviderOption configures the TelegramProvider.
type TelegramProviderOption func(*TelegramProvider)

// WithTelegramLogger sets the logger.
func WithTelegramLogger(log *logger.Logger) TelegramProviderOption {
	return func(p *TelegramProvider) { p.log = log }
}

// WithTelegramAPIBase sets the Telegram Bot API base URL.
func WithTelegramAPIBase(base string) TelegramProviderOption {
	return func(p *TelegramProvider) { p.apiBase = base }
}

// NewTelegramProvider creates a new Telegram notification provider.
func NewTelegramProvider(botToken string, opts ...TelegramProviderOption) *TelegramProvider {
	p := &TelegramProvider{
		log:       logger.NoOp(),
		client:    httppkg.NewClient(),
		apiBase:   "https://api.telegram.org/bot" + botToken,
		rateLimit: time.Second / 30, // 30 msg/sec limit
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

// Name returns the provider name.
func (p *TelegramProvider) Name() string { return "telegram" }

// Supports reports whether this provider handles Telegram channels.
func (p *TelegramProvider) Supports(channel entities.ChannelType) bool {
	return channel == entities.ChannelTelegram
}

// RateLimit returns Telegram's rate limit: 30 req/sec.
func (p *TelegramProvider) RateLimit() (int, time.Duration) { return 30, time.Second }

// Send delivers a notification via the Telegram Bot API sendMessage endpoint.
func (p *TelegramProvider) Send(ctx context.Context, msg *entities.Notification) error {
	if msg.Recipient == "" {
		return fmt.Errorf("telegram: chat_id (recipient) is required")
	}

	chatID, err := strconv.ParseInt(msg.Recipient, 10, 64)
	if err != nil {
		return fmt.Errorf("telegram: invalid chat_id %q: %w", msg.Recipient, err)
	}

	payload := telegramSendMessage{
		ChatID:    chatID,
		Text:      p.buildText(msg),
		ParseMode: "HTML",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("telegram: marshal payload: %w", err)
	}

	headers := http.Header{"Content-Type": {"application/json"}}
	resp, err := p.client.Post(ctx, p.apiBase+"/sendMessage", body, headers)
	if err != nil {
		return fmt.Errorf("telegram: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram: sendMessage returned status %d", resp.StatusCode)
	}

	var result telegramResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("telegram: decode response: %w", err)
	}
	if !result.OK {
		return fmt.Errorf("telegram: API error: %s", result.Description)
	}
	return nil
}

// buildText composes the message body with optional subject.
func (p *TelegramProvider) buildText(msg *entities.Notification) string {
	var sb strings.Builder
	if msg.Subject != "" {
		sb.WriteString(msg.Subject)
		sb.WriteString("\n\n")
	}
	sb.WriteString(msg.Body)
	return sb.String()
}

// SendWithReply sends a message and optionally replies to a message.
func (p *TelegramProvider) SendWithReply(ctx context.Context, chatID int64, text string, replyTo int64) error {
	payload := telegramSendMessage{
		ChatID:    chatID,
		Text:      text,
		ParseMode: "HTML",
	}
	if replyTo > 0 {
		payload.ReplyToMessageID = replyTo
	}

	body, _ := json.Marshal(payload)
	headers := http.Header{"Content-Type": {"application/json"}}
	resp, err := p.client.Post(ctx, p.apiBase+"/sendMessage", body, headers)
	if err != nil {
		return fmt.Errorf("telegram: SendWithReply: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram: SendWithReply returned %d", resp.StatusCode)
	}
	return nil
}

// GetMe returns bot information.
func (p *TelegramProvider) GetMe(ctx context.Context) (*tgUser, error) {
	resp, err := p.client.Get(ctx, p.apiBase+"/getMe", nil)
	if err != nil {
		return nil, fmt.Errorf("telegram: GetMe: %w", err)
	}
	defer resp.Body.Close()

	var result telegramResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("telegram: decode GetMe: %w", err)
	}
	if !result.OK {
		return nil, fmt.Errorf("telegram: GetMe error: %s", result.Description)
	}
	return result.Result, nil
}

// SetWebhook registers the webhook URL for the bot.
func (p *TelegramProvider) SetWebhook(ctx context.Context, webhookURL string) error {
	params := url.Values{"url": {webhookURL}}
	resp, err := p.client.Post(ctx, p.apiBase+"/setWebhook?"+params.Encode(), nil, nil)
	if err != nil {
		return fmt.Errorf("telegram: SetWebhook: %w", err)
	}
	defer resp.Body.Close()
	return nil
}

// ─── Telegram API types ───────────────────────────────────────────────────────

type telegramSendMessage struct {
	ChatID                int64  `json:"chat_id"`
	Text                  string `json:"text"`
	ParseMode             string `json:"parse_mode,omitempty"`
	ReplyToMessageID      int64  `json:"reply_to_message_id,omitempty"`
	DisableWebPagePreview bool   `json:"disable_web_page_preview,omitempty"`
}

type telegramResponse struct {
	OK          bool    `json:"ok"`
	Description string  `json:"description,omitempty"`
	ErrorCode   int     `json:"error_code,omitempty"`
	Result      *tgUser `json:"result,omitempty"`
}

type tgUser struct {
	ID           int64  `json:"id"`
	IsBot        bool   `json:"is_bot"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name,omitempty"`
	Username     string `json:"username,omitempty"`
	LanguageCode string `json:"language_code,omitempty"`
}
