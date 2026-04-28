// Package notification defines interfaces and types for multi-channel notification providers.
package notification

import (
	"context"
	"time"
)

// ChannelType represents the delivery channel for a notification.
type ChannelType string

const (
	ChannelDiscord  ChannelType = "discord"
	ChannelTelegram ChannelType = "telegram"
	ChannelWhatsApp ChannelType = "whatsapp"
	ChannelEmail    ChannelType = "email"
	ChannelSlack    ChannelType = "slack"
	ChannelWebhook  ChannelType = "webhook"
	ChannelSMS      ChannelType = "sms"
)

// NotificationStatus represents the delivery status.
type NotificationStatus string

const (
	StatusPending   NotificationStatus = "pending"
	StatusSent      NotificationStatus = "sent"
	StatusDelivered NotificationStatus = "delivered"
	StatusFailed    NotificationStatus = "failed"
	StatusRetrying  NotificationStatus = "retrying"
)

// Notification represents a single notification message to be delivered.
type Notification struct {
	ID          string                 `json:"id"`
	Channel     ChannelType            `json:"channel"`
	Recipient   string                 `json:"recipient"` // e.g. email address, phone, user ID
	Subject     string                 `json:"subject"`   // for email
	Body        string                 `json:"body"`
	HTMLBody    string                 `json:"html_body,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	ScheduledAt *time.Time             `json:"scheduled_at,omitempty"`
	ExpiresAt   *time.Time             `json:"expires_at,omitempty"`
	Status      NotificationStatus     `json:"status"`
	RetryCount  int                    `json:"retry_count"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// NotifierProvider is the interface that all notification channel providers must implement.
type NotifierProvider interface {
	// Send delivers a notification through the provider's channel.
	// The implementation should be context-aware and respect deadlines.
	Send(ctx context.Context, msg *Notification) error

	// Supports reports whether this provider handles the given channel type.
	Supports(channel ChannelType) bool

	// Name returns the provider's unique identifier (e.g. "discord-webhook", "sendgrid").
	Name() string

	// RateLimit returns the provider's rate limit: requests per minute and minimum retry-after duration.
	RateLimit() (requestsPerMinute int, retryAfter time.Duration)
}

// ProviderRegistry holds all registered notification providers.
type ProviderRegistry struct {
	providers map[ChannelType]NotifierProvider
}

// NewProviderRegistry creates a new registry with the given providers.
func NewProviderRegistry(providers ...NotifierProvider) *ProviderRegistry {
	r := &ProviderRegistry{
		providers: make(map[ChannelType]NotifierProvider),
	}
	for _, p := range providers {
		// A provider can support multiple channels; register it for each.
		// For simplicity, we use the provider name as the key; callers should
		// use Supports() to check compatibility.
		if _, exists := r.providers[ChannelType(p.Name())]; !exists {
			r.providers[ChannelType(p.Name())] = p
		}
	}
	return r
}

// Register adds a provider to the registry.
func (r *ProviderRegistry) Register(provider NotifierProvider) {
	r.providers[ChannelType(provider.Name())] = provider
}

// Get returns the provider for the given channel type.
// If multiple providers support the same channel, the first registered one is returned.
func (r *ProviderRegistry) Get(channel ChannelType) NotifierProvider {
	for _, p := range r.providers {
		if p.Supports(channel) {
			return p
		}
	}
	return nil
}

// All returns all registered providers.
func (r *ProviderRegistry) All() []NotifierProvider {
	result := make([]NotifierProvider, 0, len(r.providers))
	for _, p := range r.providers {
		result = append(result, p)
	}
	return result
}

// Dispatch sends a notification through the appropriate provider for its channel.
// It returns an error if no provider is registered for the channel.
func Dispatch(ctx context.Context, registry *ProviderRegistry, msg *Notification) error {
	provider := registry.Get(msg.Channel)
	if provider == nil {
		return &NoProviderError{Channel: msg.Channel}
	}
	return provider.Send(ctx, msg)
}

// NoProviderError is returned when no provider is registered for a channel.
type NoProviderError struct {
	Channel ChannelType
}

func (e *NoProviderError) Error() string {
	return "notification: no provider registered for channel " + string(e.Channel)
}

// RetryPolicy defines when and how to retry a failed notification.
type RetryPolicy struct {
	MaxRetries    int
	InitialDelay  time.Duration
	MaxDelay      time.Duration
	BackoffFactor float64
}

// DefaultRetryPolicy is a sensible default retry policy.
var DefaultRetryPolicy = RetryPolicy{
	MaxRetries:    3,
	InitialDelay:  30 * time.Second,
	MaxDelay:      10 * time.Minute,
	BackoffFactor: 2.0,
}

// NextDelay calculates the delay for the given retry attempt using exponential backoff with jitter.
func (p RetryPolicy) NextDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return p.InitialDelay
	}
	delay := time.Duration(p.InitialDelay) * time.Duration(pow(p.BackoffFactor, attempt))
	if delay > p.MaxDelay {
		delay = p.MaxDelay
	}
	return delay
}

func pow(base float64, exp int) float64 {
	result := 1.0
	for i := 0; i < exp; i++ {
		result *= base
	}
	return result
}

// DigestNotification bundles multiple notifications into a single digest message.
type DigestNotification struct {
	Notifications []Notification `json:"notifications"`
	Subject       string         `json:"subject"`
	Summary       string         `json:"summary"`
	Channel       ChannelType    `json:"channel"`
	Recipient     string         `json:"recipient"`
}

// DigestProvider is an optional interface for providers that support digest delivery.
type DigestProvider interface {
	SendDigest(ctx context.Context, digest *DigestNotification) error
}
