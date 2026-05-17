// Package commands implements all bot commands for the erg-server binary.
// Each command is a standalone function that receives the command context and
// returns a response string to send back to the user.
//
// Service dependencies (crawler, trending) are injected at startup via
// SetCrawlerService/SetTrendingService (see services.go).
package commands

import (
	"context"
	"net/url"
	"strings"

	"erg.ninja/internal/modules/bot/domain/entity"
)

// CommandContext holds the runtime context for a command execution.
// It provides access to all service dependencies.
type CommandContext struct {
	// Service clients are injected via SetCrawlerService/SetTrendingService.
}

// HandlerFunc is the function signature for a command handler.
type HandlerFunc func(ctx context.Context, args []string, update *models.PlatformUpdate) string

// SetCommandContext configures the global command context (used for service dependencies).
func SetCommandContext(_ *CommandContext) {
	// Service dependencies are now injected via SetCrawlerService/SetTrendingService.
}

// isValidURL validates a URL: must have HTTP or HTTPS scheme, and a valid host.
func isValidURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return false
	}
	if parsed.Host == "" {
		return false
	}
	return true
}

// ─── Registry builder ─────────────────────────────────────────────────────────

// BuildRegistry creates the full command registry with all 36+ commands.
func BuildRegistry() *models.CommandRegistry {
	r := models.NewCommandRegistry()

	// RSS commands.
	r.Register(&models.CommandEntry{Name: "rss add", Usage: "/rss add <url>", Description: "Thêm RSS feed mới", Category: "rss"})
	r.Register(&models.CommandEntry{Name: "rss list", Usage: "/rss list", Description: "Liệt kê tất cả RSS feeds", Category: "rss"})
	r.Register(&models.CommandEntry{Name: "rss remove", Usage: "/rss remove <url>", Description: "Xóa RSS feed", Category: "rss"})
	r.Register(&models.CommandEntry{Name: "rss sync", Usage: "/rss sync", Description: "Đồng bộ tất cả feeds ngay", Category: "rss"})
	r.Register(&models.CommandEntry{Name: "rss preview", Usage: "/rss preview <url>", Description: "Xem trước RSS feed", Category: "rss"})

	// Crawl commands.
	r.Register(&models.CommandEntry{Name: "crawl start", Usage: "/crawl start <url>", Description: "Bắt đầu crawl URL", Category: "crawl"})
	r.Register(&models.CommandEntry{Name: "crawl status", Usage: "/crawl status [job_id]", Description: "Kiểm tra trạng thái crawl", Category: "crawl"})
	r.Register(&models.CommandEntry{Name: "crawl stop", Usage: "/crawl stop <job_id>", Description: "Dừng crawl job", Category: "crawl"})
	r.Register(&models.CommandEntry{Name: "crawl batch", Usage: "/crawl batch <url1> [url2] [url3]...", Description: "Crawl nhiều URL cùng lúc", Category: "crawl"})
	r.Register(&models.CommandEntry{Name: "crawl history", Usage: "/crawl history [limit]", Description: "Lịch sử crawl gần đây", Category: "crawl"})

	// Trending commands.
	r.Register(&models.CommandEntry{Name: "trending top", Usage: "/trending top", Description: "Top 10 topics đang hot", Category: "trending"})
	r.Register(&models.CommandEntry{Name: "trending keyword", Usage: "/trending keyword <topic>", Description: "Chi tiết topic cụ thể", Category: "trending"})

	// Draft commands.
	r.Register(&models.CommandEntry{Name: "draft list", Usage: "/draft list", Description: "Liệt kê bản nháp", Category: "draft"})
	r.Register(&models.CommandEntry{Name: "draft publish", Usage: "/draft publish <id>", Description: "Xuất bản bản nháp", Category: "draft"})
	r.Register(&models.CommandEntry{Name: "draft delete", Usage: "/draft delete <id>", Description: "Xóa bản nháp", Category: "draft"})

	// Stats commands.
	r.Register(&models.CommandEntry{Name: "stats users", Usage: "/stats users", Description: "Số người dùng hệ thống", Category: "stats"})
	r.Register(&models.CommandEntry{Name: "stats crawler", Usage: "/stats crawler", Description: "Thống kê crawler", Category: "stats"})
	r.Register(&models.CommandEntry{Name: "stats queue", Usage: "/stats queue", Description: "Trạng thái job queue", Category: "stats"})
	r.Register(&models.CommandEntry{Name: "stats system", Usage: "/stats system", Description: "Tổng quan hệ thống", Category: "stats"})

	// System commands.
	r.Register(&models.CommandEntry{Name: "system health", Usage: "/system health", Description: "Health check tất cả modules", Category: "system"})
	r.Register(&models.CommandEntry{Name: "system ping", Usage: "/system ping", Description: "Kiểm tra bot online", Category: "system"})
	r.Register(&models.CommandEntry{Name: "system reload", Usage: "/system reload", Description: "Reload cấu hình (admin)", Category: "system"})
	r.Register(&models.CommandEntry{Name: "system version", Usage: "/system version", Description: "Phiên bản hiện tại", Category: "system"})

	// Help.
	r.Register(&models.CommandEntry{Name: "help", Usage: "/help", Description: "Hiển thị danh sách lệnh", Category: "system"})

	return r
}
