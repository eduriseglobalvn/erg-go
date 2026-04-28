// Package jobs provides Asynq job handlers for the crawler module.
package jobs

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"

	"erg.ninja/internal/modules/crawler"
	"erg.ninja/internal/modules/crawler/entities"
	"erg.ninja/pkg/event"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/queue"
)

// Job type names.
const (
	TypeCrawlJob       = "crawler:crawl"
	TypeRefreshFeedJob = "crawler:refresh_feed"
	TypeReindexJob     = "crawler:reindex"
)

// CrawlJobHandler processes a crawl job from the Asynq queue.
type CrawlJobHandler struct {
	svc      *crawler.Service
	log      *logger.Logger
	eventBus *event.EventBus
}

// NewCrawlJobHandler creates a new crawl job handler.
func NewCrawlJobHandler(svc *crawler.Service, log *logger.Logger, bus *event.EventBus) *CrawlJobHandler {
	return &CrawlJobHandler{svc: svc, log: log, eventBus: bus}
}

// Handle processes a crawler:crawl Asynq task.
// Payload: entities.CrawlJobPayload
func (h *CrawlJobHandler) Handle(ctx context.Context, t *asynq.Task) error {
	var payload entities.CrawlJobPayload
	if err := queue.ParsePayload(t.Payload(), &payload); err != nil {
		return fmt.Errorf("crawl-job: parse payload: %w", err)
	}

	h.log.Info().
		Str("job_id", payload.JobID).
		Str("url", payload.URL).
		Int("priority", payload.Priority).
		Int("depth", payload.Depth).
		Msg("crawl-job: processing")

	// Run the full 12-step pipeline.
	result := h.svc.RunPipeline(ctx, payload.URL, payload.FeedID, payload.JobID)

	if result.Success {
		h.log.Info().
			Str("job_id", payload.JobID).
			Str("url", payload.URL).
			Float64("quality_score", result.QualityScore).
			Dur("duration", result.Duration).
			Msg("crawl-job: completed successfully")
	} else {
		h.log.Warn().
			Str("job_id", payload.JobID).
			Str("url", payload.URL).
			Str("status", string(result.Status)).
			Str("error", result.ErrorMsg).
			Int("step", result.Step).
			Msg("crawl-job: completed with failure")
	}

	// Publish event for downstream modules (notification, trending).
	if h.eventBus != nil {
		eventType := "crawl.success"
		if !result.Success {
			eventType = "crawl.failed"
		}
		eventPayload := map[string]string{
			"url":           payload.URL,
			"feed_id":       payload.FeedID,
			"job_id":        payload.JobID,
			"status":        string(result.Status),
			"quality_score": fmt.Sprintf("%.1f", result.QualityScore),
			"title":         result.Title,
		}
		if result.ErrorMsg != "" {
			eventPayload["error"] = result.ErrorMsg
		}
		_ = h.eventBus.Publish(ctx, eventType, eventPayload)
	}

	return nil
}
