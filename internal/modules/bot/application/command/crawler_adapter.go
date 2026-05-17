// Package commands provides service adapters for bot command integration with other modules.
package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// HTTPCrawlerAdapter implements CrawlerServiceClient using the local HTTP API.
// Used when crawler service is not injected directly (dev/standalone mode).
type HTTPCrawlerAdapter struct {
	BaseURL string
	Client  *http.Client
}

// NewHTTPCrawlerAdapter creates a crawler adapter pointing to the local API server.
func NewHTTPCrawlerAdapter(baseURL string) *HTTPCrawlerAdapter {
	return &HTTPCrawlerAdapter{
		BaseURL: baseURL,
		Client:  &http.Client{Timeout: 10 * time.Second},
	}
}

// EnqueueURL implements CrawlerServiceClient.EnqueueURL via local HTTP.
func (a *HTTPCrawlerAdapter) EnqueueURL(ctx context.Context, url, source string, priority int) (string, error) {
	payload := map[string]any{
		"url":      url,
		"source":   source,
		"priority": priority,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", a.BaseURL+"/api/crawler/crawl", strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("crawler returned HTTP %d", resp.StatusCode)
	}
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if jobID, ok := result["job_id"].(string); ok && jobID != "" {
		return jobID, nil
	}
	return fmt.Sprintf("sim-%d", time.Now().UnixNano()), nil
}

// GetJobStatus implements CrawlerServiceClient.GetJobStatus via local HTTP.
func (a *HTTPCrawlerAdapter) GetJobStatus(ctx context.Context, jobID string) (string, float64, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", a.BaseURL+fmt.Sprintf("/api/crawler/crawl/%s", jobID), nil)
	if err != nil {
		return "unknown", 0, err
	}
	resp, err := a.Client.Do(req)
	if err != nil {
		return "unknown", 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "not_found", 0, nil
	}
	var result map[string]any
	bs, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	json.Unmarshal(bs, &result)
	status := getStringField(result, "status", "unknown")
	score := getFloatField(result, "score", 0)
	return status, score, nil
}

// GetStats implements CrawlerServiceClient.GetStats via local HTTP.
func (a *HTTPCrawlerAdapter) GetStats(ctx context.Context) (int, float64, float64, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", a.BaseURL+"/api/crawler/stats", nil)
	if err != nil {
		return 0, 0, 0, err
	}
	resp, err := a.Client.Do(req)
	if err != nil {
		return 0, 0, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return 0, 0, 0, fmt.Errorf("crawler stats returned HTTP %d", resp.StatusCode)
	}
	var result map[string]any
	bs, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	json.Unmarshal(bs, &result)
	total := getIntField(result, "total_crawled", 0)
	passRate := getFloatField(result, "pass_rate", 0)
	avgScore := getFloatField(result, "avg_quality_score", 0)
	return total, passRate, avgScore, nil
}

// GetRecentHistory implements CrawlerServiceClient.GetRecentHistory via local HTTP.
func (a *HTTPCrawlerAdapter) GetRecentHistory(ctx context.Context, limit int) ([]CrawlHistoryItem, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		a.BaseURL+fmt.Sprintf("/api/crawler/history?limit=%d", limit), nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("crawler history returned HTTP %d", resp.StatusCode)
	}
	var result map[string]any
	bs, _ := io.ReadAll(io.LimitReader(resp.Body, 65536))
	if err := json.Unmarshal(bs, &result); err != nil {
		return nil, err
	}
	itemsRaw, ok := result["items"].([]any)
	if !ok {
		return nil, nil
	}
	items := make([]CrawlHistoryItem, 0, len(itemsRaw))
	for _, item := range itemsRaw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		items = append(items, CrawlHistoryItem{
			URL:    getStringField(m, "url", ""),
			Status: getStringField(m, "status", ""),
			Score:  getFloatField(m, "quality_score", 0),
		})
	}
	return items, nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func getStringField(m map[string]any, key, def string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return def
}

func getFloatField(m map[string]any, key string, def float64) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	return def
}

func getIntField(m map[string]any, key string, def int) int {
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	return def
}

// SimCrawlerAdapter implements CrawlerServiceClient for testing without a real crawler.
type SimCrawlerAdapter struct{}

// NewSimCrawlerAdapter creates a simulated crawler adapter.
func NewSimCrawlerAdapter() *SimCrawlerAdapter { return &SimCrawlerAdapter{} }

// EnqueueURL simulates enqueueing a crawl URL.
func (a *SimCrawlerAdapter) EnqueueURL(ctx context.Context, url, source string, priority int) (string, error) {
	return fmt.Sprintf("sim-%d", time.Now().UnixNano()), nil
}

// GetJobStatus simulates getting job status.
func (a *SimCrawlerAdapter) GetJobStatus(ctx context.Context, jobID string) (string, float64, error) {
	return "success", 85.0, nil
}

// GetStats simulates getting crawl stats.
func (a *SimCrawlerAdapter) GetStats(ctx context.Context) (int, float64, float64, error) {
	return 0, 0, 0, nil
}

// GetRecentHistory simulates getting recent crawl history.
func (a *SimCrawlerAdapter) GetRecentHistory(ctx context.Context, limit int) ([]CrawlHistoryItem, error) {
	return nil, nil
}

// InitCrawlerService auto-detects the best crawler adapter.
func InitCrawlerService(baseURL string) CrawlerServiceClient {
	if crawlerSvc != nil {
		return crawlerSvc
	}
	return NewHTTPCrawlerAdapter(baseURL)
}
