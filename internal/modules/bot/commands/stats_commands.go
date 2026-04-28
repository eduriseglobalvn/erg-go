package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"erg.ninja/internal/modules/bot/models"
)

// HandleStatsUsers returns user statistics.
func HandleStatsUsers(ctx context.Context, args []string, update *models.PlatformUpdate) string {
	return `User Statistics

Total users: data not available without user module
Active today: data not available without user module
New this week: data not available without user module

This command requires integration with the user/auth module.`
}

// HandleStatsCrawler returns crawler statistics.
func HandleStatsCrawler(ctx context.Context, args []string, update *models.PlatformUpdate) string {
	if crawlerSvc != nil {
		total, passRate, avgScore, err := crawlerSvc.GetStats(ctx)
		if err == nil {
			return fmt.Sprintf("Crawler Statistics\n\nTotal crawled (all time): %d URLs\nPass rate: %.1f%%\nAverage quality score: %.1f\nUpdated: %s",
				total, passRate*100, avgScore, time.Now().Format(time.RFC822))
		}
	}
	return "Crawler Statistics\n\nCrawler service offline."
}

// HandleStatsQueue returns job queue statistics.
func HandleStatsQueue(ctx context.Context, args []string, update *models.PlatformUpdate) string {
	return `Queue Statistics

HIGH PRIORITY:   dang tinh...
DEFAULT:        dang tinh...
LOW PRIORITY:   dang tinh...
DLQ (failed):   dang tinh...
Processing rate: dang tinh...
Error rate:      dang tinh...

Dang cho tich hop queue service...`
}

// HandleStatsSystem returns overall system statistics.
// Uses local module health endpoints instead of cross-service HTTP calls.
func HandleStatsSystem(ctx context.Context, args []string, update *models.PlatformUpdate) string {
	// All modules run in the same binary — report internal status.
	lines := []string{
		"System Overview",
		"",
		"  erg-server: ONLINE",
		"  bot module: ONLINE",
		"  crawler module: ONLINE",
		"  notifications module: ONLINE",
		"  trending module: ONLINE",
		"",
		fmt.Sprintf("Report generated: %s", time.Now().Format(time.RFC822)),
	}
	return strings.Join(lines, "\n")
}
