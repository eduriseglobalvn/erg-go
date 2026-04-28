package crawler

import (
	"strings"
	"time"

	"erg.ninja/internal/modules/crawler/entities"
)

type historyItemResponse struct {
	ID            string    `json:"id"`
	URL           string    `json:"url"`
	Status        string    `json:"status"`
	ErrorMessage  string    `json:"errorMessage,omitempty"`
	CrawledAt     time.Time `json:"crawledAt"`
	QualityScore  float64   `json:"qualityScore,omitempty"`
	SourceType    string    `json:"sourceType,omitempty"`
	SourceName    string    `json:"sourceName,omitempty"`
	DuplicateOf   string    `json:"duplicateOf,omitempty"`
	DedupReason   string    `json:"dedupReason,omitempty"`
	QualityLegacy float64   `json:"quality_score,omitempty"`
}

type rssSourceResponse struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	URL              string     `json:"url"`
	TargetCategoryID string     `json:"targetCategoryId,omitempty"`
	CategoryName     string     `json:"categoryName,omitempty"`
	CronExpression   string     `json:"cronExpression,omitempty"`
	IsActive         bool       `json:"isActive"`
	AutoPublish      bool       `json:"autoPublish"`
	LastRunAt        *time.Time `json:"lastRunAt,omitempty"`
	CreatedAt        time.Time  `json:"createdAt,omitempty"`
}

type activePipelineResponse struct {
	JobID        string  `json:"jobId"`
	URL          string  `json:"url"`
	Source       string  `json:"source"`
	Status       string  `json:"status"`
	CurrentStep  string  `json:"currentStep"`
	Progress     int     `json:"progress"`
	TimeStarted  string  `json:"timeStarted"`
	Message      string  `json:"message,omitempty"`
	QualityScore float64 `json:"qualityScore,omitempty"`
}

type blacklistCompatResponse struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Value     string `json:"value"`
	Reason    string `json:"reason,omitempty"`
	CreatedBy string `json:"createdBy,omitempty"`
	IsActive  bool   `json:"isActive"`
	CreatedAt string `json:"createdAt,omitempty"`
}

func newHistoryItemResponse(history *entities.CrawlHistory) historyItemResponse {
	status := strings.ToUpper(string(history.Status))
	switch history.Status {
	case entities.CrawlStatusDuplicate, entities.CrawlStatusBlacklisted, entities.CrawlStatusCanceled:
		status = "REJECTED"
	case entities.CrawlStatusSuccess:
		status = "SUCCESS"
	case entities.CrawlStatusFailed:
		status = "FAILED"
	default:
		status = "PENDING"
	}

	sourceType := "manual"
	if history.FeedID != "" {
		sourceType = "rss"
	}

	crawledAt := history.CrawledAt
	if crawledAt.IsZero() {
		crawledAt = history.CreatedAt
	}

	item := historyItemResponse{
		ID:            history.ID,
		URL:           history.URL,
		Status:        status,
		ErrorMessage:  history.ErrorMsg,
		CrawledAt:     crawledAt,
		QualityScore:  history.QualityScore,
		SourceType:    sourceType,
		QualityLegacy: history.QualityScore,
	}
	if history.Status == entities.CrawlStatusDuplicate {
		item.DuplicateOf = history.ContentHash
		item.DedupReason = history.ErrorMsg
	}
	return item
}

func newRSSSourceResponse(feed *entities.RSSFeed) rssSourceResponse {
	lastRun := feed.LastFetchAt
	if lastRun == nil {
		lastRun = feed.LastItemAt
	}
	return rssSourceResponse{
		ID:               feed.ID,
		Name:             feed.Name,
		URL:              feed.URL,
		TargetCategoryID: feed.Category,
		CronExpression:   feed.Frequency,
		IsActive:         feed.Enabled,
		AutoPublish:      false,
		LastRunAt:        lastRun,
		CreatedAt:        feed.CreatedAt,
	}
}

func newBlacklistCompatResponse(entry *entities.ContentBlacklist) blacklistCompatResponse {
	createdAt := ""
	if !entry.CreatedAt.IsZero() {
		createdAt = entry.CreatedAt.UTC().Format(time.RFC3339)
	}

	return blacklistCompatResponse{
		ID:        entry.ID,
		Type:      string(entry.Type),
		Value:     entry.Pattern,
		Reason:    entry.Reason,
		CreatedBy: entry.CreatedBy,
		IsActive:  entry.Enabled,
		CreatedAt: createdAt,
	}
}
