package ai_content

import (
	"time"
)

// ProviderType represents the AI provider (e.g., gemini, openai).
type ProviderType string

const (
	ProviderGemini ProviderType = "gemini"
	ProviderOpenAI ProviderType = "openai"
	ProviderClaude ProviderType = "claude"
)

// ApiKeyType represents the type of API key.
type ApiKeyType string

const (
	ApiKeyShared  ApiKeyType = "shared"
	ApiKeyPrivate ApiKeyType = "private"
)

// ApiKeyStatus represents the current status of the key.
type ApiKeyStatus string

const (
	ApiStatusActive        ApiKeyStatus = "active"
	ApiStatusQuotaExceeded ApiKeyStatus = "quota_exceeded"
	ApiStatusRateLimited   ApiKeyStatus = "rate_limited"
	ApiStatusError         ApiKeyStatus = "error"
)

// ApiKey represents the MongoDB entity for an API key.
type ApiKey struct {
	ID                  string       `bson:"_id"`
	Label               string       `bson:"label,omitempty"`
	ProjectID           string       `bson:"projectId,omitempty"`
	Key                 string       `bson:"key"`
	Provider            ProviderType `bson:"provider"`
	Type                ApiKeyType   `bson:"type"`
	Status              ApiKeyStatus `bson:"status"`
	OwnerID             string       `bson:"ownerId,omitempty"`
	ConsecutiveErrors   int          `bson:"consecutiveErrors"`
	Model               string       `bson:"model,omitempty"`
	MaxTokensPerRequest int          `bson:"maxTokensPerRequest"`
	DefaultTemperature  float64      `bson:"defaultTemperature"`
	CreatedAt           time.Time    `bson:"createdAt"`
	UpdatedAt           time.Time    `bson:"updatedAt"`
}

// GenerateRequest defines the payload for POST /generate
type GenerateRequest struct {
	Topic      string `json:"topic" binding:"required"`
	CategoryID string `json:"categoryId" binding:"required"`
}

// RefineRequest defines the payload for POST /refine
type RefineRequest struct {
	Content     string `json:"content"`
	Text        string `json:"text"`
	Instruction string `json:"instruction" binding:"required"`
}

// JobPayload is the payload enqueued to Asynq for background generation.
type JobPayload struct {
	Topic      string `json:"topic"`
	CategoryID string `json:"categoryId"`
	UserID     string `json:"userId"`
}
