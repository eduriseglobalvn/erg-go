// Package bot provides cross-module integration adapters for the bot module.
package bot

import (
	"context"

	"erg.ninja/internal/modules/bot/commands"
	"erg.ninja/internal/modules/crawler"
	"erg.ninja/internal/modules/trending"
)

// CrawlerAdapter wraps the crawler module's service to implement bot.commands.CrawlerServiceClient.
type CrawlerAdapter struct {
	svc *crawler.Service
}

// NewCrawlerAdapter creates a crawler adapter from the crawler module's service.
func NewCrawlerAdapter(svc *crawler.Service) *CrawlerAdapter {
	return &CrawlerAdapter{svc: svc}
}

// EnqueueURL implements the interface.
func (a *CrawlerAdapter) EnqueueURL(ctx context.Context, url, source string, priority int) (string, error) {
	return a.svc.EnqueueURL(ctx, url, source, priority)
}

// GetJobStatus implements the interface.
func (a *CrawlerAdapter) GetJobStatus(ctx context.Context, jobID string) (string, float64, error) {
	return a.svc.GetJobStatus(ctx, jobID)
}

// GetStats implements the interface.
func (a *CrawlerAdapter) GetStats(ctx context.Context) (int, float64, float64, error) {
	return a.svc.GetStats(ctx)
}

// GetRecentHistory implements the interface.
func (a *CrawlerAdapter) GetRecentHistory(ctx context.Context, limit int) ([]commands.CrawlHistoryItem, error) {
	items, err := a.svc.GetRecentHistory(ctx, limit)
	if err != nil {
		return nil, err
	}
	result := make([]commands.CrawlHistoryItem, 0, len(items))
	for _, it := range items {
		result = append(result, commands.CrawlHistoryItem{
			URL:     it.URL,
			Status:  it.Status,
			Score:   it.Score,
			Updated: it.Updated,
		})
	}
	return result, nil
}

// TrendingAdapter wraps the trending module's service to implement bot.commands.TrendingServiceClient.
type TrendingAdapter struct {
	svc *trending.Service
}

// NewTrendingAdapter creates a trending adapter from the trending module's service.
func NewTrendingAdapter(svc *trending.Service) *TrendingAdapter {
	return &TrendingAdapter{svc: svc}
}

// GetTopTopics implements the interface.
func (a *TrendingAdapter) GetTopTopics(ctx context.Context, limit int) ([]commands.TrendingTopicItem, error) {
	topics, err := a.svc.ListTopics(ctx, int64(limit))
	if err != nil {
		return nil, err
	}
	result := make([]commands.TrendingTopicItem, 0, len(topics))
	for _, t := range topics {
		result = append(result, commands.TrendingTopicItem{
			Topic:  t.Keyword,
			Score:  t.TrendScore,
			Volume: t.SearchVolume,
			Source: t.Source,
		})
	}
	return result, nil
}

// GetTopicDetail implements the interface.
func (a *TrendingAdapter) GetTopicDetail(ctx context.Context, topic string) (*commands.TrendingTopicDetail, error) {
	t, err := a.svc.GetTopic(ctx, topic)
	if err != nil || t == nil {
		return nil, err
	}
	return &commands.TrendingTopicDetail{
		Topic:    t.Keyword,
		Score:    t.TrendScore,
		Volume:   t.SearchVolume,
		Source:   t.Source,
		Keywords: t.Keywords,
	}, nil
}
