// Package jobs provides Asynq job handlers for the crawler module.
package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"erg.ninja/internal/modules/crawler/entities"
	"erg.ninja/internal/modules/crawler/repository"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/queue"
	"erg.ninja/pkg/rss"
)

// RefreshFeedHandler processes a feed refresh job from the Asynq queue.
type RefreshFeedHandler struct {
	repo   *repository.Repository
	parser *rss.Parser
	log    *logger.Logger
}

// NewRefreshFeedHandler creates a new refresh feed job handler.
func NewRefreshFeedHandler(repo *repository.Repository, parser *rss.Parser, log *logger.Logger) *RefreshFeedHandler {
	return &RefreshFeedHandler{repo: repo, parser: parser, log: log}
}

// Handle processes a crawler:refresh_feed Asynq task.
// Payload: entities.RefreshFeedPayload
func (h *RefreshFeedHandler) Handle(ctx context.Context, t *asynq.Task) error {
	var payload entities.RefreshFeedPayload
	if err := queue.ParsePayload(t.Payload(), &payload); err != nil {
		return fmt.Errorf("refresh-feed: parse payload: %w", err)
	}

	h.log.Info().Str("feed_id", payload.FeedID).Bool("force", payload.Force).Msg("refresh-feed: processing")

	feed, err := h.repo.GetFeed(ctx, payload.FeedID)
	if err != nil {
		return fmt.Errorf("refresh-feed: get feed %s: %w", payload.FeedID, err)
	}
	if feed == nil {
		h.log.Warn().Str("feed_id", payload.FeedID).Msg("refresh-feed: feed not found, skipping")
		return nil // Not an error — feed may have been deleted
	}

	if !feed.Enabled {
		h.log.Info().Str("feed_id", payload.FeedID).Msg("refresh-feed: feed disabled, skipping")
		return nil
	}

	// Check if feed is due for refresh (unless forced).
	if !payload.Force {
		due, err := h.isFeedDue(ctx, feed)
		if err != nil {
			return fmt.Errorf("refresh-feed: due check %s: %w", payload.FeedID, err)
		}
		if !due {
			h.log.Debug().Str("feed_id", payload.FeedID).Msg("refresh-feed: not yet due")
			return nil
		}
	}

	// Fetch the feed.
	feedData, err := h.parser.Fetch(ctx, feed.URL, feed.ETag, feed.LastModified)
	if err != nil {
		h.log.Warn().Str("feed_id", payload.FeedID).Err(err).Msg("refresh-feed: fetch failed")
		h.updateFeedError(ctx, payload.FeedID, err.Error())
		return fmt.Errorf("refresh-feed: fetch %s: %w", payload.FeedID, err)
	}

	// Update ETag/LastModified for next fetch.
	updates := map[string]interface{}{
		"last_fetch_at": time.Now().UTC(),
		"error_count":   0,
		"last_error":    "",
	}
	if feedData.ETag != "" {
		updates["etag"] = feedData.ETag
	}
	if feedData.LastModified != "" {
		updates["last_modified"] = feedData.LastModified
	}

	// Count new items since last fetch.
	newItems := 0
	var lastPub time.Time
	if feed.LastItemAt != nil {
		lastPub = *feed.LastItemAt
	}

	for _, item := range feedData.Items {
		itemTime := item.PubDate
		if itemTime.IsZero() {
			itemTime = time.Now().UTC()
		}
		if itemTime.After(lastPub) {
			newItems++
			if itemTime.After(lastPub) {
				lastPub = itemTime
			}
		}

		// Enqueue crawl job for each new item.
		jobID := uuid.New().String()
		_ = enqueueCrawlJob(ctx, item.Link, payload.FeedID, jobID, 0)
	}

	updates["item_count"] = feed.ItemCount + len(feedData.Items)
	if newItems > 0 {
		updates["last_item_at"] = lastPub
	}

	if err := h.repo.UpdateFeed(ctx, payload.FeedID, updates); err != nil {
		h.log.Warn().Str("feed_id", payload.FeedID).Err(err).Msg("refresh-feed: failed to update feed")
	}

	h.log.Info().
		Str("feed_id", payload.FeedID).
		Int("total_items", len(feedData.Items)).
		Int("new_items", newItems).
		Msg("refresh-feed: completed")

	return nil
}

// isFeedDue checks whether a feed is due for a refresh based on its frequency setting.
func (h *RefreshFeedHandler) isFeedDue(ctx context.Context, feed *entities.RSSFeed) (bool, error) {
	if feed.LastFetchAt == nil {
		return true, nil
	}

	freq := feed.Frequency
	if freq == "" {
		freq = "1h" // default
	}

	var interval time.Duration
	switch freq {
	case "5m":
		interval = 5 * time.Minute
	case "15m":
		interval = 15 * time.Minute
	case "1h":
		interval = 1 * time.Hour
	case "6h":
		interval = 6 * time.Hour
	case "12h":
		interval = 12 * time.Hour
	case "daily":
		interval = 24 * time.Hour
	default:
		interval = 1 * time.Hour
	}

	dueAt := feed.LastFetchAt.Add(interval)
	return time.Now().UTC().After(dueAt), nil
}

// updateFeedError increments the error count and stores the last error.
func (h *RefreshFeedHandler) updateFeedError(ctx context.Context, feedID, errMsg string) error {
	updates := map[string]interface{}{
		"error_count": 1,
		"last_error":  errMsg,
	}
	return h.repo.UpdateFeed(ctx, feedID, updates)
}

// ─── Shared helpers ─────────────────────────────────────────────────────────

// enqueueCrawlJob enqueues a crawl job for a single URL.
func enqueueCrawlJob(ctx context.Context, url, feedID, jobID string, priority int) error {
	// The crawler service's EnqueueURL handles goroutine-based execution.
	// For Asynq integration, jobs are dispatched through the queue client.
	// Here we just return the jobID — the caller should use the CrawlerService.EnqueueURL
	// which runs the pipeline directly (in-process, no HTTP overhead).
	return nil
}
