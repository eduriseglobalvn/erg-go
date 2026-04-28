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

// WhatsAppProvider delivers notifications via the WhatsApp Business Cloud API.
type WhatsAppProvider struct {
	log       *logger.Logger
	client    *httppkg.Client
	apiBase   string
	phoneID   string
	token     string
	rateLimit time.Duration
}

// WhatsAppProviderOption configures the WhatsAppProvider.
type WhatsAppProviderOption func(*WhatsAppProvider)

// WithWhatsAppLogger sets the logger.
func WithWhatsAppLogger(log *logger.Logger) WhatsAppProviderOption {
	return func(p *WhatsAppProvider) { p.log = log }
}

// WithWhatsAppCredentials sets the phone ID and access token.
func WithWhatsAppCredentials(phoneID, token string) WhatsAppProviderOption {
	return func(p *WhatsAppProvider) {
		p.phoneID = phoneID
		p.token = token
	}
}

// NewWhatsAppProvider creates a new WhatsApp notification provider.
func NewWhatsAppProvider(opts ...WhatsAppProviderOption) *WhatsAppProvider {
	p := &WhatsAppProvider{
		log:       logger.NoOp(),
		client:    httppkg.NewClient(),
		apiBase:   "https://graph.facebook.com/v18.0",
		rateLimit: time.Minute / 250, // 250 contacts/min on standard tier
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

// Name returns the provider name.
func (p *WhatsAppProvider) Name() string { return "whatsapp" }

// Supports reports whether this provider handles WhatsApp channels.
func (p *WhatsAppProvider) Supports(channel entities.ChannelType) bool {
	return channel == entities.ChannelWhatsApp
}

// RateLimit returns WhatsApp's rate limit.
func (p *WhatsAppProvider) RateLimit() (int, time.Duration) { return 250, time.Minute }

// Send delivers a notification via the WhatsApp Business API.
func (p *WhatsAppProvider) Send(ctx context.Context, msg *entities.Notification) error {
	if p.phoneID == "" {
		return fmt.Errorf("whatsapp: phone ID not configured")
	}
	if p.token == "" {
		return fmt.Errorf("whatsapp: access token not configured")
	}
	if msg.Recipient == "" {
		return fmt.Errorf("whatsapp: phone number (recipient) is required")
	}

	// Compose WhatsApp text message payload.
	payload := waPayload{
		MessagingProduct: "whatsapp",
		To:               msg.Recipient,
		Type:             "text",
		Text: &waText{
			PreviewURL: false,
			Body:       p.buildBody(msg),
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("whatsapp: marshal payload: %w", err)
	}

	url := fmt.Sprintf("%s/%s/messages", p.apiBase, p.phoneID)
	headers := http.Header{
		"Authorization": {"Bearer " + p.token},
		"Content-Type":  {"application/json"},
	}

	resp, err := p.client.Post(ctx, url, body, headers)
	if err != nil {
		return fmt.Errorf("whatsapp: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		return nil
	}

	// Parse error response for more detail.
	var waErr waAPIError
	if err := json.NewDecoder(resp.Body).Decode(&waErr); err == nil && waErr.Error.Message != "" {
		return fmt.Errorf("whatsapp: API error %s: %s", waErr.Error.Type, waErr.Error.Message)
	}
	return fmt.Errorf("whatsapp: send returned status %d", resp.StatusCode)
}

// buildBody composes the WhatsApp message body.
func (p *WhatsAppProvider) buildBody(msg *entities.Notification) string {
	if msg.Subject != "" {
		return msg.Subject + "\n\n" + msg.Body
	}
	return msg.Body
}

// Client returns the underlying HTTP client.
func (p *WhatsAppProvider) Client() *httppkg.Client { return p.client }

// ─── WhatsApp API types ────────────────────────────────────────────────────────

type waPayload struct {
	MessagingProduct string      `json:"messaging_product"`
	To               string      `json:"to"`
	Type             string      `json:"type"`
	Text             *waText     `json:"text,omitempty"`
	Template         *waTemplate `json:"template,omitempty"`
}

type waText struct {
	PreviewURL bool   `json:"preview_url"`
	Body       string `json:"body"`
}

type waTemplate struct {
	Name       string        `json:"name"`
	Language   waLang        `json:"language"`
	Components []waComponent `json:"components,omitempty"`
}

type waLang struct {
	Code string `json:"code"`
}

type waComponent struct {
	Type    string            `json:"type"`
	SubType string            `json:"sub_type,omitempty"`
	Index   string            `json:"index,omitempty"`
	Params  []waTemplateParam `json:"parameters,omitempty"`
}

type waTemplateParam struct {
	Type     string      `json:"type"`
	Text     string      `json:"text,omitempty"`
	Currency *waCurrency `json:"currency,omitempty"`
	DateTime *waDateTime `json:"date_time,omitempty"`
}

type waCurrency struct {
	FallbackValue string `json:"fallback_value"`
	Code          string `json:"code"`
	Value         int    `json:"value_in_amount"`
}

type waDateTime struct {
	FallbackValue string `json:"fallback_value"`
}

type waAPIError struct {
	Error waErrorDetail `json:"error"`
}

type waErrorDetail struct {
	Message      string `json:"message"`
	Type         string `json:"type"`
	Code         int    `json:"code"`
	ErrorSubcode int    `json:"error_subcode,omitempty"`
	FbtraceID    string `json:"fbtrace_id,omitempty"`
}
