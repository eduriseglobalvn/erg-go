// Package models defines the domain models for the bot-service.
package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// ConversationState represents the state of a bot conversation.
type ConversationState string

const (
	StateActive    ConversationState = "active"
	StatePending   ConversationState = "pending" // wizard đang chờ input
	StateCompleted ConversationState = "completed"
	StateExpired   ConversationState = "expired"
)

// BotConversation represents a conversation session between a user and the bot
// on a specific platform (Discord, Telegram, etc.). It persists across restarts
// via MongoDB with a 30-day TTL on updated_at.
type BotConversation struct {
	ID              bson.ObjectID     `bson:"_id,omitempty"`
	UserID          string            `bson:"user_id"`
	Platform        string            `bson:"platform"`         // "discord", "telegram"
	PlatformConvID  string            `bson:"platform_conv_id"` // discord channel_id / telegram chat_id
	State           ConversationState `bson:"state"`
	WizardStep      string            `bson:"wizard_step,omitempty"` // current wizard step name
	WizardData      map[string]string `bson:"wizard_data,omitempty"` // accumulated wizard inputs
	Context         map[string]any    `bson:"context,omitempty"`     // arbitrary session context
	LastMessageID   string            `bson:"last_message_id,omitempty"`
	LastCommand     string            `bson:"last_command,omitempty"`
	PermissionLevel int               `bson:"permission_level"` // RBAC level (1-5)
	Metadata        map[string]string `bson:"metadata,omitempty"`
	CreatedAt       time.Time         `bson:"created_at"`
	UpdatedAt       time.Time         `bson:"updated_at"`
	ExpiresAt       time.Time         `bson:"expires_at,omitempty"` // TTL: auto-delete after 30 days of inactivity
}

// ShouldExpire returns true if the conversation has expired based on UpdatedAt.
func (c *BotConversation) ShouldExpire() bool {
	if c.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(c.ExpiresAt)
}

// IsActive returns true if the conversation is in an active or pending state.
func (c *BotConversation) IsActive() bool {
	return c.State == StateActive || c.State == StatePending
}

// CollectionName returns the MongoDB collection name for BotConversation.
const BotConversationCollection = "bot_conversations"
