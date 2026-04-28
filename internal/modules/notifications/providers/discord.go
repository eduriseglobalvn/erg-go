// Package providers implements multi-channel notification delivery.
package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"erg.ninja/internal/modules/notifications/entities"
	httppkg "erg.ninja/pkg/http"
	"erg.ninja/pkg/logger"
)

// DiscordProvider delivers notifications via Discord webhooks.
type DiscordProvider struct {
	log       *logger.Logger
	client    *httppkg.Client
	rateLimit time.Duration
}

// DiscordProviderOption configures the DiscordProvider.
type DiscordProviderOption func(*DiscordProvider)

// WithDiscordLogger sets the logger.
func WithDiscordLogger(log *logger.Logger) DiscordProviderOption {
	return func(p *DiscordProvider) { p.log = log }
}

// WithDiscordRateLimit sets the rate limit interval between requests.
// Default: 250ms (200 req/min for Discord webhooks).
func WithDiscordRateLimit(d time.Duration) DiscordProviderOption {
	return func(p *DiscordProvider) { p.rateLimit = d }
}

// NewDiscordProvider creates a new Discord notification provider.
func NewDiscordProvider(opts ...DiscordProviderOption) *DiscordProvider {
	p := &DiscordProvider{
		log:       logger.NoOp(),
		client:    httppkg.NewClient(),
		rateLimit: 250 * time.Millisecond,
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

// Name returns the provider name.
func (p *DiscordProvider) Name() string { return "discord-webhook" }

// Supports reports whether this provider handles Discord channels.
func (p *DiscordProvider) Supports(channel entities.ChannelType) bool {
	return channel == entities.ChannelDiscord
}

// RateLimit returns Discord's rate limit: 200 req/min, retry after 1s.
func (p *DiscordProvider) RateLimit() (int, time.Duration) { return 200, time.Second }

// Send delivers a notification via a Discord webhook embed.
func (p *DiscordProvider) Send(ctx context.Context, msg *entities.Notification) error {
	if msg.Recipient == "" {
		return fmt.Errorf("discord: webhook URL (recipient) is required")
	}

	embed := discordEmbed{
		Title:       msg.Subject,
		Description: msg.Body,
		Color:       0x5865F2, // Discord blurple
		Footer: &discordFooter{
			Text: "ERG Bot",
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	if msg.Metadata != nil {
		if url, ok := msg.Metadata["thumbnail"].(string); ok && url != "" {
			embed.Thumbnail = &discordImage{URL: url}
		}
		if url, ok := msg.Metadata["image"].(string); ok && url != "" {
			embed.Image = &discordImage{URL: url}
		}
	}

	payload := discordPayload{
		Username: "ERG Bot",
		Embeds:   []discordEmbed{embed},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("discord: marshal payload: %w", err)
	}

	headers := http.Header{"Content-Type": {"application/json"}}
	resp, err := p.client.Post(ctx, msg.Recipient, body, headers)
	if err != nil {
		return fmt.Errorf("discord: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
		return nil
	}

	// Discord returns 429 Too Many Requests on rate limit.
	if resp.StatusCode == http.StatusTooManyRequests {
		return fmt.Errorf("discord: rate limited")
	}

	return fmt.Errorf("discord: webhook returned status %d", resp.StatusCode)
}

// Client returns the underlying HTTP client for direct use.
func (p *DiscordProvider) Client() *httppkg.Client { return p.client }

// ─── Discord payload types ─────────────────────────────────────────────────────

type discordPayload struct {
	Username  string         `json:"username,omitempty"`
	AvatarURL string         `json:"avatar_url,omitempty"`
	Embeds    []discordEmbed `json:"embeds,omitempty"`
	Content   string         `json:"content,omitempty"`
}

type discordEmbed struct {
	Title       string         `json:"title,omitempty"`
	Description string         `json:"description,omitempty"`
	Color       int            `json:"color,omitempty"`
	URL         string         `json:"url,omitempty"`
	Thumbnail   *discordImage  `json:"thumbnail,omitempty"`
	Image       *discordImage  `json:"image,omitempty"`
	Footer      *discordFooter `json:"footer,omitempty"`
	Timestamp   string         `json:"timestamp,omitempty"`
	Fields      []discordField `json:"fields,omitempty"`
}

type discordImage struct {
	URL string `json:"url"`
}

type discordFooter struct {
	Text         string `json:"text"`
	IconURL      string `json:"icon_url,omitempty"`
	ProxyIconURL string `json:"proxy_icon_url,omitempty"`
}

type discordField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}
