// Package entities defines MongoDB document models for the crawler module.
package entities

import (
	"time"
)

// ─── RSS Feed ────────────────────────────────────────────────────────────────

// RSSFeed represents a monitored RSS/Atom/JSON feed.
type RSSFeed struct {
	ID           string     `bson:"_id,omitempty" json:"id"`
	Name         string     `bson:"name" json:"name"`
	URL          string     `bson:"url" json:"url"`
	Type         string     `bson:"type" json:"type"` // "rss" | "atom" | "json"
	Category     string     `bson:"category,omitempty" json:"category,omitempty"`
	Language     string     `bson:"language,omitempty" json:"language,omitempty"`   // "vi" | "en"
	Frequency    string     `bson:"frequency,omitempty" json:"frequency,omitempty"` // "5m" | "15m" | "1h" | "6h" | "daily"
	Enabled      bool       `bson:"enabled" json:"enabled"`
	LastFetchAt  *time.Time `bson:"last_fetch_at,omitempty" json:"last_fetch_at,omitempty"`
	LastItemAt   *time.Time `bson:"last_item_at,omitempty" json:"last_item_at,omitempty"`
	ItemCount    int        `bson:"item_count" json:"item_count"`
	ErrorCount   int        `bson:"error_count" json:"error_count"`
	LastError    string     `bson:"last_error,omitempty" json:"last_error,omitempty"`
	ETag         string     `bson:"etag,omitempty" json:"etag,omitempty"`
	LastModified string     `bson:"last_modified,omitempty" json:"last_modified,omitempty"`
	CreatedAt    time.Time  `bson:"created_at" json:"createdAt"`
	UpdatedAt    time.Time  `bson:"updated_at" json:"updatedAt"`
}

// RSSFeedCollection is the MongoDB collection name.
const RSSFeedCollection = "rss_feeds"

// ─── Crawl History ───────────────────────────────────────────────────────────

// CrawlStatus represents the status of a crawl job.
type CrawlStatus string

const (
	CrawlStatusPending     CrawlStatus = "pending"
	CrawlStatusRunning     CrawlStatus = "running"
	CrawlStatusSuccess     CrawlStatus = "success"
	CrawlStatusFailed      CrawlStatus = "failed"
	CrawlStatusCanceled    CrawlStatus = "canceled"
	CrawlStatusDuplicate   CrawlStatus = "duplicate"
	CrawlStatusBlacklisted CrawlStatus = "blacklisted"
)

// CrawlHistory records metadata for each URL crawl operation.
type CrawlHistory struct {
	ID           string      `bson:"_id,omitempty" json:"id"`
	URL          string      `bson:"url" json:"url"`
	FeedID       string      `bson:"feed_id,omitempty" json:"feed_id,omitempty"`
	JobID        string      `bson:"job_id,omitempty" json:"job_id,omitempty"`
	Status       CrawlStatus `bson:"status" json:"status"`
	HTTPStatus   int         `bson:"http_status,omitempty" json:"http_status,omitempty"`
	ResponseSize int64       `bson:"response_size,omitempty" json:"response_size,omitempty"`
	DurationMS   int64       `bson:"duration_ms,omitempty" json:"duration_ms,omitempty"`
	ErrorCode    string      `bson:"error_code,omitempty" json:"error_code,omitempty"`
	ErrorMsg     string      `bson:"error_msg,omitempty" json:"error_msg,omitempty"`
	QualityScore float64     `bson:"quality_score,omitempty" json:"quality_score,omitempty"`
	Step         int         `bson:"step,omitempty" json:"step,omitempty"` // pipeline step reached
	Title        string      `bson:"title,omitempty" json:"title,omitempty"`
	Description  string      `bson:"description,omitempty" json:"description,omitempty"`
	ContentHash  string      `bson:"content_hash,omitempty" json:"content_hash,omitempty"`
	Tags         []string    `bson:"tags,omitempty" json:"tags,omitempty"`
	Language     string      `bson:"language,omitempty" json:"language,omitempty"`
	CrawledAt    time.Time   `bson:"crawled_at" json:"crawled_at"`
	CreatedAt    time.Time   `bson:"created_at" json:"createdAt"`
	UpdatedAt    time.Time   `bson:"updated_at" json:"updatedAt"`
}

// CrawlHistoryCollection is the MongoDB collection name.
const CrawlHistoryCollection = "crawl_histories"

// ─── Content Fingerprint ─────────────────────────────────────────────────────

// ContentFingerprint stores SimHash + SHA-256 for deduplication.
type ContentFingerprint struct {
	ID        string    `bson:"_id,omitempty" json:"id"`
	URL       string    `bson:"url" json:"url"`
	SimHash   uint64    `bson:"simhash" json:"simhash"`
	SHA256    string    `bson:"sha256" json:"sha256"` // hex string
	Bucket    uint16    `bson:"bucket" json:"bucket"` // top 16 bits of SimHash
	CrawlID   string    `bson:"crawl_id,omitempty" json:"crawl_id,omitempty"`
	CrawledAt time.Time `bson:"crawled_at" json:"crawled_at"`
}

// ContentFingerprintCollection is the MongoDB collection name.
const ContentFingerprintCollection = "content_fingerprints"

// ─── Content Blacklist ───────────────────────────────────────────────────────

// BlacklistType identifies what is being blocked.
type BlacklistType string

const (
	BlacklistURL     BlacklistType = "url"
	BlacklistDomain  BlacklistType = "domain"
	BlacklistKeyword BlacklistType = "keyword"
	BlacklistIP      BlacklistType = "ip"
)

// ContentBlacklist blocks URLs, domains, or keywords from crawling.
type ContentBlacklist struct {
	ID           string        `bson:"_id,omitempty" json:"id"`
	Type         BlacklistType `bson:"type" json:"type"`
	Pattern      string        `bson:"pattern" json:"pattern"`
	Reason       string        `bson:"reason,omitempty" json:"reason,omitempty"`
	BlockedCount int           `bson:"blocked_count" json:"blocked_count"`
	Enabled      bool          `bson:"enabled" json:"enabled"`
	CreatedBy    string        `bson:"created_by,omitempty" json:"created_by,omitempty"`
	CreatedAt    time.Time     `bson:"created_at" json:"createdAt"`
	UpdatedAt    time.Time     `bson:"updated_at" json:"updatedAt"`
}

// ContentBlacklistCollection is the MongoDB collection name.
const ContentBlacklistCollection = "content_blacklists"

// ─── Domain Reputation ───────────────────────────────────────────────────────

// DomainReputation tracks crawl behavior per domain.
type DomainReputation struct {
	ID           string    `bson:"_id,omitempty" json:"id"`
	Domain       string    `bson:"domain" json:"domain"`
	BlockCount   int       `bson:"block_count" json:"block_count"` // times blocked by anti-bot
	SuccessCount int       `bson:"success_count" json:"success_count"`
	FailCount    int       `bson:"fail_count" json:"fail_count"`
	AvgDuration  int64     `bson:"avg_duration_ms" json:"avg_duration_ms"`
	Score        float64   `bson:"score" json:"score"` // 0-100, computed
	LastSeenAt   time.Time `bson:"last_seen_at" json:"last_seen_at"`
	UpdatedAt    time.Time `bson:"updated_at" json:"updatedAt"`
}

// DomainReputationCollection is the MongoDB collection name.
const DomainReputationCollection = "domain_reputations"

// ─── Crawl Job (Asynq) ───────────────────────────────────────────────────────

// CrawlJobPayload is the payload for a crawl Asynq job.
type CrawlJobPayload struct {
	URL      string `json:"url"`
	FeedID   string `json:"feed_id,omitempty"`
	JobID    string `json:"job_id"`
	Priority int    `json:"priority"` // 1-10, higher = more urgent
	Depth    int    `json:"depth"`    // crawl depth for spidering
	ConfigID string `json:"config_id,omitempty"`
	UserID   string `json:"user_id,omitempty"`
}

// RefreshFeedPayload is the payload for a feed refresh Asynq job.
type RefreshFeedPayload struct {
	FeedID string `json:"feed_id"`
	Force  bool   `json:"force"` // force refresh even if not due
}

// ReindexPayload is the payload for a reindex Asynq job.
type ReindexPayload struct {
	Algorithm string `json:"algorithm"` // "simhash" | "all"
	Since     string `json:"since"`     // ISO timestamp, reindex entries since
	BatchSize int    `json:"batch_size"`
}

// ─── SSE Event types ─────────────────────────────────────────────────────────

// CrawlProgressEvent is streamed via SSE during a crawl job.
type CrawlProgressEvent struct {
	JobID   string `json:"job_id"`
	Step    int    `json:"step"`
	Total   int    `json:"total_steps"`
	Message string `json:"message"`
	Done    bool   `json:"done"`
	Error   string `json:"error,omitempty"`
}

// JobStatusResponse is the API response for job status.
type JobStatusResponse struct {
	JobID   string      `json:"job_id"`
	URL     string      `json:"url"`
	Status  CrawlStatus `json:"status"`
	Step    int         `json:"step"`
	Message string      `json:"message,omitempty"`
	Error   string      `json:"error,omitempty"`
}
