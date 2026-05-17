package dto

import (
	"time"
)

// ProviderType represents the AI provider (e.g., gemini, openai).
type ProviderType string

const (
	ProviderGemini ProviderType = "gemini"
	ProviderGroq   ProviderType = "groq"
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
	ApiStatusInactive      ApiKeyStatus = "inactive"
	ApiStatusQuotaExceeded ApiKeyStatus = "quota_exceeded"
	ApiStatusRateLimited   ApiKeyStatus = "rate_limited"
	ApiStatusError         ApiKeyStatus = "error"
)

// ApiKey represents the MongoDB entity for an API key.
type ApiKey struct {
	ID                  string       `bson:"_id"`
	Label               string       `bson:"label,omitempty"`
	ProjectID           string       `bson:"projectId,omitempty"`
	Key                 string       `bson:"key,omitempty" json:"-"`
	EncryptedKey        string       `bson:"encryptedKey,omitempty" json:"-"`
	KeyNonce            string       `bson:"keyNonce,omitempty" json:"-"`
	KeyVersion          string       `bson:"keyVersion,omitempty" json:"-"`
	KeyFingerprint      string       `bson:"keyFingerprint,omitempty" json:"-"`
	MaskedKeyPreview    string       `bson:"maskedKeyPreview,omitempty" json:"-"`
	Provider            ProviderType `bson:"provider"`
	Type                ApiKeyType   `bson:"type"`
	Status              ApiKeyStatus `bson:"status"`
	OwnerID             string       `bson:"ownerId,omitempty"`
	Selected            bool         `bson:"selected"`
	ConsecutiveErrors   int          `bson:"consecutiveErrors"`
	Model               string       `bson:"model,omitempty"`
	MaxTokensPerRequest int          `bson:"maxTokensPerRequest"`
	DefaultTemperature  float64      `bson:"defaultTemperature"`
	TodayUsage          int          `bson:"todayUsage"`
	MaxDailyQuota       int          `bson:"maxDailyQuota"`
	UsageCount          int64        `bson:"usageCount"`
	LastUsedAt          *time.Time   `bson:"lastUsedAt,omitempty"`
	LastTestedAt        *time.Time   `bson:"lastTestedAt,omitempty"`
	CooldownUntil       *time.Time   `bson:"cooldownUntil,omitempty"`
	LastErrorMessage    string       `bson:"lastErrorMessage,omitempty"`
	CreatedAt           time.Time    `bson:"createdAt"`
	UpdatedAt           time.Time    `bson:"updatedAt"`
}

// CreateAPIKeyRequest is the payload for creating an AI provider key.
type CreateAPIKeyRequest struct {
	Key                 string       `json:"key" binding:"required"`
	Label               string       `json:"label"`
	ProjectID           string       `json:"projectId"`
	Provider            ProviderType `json:"provider" binding:"required"`
	Type                ApiKeyType   `json:"type"`
	Model               string       `json:"model"`
	MaxTokensPerRequest int          `json:"maxTokensPerRequest"`
	DefaultTemperature  float64      `json:"defaultTemperature"`
	MaxDailyQuota       int          `json:"maxDailyQuota"`
	Selected            bool         `json:"selected"`
}

// APIKeyResponse is the safe API shape returned to the admin UI.
type APIKeyResponse struct {
	ID                  string       `json:"id"`
	Label               string       `json:"label"`
	ProjectID           string       `json:"projectId"`
	MaskedKey           string       `json:"maskedKey"`
	Key                 string       `json:"key"`
	Provider            ProviderType `json:"provider"`
	Type                ApiKeyType   `json:"type"`
	Status              ApiKeyStatus `json:"status"`
	Selected            bool         `json:"selected"`
	Model               string       `json:"model"`
	MaxTokensPerRequest int          `json:"maxTokensPerRequest"`
	DefaultTemperature  float64      `json:"defaultTemperature"`
	TodayUsage          int          `json:"todayUsage"`
	MaxDailyQuota       int          `json:"maxDailyQuota"`
	UsageCount          int64        `json:"usageCount"`
	LastUsedAt          *time.Time   `json:"lastUsedAt"`
	LastTestedAt        *time.Time   `json:"lastTestedAt"`
	CooldownUntil       *time.Time   `json:"cooldownUntil"`
	LastErrorMessage    string       `json:"lastErrorMessage"`
	CreatedAt           time.Time    `json:"createdAt"`
	UpdatedAt           time.Time    `json:"updatedAt"`
}

type APIKeysDashboard struct {
	TotalKeys    int             `json:"total_keys"`
	ActiveKeys   int             `json:"active_keys"`
	SelectedKey  *APIKeyResponse `json:"selected_key"`
	TotalUsage   int64           `json:"total_usage"`
	MonthlyUsage int64           `json:"monthly_usage"`
	ByProvider   map[string]int  `json:"by_provider"`
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
