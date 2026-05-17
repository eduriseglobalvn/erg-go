// Package commands implements all bot commands for the erg-server binary.
// Service dependencies are injected at startup via SetCrawlerService/SetTrendingService.
package commands

import (
	"context"
	"time"
)

// ─── Service interfaces ───────────────────────────────────────────────────────

// CrawlerServiceClient is the interface for the crawler module's service operations.
type CrawlerServiceClient interface {
	// EnqueueURL enqueues a URL for crawling, returns job_id.
	EnqueueURL(ctx context.Context, url, source string, priority int) (string, error)
	// GetJobStatus returns the status of a crawl job by job_id.
	GetJobStatus(ctx context.Context, jobID string) (status string, score float64, err error)
	// GetStats returns aggregated crawl statistics.
	GetStats(ctx context.Context) (totalCrawled int, passRate float64, avgScore float64, err error)
	// GetRecentHistory returns the most recent crawl history entries (up to limit).
	GetRecentHistory(ctx context.Context, limit int) ([]CrawlHistoryItem, error)
}

// CrawlHistoryItem represents a single crawl history entry for bot commands.
type CrawlHistoryItem struct {
	URL     string
	Status  string
	Score   float64
	Updated time.Time
}

// TrendingServiceClient is the interface for the trending module's service operations.
type TrendingServiceClient interface {
	// GetTopTopics returns the top N trending topics.
	GetTopTopics(ctx context.Context, limit int) ([]TrendingTopicItem, error)
	// GetTopicDetail returns details for a specific topic keyword.
	GetTopicDetail(ctx context.Context, topic string) (*TrendingTopicDetail, error)
}

// TrendingTopicItem represents a single trending topic.
type TrendingTopicItem struct {
	Topic  string
	Score  float64
	Volume int
	Source string
}

// TrendingTopicDetail represents detailed info for a trending topic.
type TrendingTopicDetail struct {
	Topic    string
	Score    float64
	Volume   int
	Source   string
	Keywords []string
}

// ─── Service handles (injected at startup) ────────────────────────────────────

// crawlerSvc holds the injected crawler service client.
var crawlerSvc CrawlerServiceClient

// trendingSvc holds the injected trending service client.
var trendingSvc TrendingServiceClient

// SetCrawlerService injects the crawler service client.
func SetCrawlerService(svc CrawlerServiceClient) {
	crawlerSvc = svc
}

// SetTrendingService injects the trending service client.
func SetTrendingService(svc TrendingServiceClient) {
	trendingSvc = svc
}

// GetCrawlerService returns the current crawler service client (nil-safe).
func GetCrawlerService() CrawlerServiceClient { return crawlerSvc }

// GetTrendingService returns the current trending service client (nil-safe).
func GetTrendingService() TrendingServiceClient { return trendingSvc }
