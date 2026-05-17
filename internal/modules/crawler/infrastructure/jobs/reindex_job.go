// Package jobs provides Asynq job handlers for the crawler module.
package jobs

import (
	"context"
	"crypto/sha256"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hibiken/asynq"

	entities "erg.ninja/internal/modules/crawler/domain/entity"
	"erg.ninja/internal/modules/crawler/infrastructure/repository"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/queue"
)

// ReindexHandler processes a content reindex job from the Asynq queue.
type ReindexHandler struct {
	repo *repository.Repository
	log  *logger.Logger
}

// NewReindexHandler creates a new reindex job handler.
func NewReindexHandler(repo *repository.Repository, log *logger.Logger) *ReindexHandler {
	return &ReindexHandler{repo: repo, log: log}
}

// Handle processes a crawler:reindex Asynq task.
// Payload: entities.ReindexPayload
func (h *ReindexHandler) Handle(ctx context.Context, t *asynq.Task) error {
	var payload entities.ReindexPayload
	if err := queue.ParsePayload(t.Payload(), &payload); err != nil {
		return fmt.Errorf("reindex: parse payload: %w", err)
	}

	h.log.Info().
		Str("algorithm", payload.Algorithm).
		Str("since", payload.Since).
		Int("batch_size", payload.BatchSize).
		Msg("reindex: started")

	batchSize := payload.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}

	processed := 0
	failed := 0
	offset := int64(0)

	for {
		filter := repository.ListCrawlHistoryParams{
			Limit:  int64(batchSize),
			Offset: offset,
			Status: entities.CrawlStatusSuccess,
		}

		// Skip entries older than "since" timestamp (filtered in-memory).
		var sinceTime time.Time
		if payload.Since != "" {
			var err error
			sinceTime, err = time.Parse(time.RFC3339, payload.Since)
			if err != nil {
				return fmt.Errorf("reindex: parse since %s: %w", payload.Since, err)
			}
		}

		histories, total, err := h.repo.ListCrawlHistory(ctx, filter)
		if err != nil {
			return fmt.Errorf("reindex: list history: %w", err)
		}

		if len(histories) == 0 {
			break
		}

		for _, history := range histories {
			if sinceTime.After(history.UpdatedAt) {
				continue
			}
			if err := h.reindexHistory(ctx, history, payload.Algorithm); err != nil {
				h.log.Warn().Str("url", history.URL).Err(err).Msg("reindex: failed for url")
				failed++
				continue
			}
			processed++
		}

		offset += int64(len(histories))
		if offset >= total {
			break
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	h.log.Info().
		Int("processed", processed).
		Int("failed", failed).
		Msg("reindex: completed")

	return nil
}

// reindexHistory re-fingerprints a single crawl history entry.
func (h *ReindexHandler) reindexHistory(ctx context.Context, history *entities.CrawlHistory, algorithm string) error {
	if history.URL == "" {
		return fmt.Errorf("empty url")
	}

	// Fetch the original page.
	content, _, err := fetchRawPage(ctx, history.URL)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}

	switch algorithm {
	case "simhash", "all", "":
		simhash := computeSimHash(content)
		sha256Str := computeSHA256(content)

		fp := &entities.ContentFingerprint{
			ID:      history.ID + "-fp",
			URL:     history.URL,
			SimHash: simhash,
			SHA256:  sha256Str,
			Bucket:  uint16(simhash >> 48),
			CrawlID: history.ID,
		}
		if err := h.repo.StoreFingerprint(ctx, fp); err != nil {
			h.log.Warn().Str("url", history.URL).Err(err).Msg("reindex: store fingerprint failed")
		}

	case "sha256":
		_ = computeSHA256(content)
	}

	return nil
}

// fetchRawPage fetches raw HTML for a URL with a 15-second timeout.
func fetchRawPage(ctx context.Context, urlStr string) (string, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; ERG-Bot/1.0; +https://erg.ninja)")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", resp.StatusCode, fmt.Errorf("status %d", resp.StatusCode)
	}

	lr := io.LimitReader(resp.Body, 2<<20) // max 2MB
	body, err := io.ReadAll(lr)
	if err != nil {
		return "", resp.StatusCode, err
	}

	return string(body), resp.StatusCode, nil
}

// computeSimHash produces a 64-bit SimHash fingerprint via FNV-1a trigram hashing.
func computeSimHash(text string) uint64 {
	tokens := tokenizeForSimhash(strings.ToLower(text))
	if len(tokens) < 3 {
		return 0
	}

	var v0, v1 uint64
	for i := 0; i+2 < len(tokens); i++ {
		trigram := tokens[i] + " " + tokens[i+1] + " " + tokens[i+2]
		h := fnv1aHash(trigram)
		if i%2 == 0 {
			v0 += h
		} else {
			v1 += h
		}
	}

	return (v0 & 0xFFFFFFFF) ^ (v1 & 0xFFFFFFFF00000000)
}

func tokenizeForSimhash(text string) []string {
	var tokens []string
	var current strings.Builder
	for _, r := range text {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			current.WriteRune(r)
		} else {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

func fnv1aHash(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

func computeSHA256(text string) string {
	h := sha256.Sum256([]byte(text))
	return fmt.Sprintf("%x", h)
}
