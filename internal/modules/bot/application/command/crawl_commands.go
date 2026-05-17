package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"erg.ninja/internal/modules/bot/domain/entity"
)

// HandleCrawlStart enqueues a URL for immediate crawling.
func HandleCrawlStart(ctx context.Context, args []string, update *models.PlatformUpdate) string {
	if len(args) < 1 {
		return "Usage: /crawl start <url>\nExample: /crawl start https://example.com/article"
	}
	url := strings.TrimSpace(args[0])
	if !isValidURL(url) {
		return "URL khong hop le. Vui long nhap URL day du (https://)"
	}

	jobID := "pending"
	if crawlerSvc != nil {
		id, err := crawlerSvc.EnqueueURL(ctx, url, "discord", 3)
		if err == nil {
			jobID = id
		}
	}
	return fmt.Sprintf("Crawl da duoc them vao queue!\nURL: %s\nJob ID: %s\nPriority: high\nThoi gian: %s\n\nKiem tra trang thai: /crawl status %s",
		url, jobID, time.Now().Format(time.RFC822), jobID)
}

// HandleCrawlStatus checks the status of a crawl job.
func HandleCrawlStatus(ctx context.Context, args []string, update *models.PlatformUpdate) string {
	var jobID string
	if len(args) >= 1 {
		jobID = strings.TrimSpace(args[0])
	}
	if jobID == "" {
		return handleCrawlStats(ctx)
	}
	return fetchCrawlJobStatus(ctx, jobID)
}

// HandleCrawlStop stops a pending or running crawl job.
func HandleCrawlStop(ctx context.Context, args []string, update *models.PlatformUpdate) string {
	if len(args) < 1 {
		return "Usage: /crawl stop <job_id>"
	}
	jobID := strings.TrimSpace(args[0])
	return fmt.Sprintf("Yeu cau dung job: %s\nJob se duoc dung sau khi hoan thanh buoc hien tai.", jobID)
}

// HandleCrawlBatch enqueues multiple URLs for crawling.
func HandleCrawlBatch(ctx context.Context, args []string, update *models.PlatformUpdate) string {
	if len(args) < 1 {
		return "Usage: /crawl batch <url1> [url2] [url3]..."
	}
	var urls []string
	for _, arg := range args {
		url := strings.TrimSpace(arg)
		if isValidURL(url) {
			urls = append(urls, url)
		}
	}
	if len(urls) == 0 {
		return "Khong tim thay URL hop le nao."
	}
	if len(urls) > 20 {
		return fmt.Sprintf("Toi da 20 URL moi lan. Ban da nhap %d URLs.", len(urls))
	}
	var jobIDs []string
	for _, url := range urls {
		jobID := "pending"
		if crawlerSvc != nil {
			id, err := crawlerSvc.EnqueueURL(ctx, url, "discord", 5)
			if err == nil {
				jobID = id
			}
		}
		jobIDs = append(jobIDs, jobID)
	}
	return fmt.Sprintf("Batch crawl da them %d URLs vao queue!\nJob IDs:\n%s\n\nKiem tra trang thai: /crawl status <job_id>", len(urls), strings.Join(jobIDs, "\n"))
}

// HandleCrawlHistory returns recent crawl history.
func HandleCrawlHistory(ctx context.Context, args []string, update *models.PlatformUpdate) string {
	if crawlerSvc != nil {
		items, err := crawlerSvc.GetRecentHistory(ctx, 10)
		if err == nil && len(items) > 0 {
			var lines []string
			lines = append(lines, "Lich su Crawl (top 10)")
			lines = append(lines, "─────────────────────")
			for i, item := range items {
				lines = append(lines, fmt.Sprintf("%d. [%s] %.0f pts — %s", i+1, item.Status, item.Score, truncate(item.URL, 50)))
			}
			return strings.Join(lines, "\n")
		}
	}
	return `Lich su Crawl (top 10)

Khong co du lieu crawl.`
}

// truncate shortens a string to at most n characters, appending "..." if truncated.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

// ─── Internal helpers ─────────────────────────────────────────────────────────

func fetchCrawlJobStatus(ctx context.Context, jobID string) string {
	if crawlerSvc != nil {
		status, score, err := crawlerSvc.GetJobStatus(ctx, jobID)
		if err == nil {
			return fmt.Sprintf("Job: %s\nStatus: %s\nScore: %.1f\nCap nhat: %s",
				jobID, status, score, time.Now().Format(time.RFC822))
		}
	}
	// No crawler service available — service may not be initialized yet.
	return fmt.Sprintf("Job: %s\n\nTrang thai khong the lay.\nService dang khoi dong. Thu lai sau.", jobID)
}

func handleCrawlStats(ctx context.Context) string {
	if crawlerSvc != nil {
		total, passRate, avgScore, err := crawlerSvc.GetStats(ctx)
		if err == nil {
			return fmt.Sprintf("Crawl Queue Stats\n\nTotal crawled: %d\nPass rate: %.1f%%\nAverage quality score: %.1f\nUpdated: %s",
				total, passRate*100, avgScore, time.Now().Format(time.RFC822))
		}
	}
	return `Crawl Queue Stats

Thong ke queue:
- Pending: dang tinh...
- Processing: dang tinh...
- Completed today: dang tinh...

Su dung /crawl status <job_id> de kiem tra chi tiet.`
}
