package platform

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"erg.ninja/pkg/logger"
)

const telegramAPIBase = "https://api.telegram.org/bot"

// TelegramClient is a minimal Telegram Bot API client.
type TelegramClient struct {
	token       string
	http        *http.Client
	log         *logger.Logger
	rateLimiter *telegramRateLimiter
}

// TelegramOption configures a TelegramClient.
type TelegramOption func(*TelegramClient)

// WithTelegramLogger sets the logger.
func WithTelegramLogger(log *logger.Logger) TelegramOption {
	return func(c *TelegramClient) {
		c.log = log
	}
}

// NewTelegramClient creates a TelegramClient with the given bot token.
func NewTelegramClient(token string, opts ...TelegramOption) *TelegramClient {
	c := &TelegramClient{
		token:       token,
		http:        &http.Client{Timeout: 10 * time.Second},
		log:         logger.NoOp(),
		rateLimiter: newTelegramRateLimiter(30, time.Second), // 30 msg/sec Telegram limit
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// telegramRateLimiter implements a token-bucket rate limiter for Telegram.
type telegramRateLimiter struct {
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

func newTelegramRateLimiter(limit int, window time.Duration) *telegramRateLimiter {
	return &telegramRateLimiter{
		tokens:     float64(limit),
		maxTokens:  float64(limit),
		refillRate: float64(limit) / window.Seconds(),
		lastRefill: time.Now(),
	}
}

func (r *telegramRateLimiter) allow() bool {
	now := time.Now()
	elapsed := now.Sub(r.lastRefill).Seconds()
	r.tokens = min(r.maxTokens, r.tokens+elapsed*r.refillRate)
	r.lastRefill = now
	if r.tokens >= 1 {
		r.tokens--
		return true
	}
	return false
}

// sendRequest performs a POST request to the Telegram Bot API.
func (c *TelegramClient) sendRequest(ctx context.Context, method string, body any) ([]byte, error) {
	if !c.rateLimiter.allow() {
		return nil, fmt.Errorf("telegram: rate limit exceeded")
	}

	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("telegram: marshal body: %w", err)
	}

	url := telegramAPIBase + c.token + "/" + method
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("telegram: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("telegram: do request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // max 1 MB
	if err != nil {
		return nil, fmt.Errorf("telegram: read body: %w", err)
	}

	type tgResponse struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
		ErrorCode   int    `json:"error_code"`
	}
	var tgResp tgResponse
	if err := json.Unmarshal(bodyBytes, &tgResp); err != nil {
		return nil, fmt.Errorf("telegram: unmarshal response: %w", err)
	}
	if !tgResp.OK {
		return nil, fmt.Errorf("telegram: api error %d: %s", tgResp.ErrorCode, tgResp.Description)
	}

	return bodyBytes, nil
}

// SendMessage sends a text message to a Telegram chat.
func (c *TelegramClient) SendMessage(ctx context.Context, chatID int64, text string) (*TelegramMessage, error) {
	type request struct {
		ChatID              int64  `json:"chat_id"`
		Text                string `json:"text"`
		ParseMode           string `json:"parse_mode,omitempty"` // "MarkdownV2" or "HTML"
		DisableNotification bool   `json:"disable_notification,omitempty"`
		ReplyToMessageID    int64  `json:"reply_to_message_id,omitempty"`
	}
	var resp TelegramMessage
	body, err := c.sendRequest(ctx, "sendMessage", request{
		ChatID: chatID,
		Text:   text,
	})
	if err != nil {
		return nil, fmt.Errorf("telegram: send message: %w", err)
	}
	// The Telegram API wraps the result in {ok: true, result: {...}}.
	type wrapper struct {
		Result json.RawMessage `json:"result"`
	}
	var w wrapper
	if err := json.Unmarshal(body, &w); err != nil {
		return nil, fmt.Errorf("telegram: unmarshal result: %w", err)
	}
	if err := json.Unmarshal(w.Result, &resp); err != nil {
		return nil, fmt.Errorf("telegram: parse message: %w", err)
	}
	return &resp, nil
}

// SendHTMLMessage sends an HTML-formatted message.
func (c *TelegramClient) SendHTMLMessage(ctx context.Context, chatID int64, html string) (*TelegramMessage, error) {
	type request struct {
		ChatID    int64  `json:"chat_id"`
		Text      string `json:"text"`
		ParseMode string `json:"parse_mode"`
	}
	var resp TelegramMessage
	body, err := c.sendRequest(ctx, "sendMessage", request{ChatID: chatID, Text: html, ParseMode: "HTML"})
	if err != nil {
		return nil, err
	}
	type wrapper struct {
		Result json.RawMessage `json:"result"`
	}
	var w wrapper
	if err := json.Unmarshal(body, &w); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(w.Result, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// EditMessageText edits an existing message.
func (c *TelegramClient) EditMessageText(ctx context.Context, chatID int64, messageID int64, text string) error {
	type request struct {
		ChatID    int64  `json:"chat_id"`
		MessageID int64  `json:"message_id"`
		Text      string `json:"text"`
		ParseMode string `json:"parse_mode,omitempty"`
	}
	_, err := c.sendRequest(ctx, "editMessageText", request{
		ChatID:    chatID,
		MessageID: messageID,
		Text:      text,
	})
	return err
}

// AnswerCallbackQuery answers a callback query from an inline button.
func (c *TelegramClient) AnswerCallbackQuery(ctx context.Context, callbackID string, text string) error {
	type request struct {
		CallbackQueryID string `json:"callback_query_id"`
		Text            string `json:"text,omitempty"`
		ShowAlert       bool   `json:"show_alert,omitempty"`
	}
	_, err := c.sendRequest(ctx, "answerCallbackQuery", request{
		CallbackQueryID: callbackID,
		Text:            text,
	})
	return err
}

// SendChatAction sends a chat action (typing, etc.) to indicate activity.
func (c *TelegramClient) SendChatAction(ctx context.Context, chatID int64, action string) error {
	type request struct {
		ChatID int64  `json:"chat_id"`
		Action string `json:"action"`
	}
	_, err := c.sendRequest(ctx, "sendChatAction", request{ChatID: chatID, Action: action})
	return err
}

// TelegramMessage represents a Telegram message object.
type TelegramMessage struct {
	MessageID int64            `json:"message_id"`
	Chat      TelegramChat     `json:"chat"`
	Text      string           `json:"text,omitempty"`
	From      *TelegramUser    `json:"from,omitempty"`
	Date      int64            `json:"date"`
	Entities  []TelegramEntity `json:"entities,omitempty"`
}

// TelegramChat represents a Telegram chat.
type TelegramChat struct {
	ID    int64  `json:"id"`
	Title string `json:"title,omitempty"`
	Type  string `json:"type"` // "private", "group", "supergroup", "channel"
}

// TelegramUser represents a Telegram user.
type TelegramUser struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name,omitempty"`
	Username  string `json:"username,omitempty"`
}

// TelegramEntity represents a message entity (e.g. bot command).
type TelegramEntity struct {
	Type   string `json:"type"`
	Offset int    `json:"offset"`
	Length int    `json:"length"`
}

// TelegramUpdate represents an incoming update from Telegram webhook.
type TelegramUpdate struct {
	UpdateID      int64                  `json:"update_id"`
	Message       *TelegramMessage       `json:"message,omitempty"`
	CallbackQuery *TelegramCallbackQuery `json:"callback_query,omitempty"`
}

// TelegramCallbackQuery represents a callback query.
type TelegramCallbackQuery struct {
	ID      string           `json:"id"`
	From    *TelegramUser    `json:"from"`
	Message *TelegramMessage `json:"message,omitempty"`
	Data    string           `json:"data,omitempty"`
}

// ParseTelegramUpdate parses a raw JSON payload into a TelegramUpdate.
func ParseTelegramUpdate(b []byte) (*TelegramUpdate, error) {
	var u TelegramUpdate
	if err := json.Unmarshal(b, &u); err != nil {
		return nil, fmt.Errorf("telegram: parse update: %w", err)
	}
	return &u, nil
}

// VerifyTelegramRequest verifies a Telegram webhook request using HMAC-SHA256.
func VerifyTelegramRequest(dataCheckString, botToken, hash string) bool {
	if hash == "" || botToken == "" {
		return false
	}
	// Decode the hex hash from the header.
	decodedSig, err := hex.DecodeString(hash)
	if err != nil {
		return false
	}
	expectedMAC := hmacSHA256Raw([]byte(dataCheckString), []byte(botToken))
	return hmacEqual(decodedSig, expectedMAC)
}

// ExtractCommandFromMessage parses a Telegram message and returns the command and arguments.
func ExtractCommandFromMessage(msg *TelegramMessage) (cmd string, args []string) {
	if msg == nil || msg.Text == "" {
		return "", nil
	}
	text := msg.Text
	if len(text) > 0 && text[0] == '/' {
		parts := splitTextArgs(text[1:])
		if len(parts) > 0 {
			return parts[0], parts[1:]
		}
	}
	return "", nil
}

// splitTextArgs splits "@botname args" or "command args" into parts.
func splitTextArgs(text string) []string {
	var parts []string
	var current []byte
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c == ' ' {
			if len(current) > 0 {
				parts = append(parts, string(current))
				current = nil
			}
			continue
		}
		current = append(current, c)
	}
	if len(current) > 0 {
		parts = append(parts, string(current))
	}
	return parts
}

// ParseChatID extracts the int64 chat ID from a TelegramMessage.
func ParseChatID(msg *TelegramMessage) int64 {
	return msg.Chat.ID
}

// MessageID extracts the message ID from a TelegramMessage.
func MessageID(msg *TelegramMessage) int64 {
	if msg == nil {
		return 0
	}
	return msg.MessageID
}
