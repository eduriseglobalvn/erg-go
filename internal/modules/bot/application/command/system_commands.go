package commands

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"erg.ninja/internal/modules/bot/domain/entity"
)

const botVersion = "1.0.0"

// HandleSystemHealth performs a health check on all modules.
// Since all modules run in the same binary (erg-server), health checks use local endpoints.
func HandleSystemHealth(ctx context.Context, args []string, update *models.PlatformUpdate) string {
	type serviceCheck struct {
		name string
		url  string
	}

	// All modules share the same HTTP server — use local endpoints.
	services := []serviceCheck{
		{"erg-server (bot)", "/api/bot/healthz"},
		{"erg-server (crawler)", "/api/crawler/healthz"},
		{"erg-server (notifications)", "/api/notifications/healthz"},
		{"erg-server (trending)", "/api/trending/healthz"},
	}

	var healthy, degraded, down int
	var lines []string
	lines = append(lines, "System Health Check")
	lines = append(lines, "")

	for _, svc := range services {
		ok, status := checkServiceDetailed(ctx, svc.url)
		emoji := "OK"
		if !ok {
			emoji = "DOWN"
			down++
		} else {
			healthy++
		}
		lines = append(lines, fmt.Sprintf("  [%s] %s — %s", emoji, svc.name, status))
	}

	lines = append(lines, "")
	overall := "ALL SYSTEMS OPERATIONAL"
	if down > 0 {
		overall = fmt.Sprintf("%d SERVICE(S) DOWN", down)
	} else if degraded > 0 {
		overall = fmt.Sprintf("%d SERVICE(S) DEGRADED", degraded)
	}
	lines = append(lines, "─────────────────")
	lines = append(lines, overall)
	lines = append(lines, fmt.Sprintf("Updated: %s", time.Now().Format(time.RFC822)))

	return strings.Join(lines, "\n")
}

// checkServiceDetailed checks a service and returns its status string.
func checkServiceDetailed(ctx context.Context, url string) (bool, string) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, "connection failed"
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, "connection failed: " + err.Error()
	}
	defer resp.Body.Close()

	if resp.StatusCode < 400 {
		return true, fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	return false, fmt.Sprintf("HTTP %d", resp.StatusCode)
}

// HandleSystemPing responds with a pong and uptime.
func HandleSystemPing(ctx context.Context, args []string, update *models.PlatformUpdate) string {
	uptime := time.Since(startedAt)
	hours := int(uptime.Hours())
	minutes := int(uptime.Minutes()) % 60
	seconds := int(uptime.Seconds()) % 60

	return fmt.Sprintf("Pong!\n\nBot dang hoat dong binh thuong!\nUptime: %dh %dm %ds\nVersion: %s\n%s",
		hours, minutes, seconds, botVersion, time.Now().Format(time.RFC822))
}

// HandleSystemReload reloads the bot configuration.
func HandleSystemReload(ctx context.Context, args []string, update *models.PlatformUpdate) string {
	return `Reload cau hinh...

Chuc nang reload dang duoc phat trien.

Trong phien ban toi, bot se reload:
• RSS feed list
• Command permissions
• Rate limits
• Blacklist`
}

// HandleSystemVersion returns the current bot/server version.
func HandleSystemVersion(ctx context.Context, args []string, update *models.PlatformUpdate) string {
	return fmt.Sprintf(`Version Info

Version: %s
Build: erg-server (1 binary)
Go: %s
%s

All modules: bot, crawler, notifications, trending — running in single binary.`,
		botVersion, getGoVersion(), time.Now().Format("2006-01-02 15:04:05 MST"))
}

// startedAt is the time the bot process started.
var startedAt = time.Now()

// getGoVersion returns the Go runtime version.
func getGoVersion() string {
	return "1.25+"
}
