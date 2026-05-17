// Package entities defines MongoDB document models for the notifications module.
package entities

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// NotificationCollection is the MongoDB collection name.
const NotificationCollection = "notifications"

// NotificationStatus represents the delivery state of a notification.
type NotificationStatus string

const (
	StatusPending   NotificationStatus = "pending"
	StatusSent      NotificationStatus = "sent"
	StatusDelivered NotificationStatus = "delivered"
	StatusFailed    NotificationStatus = "failed"
	StatusRetrying  NotificationStatus = "retrying"
	StatusCanceled  NotificationStatus = "canceled"
)

// ChannelType represents the delivery channel.
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

// Notification represents a single notification document in MongoDB.
type Notification struct {
	ID           bson.ObjectID      `bson:"_id,omitempty" json:"id"`
	UserID       bson.ObjectID      `bson:"user_id" json:"user_id"`
	UserIDText   string             `bson:"userId,omitempty" json:"userId,omitempty"`
	Type         string             `bson:"type,omitempty" json:"type,omitempty"`
	Channel      ChannelType        `bson:"channel" json:"channel"`
	Recipient    string             `bson:"recipient" json:"recipient"` // email, phone, chat_id, webhook_url
	Title        string             `bson:"title,omitempty" json:"title,omitempty"`
	Subject      string             `bson:"subject,omitempty" json:"subject,omitempty"`
	Message      string             `bson:"message,omitempty" json:"message,omitempty"`
	Body         string             `bson:"body" json:"body"`
	HTMLBody     string             `bson:"html_body,omitempty" json:"html_body,omitempty"`
	Template     string             `bson:"template,omitempty" json:"template,omitempty"`
	Data         map[string]string  `bson:"data,omitempty" json:"data,omitempty"`
	Metadata     map[string]any     `bson:"metadata,omitempty" json:"metadata,omitempty"`
	Status       NotificationStatus `bson:"status" json:"status"`
	Priority     string             `bson:"priority,omitempty" json:"priority,omitempty"`
	RetryCount   int                `bson:"retry_count" json:"retry_count"`
	MaxRetries   int                `bson:"max_retries" json:"max_retries"`
	Provider     string             `bson:"provider,omitempty" json:"provider,omitempty"`
	ErrorMsg     string             `bson:"error_msg,omitempty" json:"error_msg,omitempty"`
	Read         bool               `bson:"read" json:"read"`         // marked as read by user
	Digested     bool               `bson:"digested" json:"digested"` // included in a digest
	DigestID     string             `bson:"digest_id,omitempty" json:"digest_id,omitempty"`
	ReadAt       *time.Time         `bson:"readAt,omitempty" json:"readAt,omitempty"`
	LegacyReadAt *time.Time         `bson:"read_at,omitempty" json:"read_at,omitempty"`
	ActionURL    string             `bson:"actionUrl,omitempty" json:"actionUrl,omitempty"`
	GroupKey     string             `bson:"groupKey,omitempty" json:"groupKey,omitempty"`
	Source       string             `bson:"source,omitempty" json:"source,omitempty"`
	ScheduledAt  *time.Time         `bson:"scheduled_at,omitempty" json:"scheduled_at,omitempty"`
	SentAt       *time.Time         `bson:"sent_at,omitempty" json:"sent_at,omitempty"`
	DeliveredAt  *time.Time         `bson:"delivered_at,omitempty" json:"delivered_at,omitempty"`
	ExpiresAt    *time.Time         `bson:"expires_at,omitempty" json:"expires_at,omitempty"`
	CreatedAt    time.Time          `bson:"created_at" json:"createdAt"`
	UpdatedAt    time.Time          `bson:"updated_at" json:"updatedAt"`
}

// NotificationPreference represents per-user channel preferences.
type NotificationPreference struct {
	ID         bson.ObjectID   `bson:"_id,omitempty" json:"id"`
	UserID     bson.ObjectID   `bson:"user_id" json:"user_id"`
	Email      PreferenceValue `bson:"email" json:"email"`
	Discord    PreferenceValue `bson:"discord" json:"discord"`
	Telegram   PreferenceValue `bson:"telegram" json:"telegram"`
	WhatsApp   PreferenceValue `bson:"whatsapp" json:"whatsapp"`
	Slack      PreferenceValue `bson:"slack" json:"slack"`
	Webhooks   []WebhookPref   `bson:"webhooks,omitempty" json:"webhooks,omitempty"`
	DigestFreq DigestFrequency `bson:"digest_freq" json:"digest_freq"`
	DigestTime string          `bson:"digest_time" json:"digest_time"` // HH:MM in user's timezone
	Language   string          `bson:"language" json:"language"`       // "vi" | "en"
	CreatedAt  time.Time       `bson:"created_at" json:"createdAt"`
	UpdatedAt  time.Time       `bson:"updated_at" json:"updatedAt"`
}

// PreferenceValue controls notification opt-in for a channel.
type PreferenceValue string

const (
	PrefEnabled  PreferenceValue = "enabled"
	PrefDisabled PreferenceValue = "disabled"
	PrefDigest   PreferenceValue = "digest" // only via digest, not immediate
)

// DigestFrequency controls digest batching cadence.
type DigestFrequency string

const (
	DigestNone    DigestFrequency = "none"
	DigestDaily   DigestFrequency = "daily"
	DigestWeekly  DigestFrequency = "weekly"
	DigestMonthly DigestFrequency = "monthly"
)

// WebhookPref holds a named webhook endpoint for a channel.
type WebhookPref struct {
	Name    string `bson:"name" json:"name"`
	URL     string `bson:"url" json:"url"`
	Enabled bool   `bson:"enabled" json:"enabled"`
}

// NotificationPreferenceCollection is the MongoDB collection name.
const NotificationPreferenceCollection = "notification_preferences"

// Digest represents a batched digest document.
type Digest struct {
	ID        bson.ObjectID      `bson:"_id,omitempty" json:"id"`
	UserID    bson.ObjectID      `bson:"user_id" json:"user_id"`
	Channel   ChannelType        `bson:"channel" json:"channel"`
	Recipient string             `bson:"recipient" json:"recipient"`
	Frequency DigestFrequency    `bson:"frequency" json:"frequency"`
	Subject   string             `bson:"subject" json:"subject"`
	Items     []string           `bson:"items" json:"items"`
	Count     int                `bson:"count" json:"count"`
	Status    NotificationStatus `bson:"status" json:"status"`
	SentAt    *time.Time         `bson:"sent_at,omitempty" json:"sent_at,omitempty"`
	CreatedAt time.Time          `bson:"created_at" json:"createdAt"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updatedAt"`
}

// DigestCollection is the MongoDB collection name.
const DigestCollection = "notification_digests"

// DeliveryLog records each delivery attempt for auditing.
type DeliveryLog struct {
	ID             bson.ObjectID `bson:"_id,omitempty" json:"id"`
	NotificationID bson.ObjectID `bson:"notification_id" json:"notification_id"`
	Provider       string        `bson:"provider" json:"provider"`
	Attempt        int           `bson:"attempt" json:"attempt"`
	Status         string        `bson:"status" json:"status"` // "success" | "failure"
	HTTPStatus     int           `bson:"http_status,omitempty" json:"http_status,omitempty"`
	ResponseBody   string        `bson:"response_body,omitempty" json:"response_body,omitempty"`
	ErrorMsg       string        `bson:"error_msg,omitempty" json:"error_msg,omitempty"`
	DurationMS     int64         `bson:"duration_ms" json:"duration_ms"`
	CreatedAt      time.Time     `bson:"created_at" json:"createdAt"`
}

// DeliveryLogCollection is the MongoDB collection name.
const DeliveryLogCollection = "notification_delivery_logs"

// ─── BSON serialization helpers ──────────────────────────────────────────────

// ToBSON converts Notification to a bson.M document.
func (n *Notification) ToBSON() bson.M {
	return bson.M{
		"_id":          n.ID,
		"user_id":      n.UserID,
		"channel":      n.Channel,
		"recipient":    n.Recipient,
		"subject":      n.Subject,
		"body":         n.Body,
		"html_body":    n.HTMLBody,
		"template":     n.Template,
		"data":         n.Data,
		"metadata":     n.Metadata,
		"status":       n.Status,
		"retry_count":  n.RetryCount,
		"max_retries":  n.MaxRetries,
		"provider":     n.Provider,
		"error_msg":    n.ErrorMsg,
		"read":         n.Read,
		"digested":     n.Digested,
		"digest_id":    n.DigestID,
		"scheduled_at": n.ScheduledAt,
		"sent_at":      n.SentAt,
		"delivered_at": n.DeliveredAt,
		"expires_at":   n.ExpiresAt,
		"created_at":   n.CreatedAt,
		"updated_at":   n.UpdatedAt,
	}
}
