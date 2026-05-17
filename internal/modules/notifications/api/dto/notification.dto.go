// Package dto defines request/response types for the notifications REST API.
package dto

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	entities "erg.ninja/internal/modules/notifications/domain/entity"
)

// ─── Send ──────────────────────────────────────────────────────────────────────

// SendRequest is the POST /api/notifications/send payload.
type SendRequest struct {
	UserID    string            `json:"userId" validate:"required"`
	Channel   string            `json:"channel" validate:"required,oneof=discord telegram whatsapp email"`
	Recipient string            `json:"recipient"`
	Subject   string            `json:"subject"`
	Template  string            `json:"template,omitempty"`
	Body      string            `json:"body,omitempty"`
	Data      map[string]string `json:"data,omitempty"`
}

// BatchSendRequest is the POST /api/notifications/batch payload.
type BatchSendRequest struct {
	Notifications []SendRequest `json:"notifications" validate:"required,min=1,max=100"`
}

// ─── Preferences ────────────────────────────────────────────────────────────────

// PreferenceRequest is the PUT /api/notifications/preferences payload.
type PreferenceRequest struct {
	// Deprecated: email is no longer read from the request body.
	// user_id is now extracted from the authenticated JWT token.
	Email      string           `json:"email,omitempty"`
	Discord    string           `json:"discord"` // "enabled" | "disabled" | "digest"
	Telegram   string           `json:"telegram"`
	WhatsApp   string           `json:"whatsapp"`
	Slack      string           `json:"slack"`
	Webhooks   []WebhookPrefDTO `json:"webhooks,omitempty"`
	DigestFreq string           `json:"digestFreq"` // "none" | "daily" | "weekly" | "monthly"
	DigestTime string           `json:"digestTime"` // HH:MM
	Language   string           `json:"language"`   // "vi" | "en"
}

// WebhookPrefDTO is a named webhook endpoint.
type WebhookPrefDTO struct {
	Name    string `json:"name"`
	URL     string `json:"url" validate:"required,url"`
	Enabled bool   `json:"enabled"`
}

// PreferenceResponse is the GET /api/notifications/preferences response.
type PreferenceResponse struct {
	UserID     string           `json:"userId"`
	Email      string           `json:"email"`
	Discord    string           `json:"discord"`
	Telegram   string           `json:"telegram"`
	WhatsApp   string           `json:"whatsapp"`
	Slack      string           `json:"slack"`
	Webhooks   []WebhookPrefDTO `json:"webhooks,omitempty"`
	DigestFreq string           `json:"digestFreq"`
	DigestTime string           `json:"digestTime"`
	Language   string           `json:"language"`
}

// ─── Responses ─────────────────────────────────────────────────────────────────

// NotificationResponse is the API response for a single notification.
type NotificationResponse struct {
	ID          string            `json:"id"`
	UserID      string            `json:"userId"`
	Type        string            `json:"type,omitempty"`
	Channel     string            `json:"channel"`
	Priority    string            `json:"priority,omitempty"`
	Recipient   string            `json:"recipient"`
	Title       string            `json:"title,omitempty"`
	Subject     string            `json:"subject,omitempty"`
	Message     string            `json:"message,omitempty"`
	Body        string            `json:"body,omitempty"`
	Template    string            `json:"template,omitempty"`
	Data        map[string]string `json:"data,omitempty"`
	Status      string            `json:"status"`
	RetryCount  int               `json:"retryCount"`
	ErrorMsg    string            `json:"errorMsg,omitempty"`
	ActionURL   string            `json:"actionUrl,omitempty"`
	ReadAt      *time.Time        `json:"readAt,omitempty"`
	ScheduledAt *time.Time        `json:"scheduledAt,omitempty"`
	SentAt      *time.Time        `json:"sentAt,omitempty"`
	DeliveredAt *time.Time        `json:"deliveredAt,omitempty"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}

// NotificationListResponse is the paginated list response.
type NotificationListResponse struct {
	Data        []NotificationResponse `json:"data"`
	Items       []NotificationResponse `json:"items"`
	Total       int64                  `json:"total"`
	Limit       int64                  `json:"limit"`
	Offset      int64                  `json:"offset"`
	UnreadCount int64                  `json:"unreadCount,omitempty"`
}

// StatsResponse is the GET /api/notifications/stats response.
type StatsResponse struct {
	Pending  int64            `json:"pending"`
	Sent     int64            `json:"sent"`
	Failed   int64            `json:"failed"`
	Retrying int64            `json:"retrying"`
	ByStatus map[string]int64 `json:"byStatus"`
}

// ChannelTestRequest is a POST /api/channels/{provider}/test payload.
type ChannelTestRequest struct {
	Recipient string `json:"recipient"` // webhook_url, chat_id, phone, or email
	Message   string `json:"message"`
	Subject   string `json:"subject,omitempty"`
}

// ChannelStatusResponse is the GET /api/channels/status response.
type ChannelStatusResponse struct {
	Discord  ChannelHealth `json:"discord"`
	Telegram ChannelHealth `json:"telegram"`
	WhatsApp ChannelHealth `json:"whatsapp"`
	Email    ChannelHealth `json:"email"`
}

// ChannelHealth describes a channel's current availability.
type ChannelHealth struct {
	Enabled bool   `json:"enabled"`
	Status  string `json:"status"` // "ok" | "rate_limited" | "error"
}

// ─── Mappers ─────────────────────────────────────────────────────────────────

// ToResponse converts a Notification entity to a response DTO.
func ToResponse(n *entities.Notification) NotificationResponse {
	userID := n.UserID.Hex()
	if n.UserID.IsZero() && n.UserIDText != "" {
		userID = n.UserIDText
	}
	title := n.Title
	if title == "" {
		title = n.Subject
	}
	message := n.Message
	if message == "" {
		message = n.Body
	}
	readAt := n.ReadAt
	if readAt == nil {
		readAt = n.LegacyReadAt
	}
	return NotificationResponse{
		ID:          n.ID.Hex(),
		UserID:      userID,
		Type:        n.Type,
		Channel:     string(n.Channel),
		Priority:    n.Priority,
		Recipient:   n.Recipient,
		Title:       title,
		Subject:     n.Subject,
		Message:     message,
		Body:        n.Body,
		Template:    n.Template,
		Data:        n.Data,
		Status:      string(n.Status),
		RetryCount:  n.RetryCount,
		ErrorMsg:    n.ErrorMsg,
		ActionURL:   n.ActionURL,
		ReadAt:      readAt,
		ScheduledAt: n.ScheduledAt,
		SentAt:      n.SentAt,
		DeliveredAt: n.DeliveredAt,
		CreatedAt:   n.CreatedAt,
		UpdatedAt:   n.UpdatedAt,
	}
}

// ToResponses converts a slice of Notification entities to response DTOs.
func ToResponses(nn []*entities.Notification) []NotificationResponse {
	out := make([]NotificationResponse, len(nn))
	for i, n := range nn {
		out[i] = ToResponse(n)
	}
	return out
}

// PreferenceToResponse converts a NotificationPreference entity to a response DTO.
func PreferenceToResponse(p *entities.NotificationPreference) PreferenceResponse {
	webhooks := make([]WebhookPrefDTO, len(p.Webhooks))
	for i, w := range p.Webhooks {
		webhooks[i] = WebhookPrefDTO{Name: w.Name, URL: w.URL, Enabled: w.Enabled}
	}
	return PreferenceResponse{
		UserID:     p.UserID.Hex(),
		Email:      string(p.Email),
		Discord:    string(p.Discord),
		Telegram:   string(p.Telegram),
		WhatsApp:   string(p.WhatsApp),
		Slack:      string(p.Slack),
		Webhooks:   webhooks,
		DigestFreq: string(p.DigestFreq),
		DigestTime: p.DigestTime,
		Language:   p.Language,
	}
}

// PreferenceFromRequest converts a PreferenceRequest DTO to an entity.
func PreferenceFromRequest(userID string, r PreferenceRequest) *entities.NotificationPreference {
	webhooks := make([]entities.WebhookPref, len(r.Webhooks))
	for i, w := range r.Webhooks {
		webhooks[i] = entities.WebhookPref{Name: w.Name, URL: w.URL, Enabled: w.Enabled}
	}
	uid, _ := bson.ObjectIDFromHex(userID)
	return &entities.NotificationPreference{
		UserID:     uid,
		Email:      entities.PreferenceValue(r.Email),
		Discord:    entities.PreferenceValue(r.Discord),
		Telegram:   entities.PreferenceValue(r.Telegram),
		WhatsApp:   entities.PreferenceValue(r.WhatsApp),
		Slack:      entities.PreferenceValue(r.Slack),
		Webhooks:   webhooks,
		DigestFreq: entities.DigestFrequency(r.DigestFreq),
		DigestTime: r.DigestTime,
		Language:   r.Language,
	}
}
