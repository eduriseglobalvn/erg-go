// Package platform provides platform-specific API clients for Discord and Telegram.
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

const discordAPIBase = "https://discord.com/api/v10"

// DiscordClient is a minimal Discord API client for sending messages via webhooks
// and interacting with the Discord API using a bot token.
type DiscordClient struct {
	token       string
	http        *http.Client
	log         *logger.Logger
	rateLimiter *discordRateLimiter
}

// DiscordOption configures a DiscordClient.
type DiscordOption func(*DiscordClient)

// WithDiscordLogger sets the logger.
func WithDiscordLogger(log *logger.Logger) DiscordOption {
	return func(c *DiscordClient) {
		c.log = log
	}
}

// NewDiscordClient creates a DiscordClient with the given bot token.
func NewDiscordClient(token string, opts ...DiscordOption) *DiscordClient {
	c := &DiscordClient{
		token: token,
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
		log:         logger.NoOp(),
		rateLimiter: newDiscordRateLimiter(100, time.Minute), // 100 req/min
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// discordRateLimiter implements a token-bucket rate limiter for the Discord API.
type discordRateLimiter struct {
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

func newDiscordRateLimiter(limit int, window time.Duration) *discordRateLimiter {
	return &discordRateLimiter{
		tokens:     float64(limit),
		maxTokens:  float64(limit),
		refillRate: float64(limit) / window.Seconds(),
		lastRefill: time.Now(),
	}
}

func (r *discordRateLimiter) allow() bool {
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

// sendRequest performs an authenticated HTTP request to the Discord API.
func (c *DiscordClient) sendRequest(ctx context.Context, method, endpoint string, body any) ([]byte, error) {
	if !c.rateLimiter.allow() {
		return nil, fmt.Errorf("discord: rate limit exceeded")
	}

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("discord: marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, discordAPIBase+endpoint, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("discord: new request: %w", err)
	}

	req.Header.Set("Authorization", "Bot "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "DiscordBot (erg.ninja, 1.0)")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("discord: do request: %w", err)
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // max 1 MB
	if err != nil {
		return nil, fmt.Errorf("discord: read body: %w", err)
	}

	if resp.StatusCode >= 400 {
		c.log.Warn().Int("status", resp.StatusCode).Str("endpoint", endpoint).Msg("discord: api error")
		return nil, fmt.Errorf("discord: api error %d: %s", resp.StatusCode, string(b))
	}

	return b, nil
}

// SendDM sends a direct message to a Discord user by their user ID.
func (c *DiscordClient) SendDM(ctx context.Context, userID, content string) error {
	// Step 1: Open/create DM channel.
	type createDMRequest struct {
		RecipientID string `json:"recipient_id"`
	}
	type dmResponse struct {
		ID string `json:"id"`
	}

	b, err := c.sendRequest(ctx, "POST", "/users/@me/channels", createDMRequest{RecipientID: userID})
	if err != nil {
		return fmt.Errorf("discord: open DM channel: %w", err)
	}

	var dm dmResponse
	if err := json.Unmarshal(b, &dm); err != nil {
		return fmt.Errorf("discord: unmarshal DM response: %w", err)
	}

	// Step 2: Send message to channel.
	type msgRequest struct {
		Content string `json:"content"`
	}
	_, err = c.sendRequest(ctx, "POST", "/channels/"+dm.ID+"/messages", msgRequest{Content: content})
	if err != nil {
		return fmt.Errorf("discord: send message: %w", err)
	}

	return nil
}

// SendChannelMessage sends a message to a Discord channel (by channel ID).
func (c *DiscordClient) SendChannelMessage(ctx context.Context, channelID, content string) error {
	type msgRequest struct {
		Content string `json:"content"`
	}
	_, err := c.sendRequest(ctx, "POST", "/channels/"+channelID+"/messages", msgRequest{Content: content})
	if err != nil {
		return fmt.Errorf("discord: send channel message: %w", err)
	}
	return nil
}

// SendEmbed sends a rich embed message to a Discord channel.
func (c *DiscordClient) SendEmbed(ctx context.Context, channelID string, embed DiscordEmbed) error {
	type msgRequest struct {
		Embeds []DiscordEmbed `json:"embeds"`
	}
	_, err := c.sendRequest(ctx, "POST", "/channels/"+channelID+"/messages", msgRequest{Embeds: []DiscordEmbed{embed}})
	if err != nil {
		return fmt.Errorf("discord: send embed: %w", err)
	}
	return nil
}

// DiscordEmbed represents a Discord embed object.
type DiscordEmbed struct {
	Title       string         `json:"title,omitempty"`
	Description string         `json:"description,omitempty"`
	URL         string         `json:"url,omitempty"`
	Color       int            `json:"color,omitempty"`
	Author      *DiscordAuthor `json:"author,omitempty"`
	Fields      []DiscordField `json:"fields,omitempty"`
	Footer      *DiscordFooter `json:"footer,omitempty"`
	Timestamp   string         `json:"timestamp,omitempty"`
}

// DiscordAuthor represents a Discord embed author.
type DiscordAuthor struct {
	Name    string `json:"name,omitempty"`
	IconURL string `json:"icon_url,omitempty"`
}

// DiscordField represents a Discord embed field.
type DiscordField struct {
	Name   string `json:"name,omitempty"`
	Value  string `json:"value,omitempty"`
	Inline bool   `json:"inline,omitempty"`
}

// DiscordFooter represents a Discord embed footer.
type DiscordFooter struct {
	Text    string `json:"text,omitempty"`
	IconURL string `json:"icon_url,omitempty"`
}

// InteractionResponse is the payload sent back to Discord to acknowledge an interaction.
type InteractionResponse struct {
	Type int              `json:"type"` // 4 = ChannelMessageWithSource
	Data *InteractionData `json:"data,omitempty"`
}

// InteractionData contains the message data for an interaction response.
type InteractionData struct {
	Content    string         `json:"content,omitempty"`
	Embeds     []DiscordEmbed `json:"embeds,omitempty"`
	Components []any          `json:"components,omitempty"`
	Flags      int            `json:"flags,omitempty"`
}

// RespondToInteraction sends a response to a Discord interaction (webhook).
func (c *DiscordClient) RespondToInteraction(ctx context.Context, interactionToken, content string) error {
	payload := InteractionResponse{
		Type: 4, // ChannelMessageWithSource
		Data: &InteractionData{Content: content},
	}
	_, err := c.sendRequest(ctx, "POST", "/webhooks/interactions/"+interactionToken, payload)
	return err
}

// VerifyHMAC verifies a Discord webhook request using HMAC-SHA256.
// Discord uses "sha256=<hex>" format in the X-Hub-Signature-256 header.
func VerifyHMAC(body []byte, timestamp, signature, secret string) bool {
	if signature == "" || secret == "" {
		return false
	}

	// Remove "sha256=" prefix if present.
	sig := signature
	if len(signature) > 7 && signature[:7] == "sha256=" {
		sig = signature[7:]
	}

	// Compute HMAC-SHA256(timestamp + body) using raw bytes.
	msg := append([]byte(timestamp), body...)
	expectedMAC := hmacSHA256Raw(msg, []byte(secret))

	// Decode the hex signature.
	decodedSig, err := hex.DecodeString(sig)
	if err != nil {
		return false
	}

	// Constant-time comparison.
	return hmacEqual(decodedSig, expectedMAC)
}
