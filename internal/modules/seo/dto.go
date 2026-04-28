package seo

import (
	"encoding/json"
	"time"
)

// ─── Entities ────────────────────────────────────────────────────────────────

// SeoKeyword represents a tracked SEO keyword.
type SeoKeyword struct {
	ID        string    `bson:"_id,omitempty"          json:"id"`
	Keyword   string    `bson:"keyword"               json:"keyword"`
	TargetURL string    `bson:"target_url"           json:"targetUrl"`
	LinkLimit int       `bson:"link_limit"           json:"linkLimit"`
	IsActive  bool      `bson:"is_active"             json:"isActive"`
	CreatedAt time.Time `bson:"created_at"            json:"createdAt"`
	UpdatedAt time.Time `bson:"updated_at,omitempty" json:"updatedAt,omitempty"`
}

// SeoRedirect represents a URL redirect rule.
type SeoRedirect struct {
	ID          string    `bson:"_id,omitempty"          json:"id"`
	FromPattern string    `bson:"from_pattern"           json:"fromPattern"`
	ToURL       string    `bson:"to_url"                json:"toUrl"`
	Type        string    `bson:"type"                  json:"type"`
	IsRegex     bool      `bson:"is_regex"              json:"isRegex"`
	IsActive    bool      `bson:"is_active"             json:"isActive"`
	HitCount    int       `bson:"hit_count"            json:"hitCount"`
	CreatedAt   time.Time `bson:"created_at"            json:"createdAt"`
	UpdatedAt   time.Time `bson:"updated_at,omitempty" json:"updatedAt,omitempty"`
}

// Seo404Log represents a 404 error log entry.
type Seo404Log struct {
	ID        string    `bson:"_id,omitempty"     json:"id"`
	URL       string    `bson:"url"              json:"url"`
	Referrer  string    `bson:"referrer,omitempty" json:"referrer,omitempty"`
	UserAgent string    `bson:"useragent,omitempty" json:"userAgent,omitempty"`
	HitCount  int       `bson:"hit_count"        json:"hitCount"`
	LastSeen  time.Time `bson:"last_seen"         json:"lastSeen"`
	FirstSeen time.Time `bson:"first_seen"        json:"firstSeen"`
}

// SeoConfig stores a key-value configuration for SEO.
type SeoConfig struct {
	Key       string          `bson:"_id,omitempty" json:"key"`
	Value     json.RawMessage `bson:"value"          json:"value"`
	UpdatedBy string          `bson:"updated_by,omitempty" json:"updatedBy,omitempty"`
	UpdatedAt time.Time       `bson:"updated_at,omitempty"  json:"updatedAt,omitempty"`
}

// GSCData represents a Google Search Console data point.
type GSCData struct {
	ID          string    `bson:"_id,omitempty"   json:"id"`
	PostID      string    `bson:"post_id"        json:"postId"`
	URL         string    `bson:"url"            json:"url"`
	Clicks      int       `bson:"clicks"         json:"clicks"`
	Impressions int       `bson:"impressions"    json:"impressions"`
	CTR         float64   `bson:"ctr"            json:"ctr"`
	Position    float64   `bson:"position"       json:"position"`
	Date        time.Time `bson:"date"           json:"date"`
}

// ─── Request DTOs ────────────────────────────────────────────────────────────

type CreateKeywordRequest struct {
	Keyword   string `json:"keyword"`
	TargetURL string `json:"targetUrl"`
	LinkLimit int    `json:"linkLimit"`
}

type CreateRedirectRequest struct {
	FromPattern string `json:"fromPattern"`
	ToURL       string `json:"toUrl"`
	Type        string `json:"type"`
	IsRegex     bool   `json:"isRegex"`
}

type UpdateRedirectRequest struct {
	FromPattern string `json:"fromPattern,omitempty"`
	ToURL       string `json:"toUrl,omitempty"`
	Type        string `json:"type,omitempty"`
	IsActive    *bool  `json:"isActive,omitempty"`
}

type Log404Request struct {
	URL       string `json:"url"`
	Referrer  string `json:"referrer,omitempty"`
	UserAgent string `json:"userAgent,omitempty"`
}

type SaveSchemaRequest struct {
	Type SchemaType `json:"type"`
	Data any        `json:"data"`
}

// ─── Response DTOs ───────────────────────────────────────────────────────────

type SchemaResponse struct {
	Type SchemaType      `json:"type"`
	Data json.RawMessage `json:"data"`
}

type Seo404LogsResponse struct {
	Items      []*Seo404Log `json:"items"`
	Total      int64        `json:"total"`
	Page       int          `json:"page"`
	Limit      int          `json:"limit"`
	TotalPages int64        `json:"totalPages"`
}

type PerformanceResponse struct {
	Period              string  `json:"period"`
	TotalClicks         int64   `json:"totalClicks"`
	TotalImpressions    int64   `json:"totalImpressions"`
	AverageCTR          float64 `json:"averageCtr"`
	AveragePosition     float64 `json:"averagePosition"`
	TotalImpressionsStr string  `json:"totalImpressionsStr"`
}
