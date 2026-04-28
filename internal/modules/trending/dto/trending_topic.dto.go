package dto

import "time"

// TopicResponse is the API shape for a trending topic.
// It mirrors the FE 'TrendingTopic' interface for parity.
type TopicResponse struct {
	ID              string    `json:"_id"`
	Keyword         string    `json:"keyword"`
	TrendScore      float64   `json:"trendScore"`
	Source          string    `json:"source"`
	SearchVolume    int       `json:"searchVolume,omitempty"`
	Velocity        float64   `json:"velocity,omitempty"`
	DiscoveredAt    time.Time `json:"discoveredAt"`
	LastRefreshedAt time.Time `json:"lastCheckedAt"`
	Status          string    `json:"status"`
	Priority        int       `json:"priority"`
	Keywords        []string  `json:"keywords,omitempty"`
	URLs            []string  `json:"urls,omitempty"`
	Timeline        []int     `json:"timeline,omitempty"`
}

// NewsArticleResponse is the API shape for a supporting article.
type NewsArticleResponse struct {
	Topic          string    `json:"topic"`
	Headline       string    `json:"headline"`
	Source         string    `json:"source"`
	URL            string    `json:"url"`
	PublishedAt    time.Time `json:"publishedAt"`
	RelevanceScore float64   `json:"relevanceScore"`
}

// TrendingStats holds discovery statistics.
type TrendingStats struct {
	TotalTopics     int64   `json:"totalTopics"`
	ActiveTopics    int64   `json:"activeTopics"`
	AvgScore        float64 `json:"avgScore"`
	TopKeyword      string  `json:"topKeyword"`
	DiscoveredToday int64   `json:"discoveredToday"`
}

// SnapshotResponse is the API shape for historical snapshots.
type SnapshotResponse struct {
	Topics      []string  `json:"topics"`
	TopicCount  int       `json:"topicCount"`
	GeneratedAt time.Time `json:"generatedAt"`
}

// RefreshResponse reports a refresh result.
type RefreshResponse struct {
	Status      string `json:"status"`
	TopicCount  int    `json:"topicCount"`
	NewsCount   int    `json:"newsCount"`
	GeneratedAt string `json:"generatedAt"`
}
