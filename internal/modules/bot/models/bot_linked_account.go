package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// BotLinkedAccount links a platform user identity (Discord/Telegram) to an
// internal ERG user account. The link is established via a 6-char code with a
// 5-minute TTL stored in Redis; once verified it is persisted to MongoDB.
type BotLinkedAccount struct {
	ID             bson.ObjectID     `bson:"_id,omitempty"`
	Platform       string            `bson:"platform"`         // "discord", "telegram"
	PlatformUserID string            `bson:"platform_user_id"` // Discord snowflake / Telegram user ID
	InternalUserID string            `bson:"internal_user_id"` // ERG internal user ID
	DisplayName    string            `bson:"display_name,omitempty"`
	Username       string            `bson:"username,omitempty"` // Discord/Telegram username
	AvatarURL      string            `bson:"avatar_url,omitempty"`
	LinkCode       string            `bson:"link_code,omitempty"` // the 6-char code (only set during pending)
	VerifiedAt     time.Time         `bson:"verified_at,omitempty"`
	LinkedAt       time.Time         `bson:"linked_at"`
	UnlinkedAt     *time.Time        `bson:"unlinked_at,omitempty"` // soft-delete
	Metadata       map[string]string `bson:"metadata,omitempty"`
}

// IsVerified returns true if the account link has been verified.
func (a *BotLinkedAccount) IsVerified() bool {
	return !a.VerifiedAt.IsZero()
}

// CollectionName returns the MongoDB collection name for BotLinkedAccount.
const BotLinkedAccountCollection = "bot_linked_accounts"
