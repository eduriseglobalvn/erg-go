// Package dto defines request/response types for the crawler REST API.
package dto

import (
	"time"

	entities "erg.ninja/internal/modules/crawler/domain/entity"
)

// ─── Crawl ───────────────────────────────────────────────────────────────────

// CrawlURLRequest is the POST /api/crawler/crawl payload.
type CrawlURLRequest struct {
	URL      string `json:"url" validate:"required,url"`
	FeedID   string `json:"feed_id,omitempty"`
	Priority int    `json:"priority"` // 1-10
	Depth    int    `json:"depth"`
	ConfigID string `json:"config_id,omitempty"`
}

// CrawlResponse is the response after enqueueing a crawl job.
type CrawlResponse struct {
	JobID   string `json:"job_id"`
	URL     string `json:"url"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// ─── RSS Feeds ─────────────────────────────────────────────────────────────

// CreateFeedRequest is the POST /api/rss/feeds payload.
type CreateFeedRequest struct {
	Name      string `json:"name" validate:"required"`
	URL       string `json:"url" validate:"required,url"`
	Type      string `json:"type"` // "rss" | "atom" | "json" | "auto"
	Category  string `json:"category,omitempty"`
	Language  string `json:"language,omitempty"`  // "vi" | "en"
	Frequency string `json:"frequency,omitempty"` // "5m" | "15m" | "1h" | "6h" | "daily"
}

// UpdateFeedRequest is the PUT /api/rss/feeds/:id payload.
type UpdateFeedRequest struct {
	Name      string `json:"name,omitempty"`
	Category  string `json:"category,omitempty"`
	Language  string `json:"language,omitempty"`
	Frequency string `json:"frequency,omitempty"`
	Enabled   *bool  `json:"enabled,omitempty"`
}

// FeedResponse is the API response for a feed.
type FeedResponse struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	URL         string     `json:"url"`
	Type        string     `json:"type"`
	Category    string     `json:"category,omitempty"`
	Language    string     `json:"language,omitempty"`
	Frequency   string     `json:"frequency,omitempty"`
	Enabled     bool       `json:"enabled"`
	LastFetchAt *time.Time `json:"last_fetch_at,omitempty"`
	LastItemAt  *time.Time `json:"last_item_at,omitempty"`
	ItemCount   int        `json:"item_count"`
	ErrorCount  int        `json:"error_count"`
	LastError   string     `json:"last_error,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

// ─── Blacklist ─────────────────────────────────────────────────────────────

// CreateBlacklistRequest is the POST /api/blacklist payload.
type CreateBlacklistRequest struct {
	Type    string `json:"type" validate:"required,oneof=url domain keyword ip"`
	Pattern string `json:"pattern" validate:"required"`
	Reason  string `json:"reason,omitempty"`
}

// BlacklistResponse is the API response for a blacklist entry.
type BlacklistResponse struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`
	Pattern      string    `json:"pattern"`
	Reason       string    `json:"reason,omitempty"`
	BlockedCount int       `json:"blocked_count"`
	Enabled      bool      `json:"enabled"`
	CreatedBy    string    `json:"created_by,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}

// ─── Crawl History ───────────────────────────────────────────────────────────

// CrawlHistoryResponse is the API response for crawl history.
type CrawlHistoryResponse struct {
	ID           string    `json:"id"`
	URL          string    `json:"url"`
	FeedID       string    `json:"feed_id,omitempty"`
	JobID        string    `json:"job_id,omitempty"`
	Status       string    `json:"status"`
	HTTPStatus   int       `json:"http_status,omitempty"`
	DurationMS   int64     `json:"duration_ms,omitempty"`
	QualityScore float64   `json:"quality_score,omitempty"`
	ErrorMsg     string    `json:"error_msg,omitempty"`
	Title        string    `json:"title,omitempty"`
	Description  string    `json:"description,omitempty"`
	Tags         []string  `json:"tags,omitempty"`
	Language     string    `json:"language,omitempty"`
	CrawledAt    time.Time `json:"crawled_at,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}

// CrawlHistoryListResponse is the paginated list response.
type CrawlHistoryListResponse struct {
	Items  []CrawlHistoryResponse `json:"items"`
	Total  int64                  `json:"total"`
	Limit  int64                  `json:"limit"`
	Offset int64                  `json:"offset"`
}

// ─── Domain Reputation ──────────────────────────────────────────────────────

// DomainReputationResponse is the API response for domain reputation.
type DomainReputationResponse struct {
	Domain        string    `json:"domain"`
	BlockCount    int       `json:"block_count"`
	SuccessCount  int       `json:"success_count"`
	FailCount     int       `json:"fail_count"`
	Score         float64   `json:"score"`
	AvgDurationMS int64     `json:"avg_duration_ms"`
	LastSeenAt    time.Time `json:"last_seen_at"`
}

// ─── SSE ──────────────────────────────────────────────────────────────────────

// SSEProgressEvent mirrors CrawlProgressEvent for JSON encoding.
type SSEProgressEvent struct {
	Type    string `json:"type"` // "progress" | "done" | "error"
	JobID   string `json:"job_id"`
	Step    int    `json:"step,omitempty"`
	Message string `json:"message,omitempty"`
}

// ─── Mappers ────────────────────────────────────────────────────────────────

// FeedToResponse converts an RSSFeed entity to a response DTO.
func FeedToResponse(f *entities.RSSFeed) FeedResponse {
	return FeedResponse{
		ID:          f.ID,
		Name:        f.Name,
		URL:         f.URL,
		Type:        f.Type,
		Category:    f.Category,
		Language:    f.Language,
		Frequency:   f.Frequency,
		Enabled:     f.Enabled,
		LastFetchAt: f.LastFetchAt,
		LastItemAt:  f.LastItemAt,
		ItemCount:   f.ItemCount,
		ErrorCount:  f.ErrorCount,
		LastError:   f.LastError,
		CreatedAt:   f.CreatedAt,
		UpdatedAt:   f.UpdatedAt,
	}
}

// FeedsToResponses converts a slice of feeds to response DTOs.
func FeedsToResponses(ff []*entities.RSSFeed) []FeedResponse {
	out := make([]FeedResponse, len(ff))
	for i, f := range ff {
		out[i] = FeedToResponse(f)
	}
	return out
}

// BlacklistToResponse converts a blacklist entry to a response DTO.
func BlacklistToResponse(b *entities.ContentBlacklist) BlacklistResponse {
	return BlacklistResponse{
		ID:           b.ID,
		Type:         string(b.Type),
		Pattern:      b.Pattern,
		Reason:       b.Reason,
		BlockedCount: b.BlockedCount,
		Enabled:      b.Enabled,
		CreatedBy:    b.CreatedBy,
		CreatedAt:    b.CreatedAt,
	}
}

// BlacklistsToResponses converts a slice of blacklist entries.
func BlacklistsToResponses(bb []*entities.ContentBlacklist) []BlacklistResponse {
	out := make([]BlacklistResponse, len(bb))
	for i, b := range bb {
		out[i] = BlacklistToResponse(b)
	}
	return out
}

// CrawlHistoryToResponse converts a crawl history entity to a response DTO.
func CrawlHistoryToResponse(h *entities.CrawlHistory) CrawlHistoryResponse {
	return CrawlHistoryResponse{
		ID:           h.ID,
		URL:          h.URL,
		FeedID:       h.FeedID,
		JobID:        h.JobID,
		Status:       string(h.Status),
		HTTPStatus:   h.HTTPStatus,
		DurationMS:   h.DurationMS,
		QualityScore: h.QualityScore,
		ErrorMsg:     h.ErrorMsg,
		Title:        h.Title,
		Description:  h.Description,
		Tags:         h.Tags,
		Language:     h.Language,
		CrawledAt:    h.CrawledAt,
		CreatedAt:    h.CreatedAt,
	}
}

// CrawlHistoriesToResponses converts a slice of crawl histories.
func CrawlHistoriesToResponses(hh []*entities.CrawlHistory) []CrawlHistoryResponse {
	out := make([]CrawlHistoryResponse, len(hh))
	for i, h := range hh {
		out[i] = CrawlHistoryToResponse(h)
	}
	return out
}

// DomainRepToResponse converts a domain reputation entity to a response DTO.
func DomainRepToResponse(d *entities.DomainReputation) DomainReputationResponse {
	return DomainReputationResponse{
		Domain:        d.Domain,
		BlockCount:    d.BlockCount,
		SuccessCount:  d.SuccessCount,
		FailCount:     d.FailCount,
		Score:         d.Score,
		AvgDurationMS: d.AvgDuration,
		LastSeenAt:    d.LastSeenAt,
	}
}
