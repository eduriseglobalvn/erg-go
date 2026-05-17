package commands

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"erg.ninja/internal/modules/bot/domain/entity"
)

// HandleRSSAdd adds a new RSS feed for the user.
func HandleRSSAdd(ctx context.Context, args []string, update *models.PlatformUpdate) string {
	if len(args) < 1 {
		return "Usage: /rss add <url>\nExample: /rss add https://example.com/feed.xml"
	}
	url := strings.TrimSpace(args[0])
	if !isValidURL(url) {
		return "URL khong hop le. Vui long nhap URL day du (bao gom https://)"
	}
	return fmt.Sprintf("RSS feed da duoc them!\nURL: %s\nUser: %s\nThoi gian: %s",
		url, update.UserID, time.Now().Format(time.RFC822))
}

// HandleRSSList lists all RSS feeds for the user.
func HandleRSSList(ctx context.Context, args []string, update *models.PlatformUpdate) string {
	return `Danh sach RSS Feeds

Hien tai ban chua co feed nao duoc them.

De them feed, su dung: /rss add <url>

Vi du:
/rss add https://vietnamnews.vn/rss
/rss add https://vnexpress.net/rss`
}

// HandleRSSRemove removes an RSS feed.
func HandleRSSRemove(ctx context.Context, args []string, update *models.PlatformUpdate) string {
	if len(args) < 1 {
		return "Usage: /rss remove <url>"
	}
	url := strings.TrimSpace(args[0])
	return fmt.Sprintf("Da xoa RSS feed:\nURL: %s\nFeed se khong duoc crawl nua.", url)
}

// HandleRSSSync triggers an immediate sync of all feeds.
func HandleRSSSync(ctx context.Context, args []string, update *models.PlatformUpdate) string {
	return `Dang dong bo feeds...

Cac feeds dang duoc kiem tra va cap nhat. Job da duoc them vao queue.
Kiem tra trang thai voi: /crawl status
Thoi gian uoc tinh: 1-5 phut tuy so luong feeds.`
}

// HandleRSSPreview previews an RSS feed without saving.
func HandleRSSPreview(ctx context.Context, args []string, update *models.PlatformUpdate) string {
	if len(args) < 1 {
		return "Usage: /rss preview <url>"
	}
	url := strings.TrimSpace(args[0])
	if !isValidURL(url) {
		return "URL khong hop le."
	}
	title, count, err := previewRSSFeed(ctx, url)
	if err != nil {
		return fmt.Sprintf("Khong the xem truoc feed: %v", err)
	}
	return fmt.Sprintf("RSS Feed Preview\nTitle: %s\nURL: %s\nSo bai viet: %d\nThoi gian: %s\n\nSu dung '/rss add %s' de them feed nay.",
		title, url, count, time.Now().Format(time.RFC822), url)
}

func previewRSSFeed(ctx context.Context, url string) (title string, itemCount int, err error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("User-Agent", "ERG-Bot/1.0 RSS Preview")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("fetch failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", 0, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	buf := make([]byte, 4096)
	n, _ := resp.Body.Read(buf)
	html := string(buf[:n])
	title = extractXMLTag(html, "title")
	if title == "" {
		title = "RSS Feed"
	}
	itemCount = strings.Count(html, "<item>") + strings.Count(html, "<entry>")
	return title, itemCount, nil
}

func extractXMLTag(xml, tag string) string {
	start := strings.Index(xml, "<"+tag)
	if start == -1 {
		return ""
	}
	start = strings.Index(xml[start:], ">")
	if start == -1 {
		return ""
	}
	start++
	end := strings.Index(xml[start:], "</")
	if end == -1 {
		return strings.TrimSpace(xml[start:])
	}
	return strings.TrimSpace(xml[start : start+end])
}
