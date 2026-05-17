package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"erg.ninja/internal/modules/bot/domain/entity"
)

// HandleTrendingTop returns the top trending topics.
func HandleTrendingTop(ctx context.Context, args []string, update *models.PlatformUpdate) string {
	limit := 10
	if len(args) > 0 {
		// Support /trending top 20
		if l := parseInt(args[0]); l > 0 && l <= 50 {
			limit = l
		}
	}

	// Use injected service if available.
	if trendingSvc != nil {
		topics, err := trendingSvc.GetTopTopics(ctx, limit)
		if err != nil {
			return "Top Trending Topics\n\nKhong the lay du lieu trending. Thu lai sau."
		}
		if len(topics) == 0 {
			return "Top Trending Topics\n\nChua co data trending."
		}
		var lines []string
		lines = append(lines, "Top Trending Topics")
		lines = append(lines, fmt.Sprintf("Updated: %s\n", time.Now().Format(time.RFC822)))
		for i, topic := range topics {
			rank := i + 1
			lines = append(lines, fmt.Sprintf("#%d. %s (%s, %s)",
				rank, topic.Topic, formatVolume(topic.Volume), topic.Source))
		}
		return strings.Join(lines, "\n")
	}

	return "Top Trending Topics\n\nTrending service chua san sang. Thu lai sau."
}

// HandleTrendingKeyword returns detailed info about a specific trending keyword.
func HandleTrendingKeyword(ctx context.Context, args []string, update *models.PlatformUpdate) string {
	if len(args) < 1 {
		return "Usage: /trending keyword <topic>\nExample: /trending keyword AI"
	}
	topic := strings.TrimSpace(strings.Join(args, " "))

	// Use injected service if available.
	if trendingSvc != nil {
		detail, err := trendingSvc.GetTopicDetail(ctx, topic)
		if err != nil {
			return fmt.Sprintf("Topic: %s\n\nKhong the lay chi tiet. Thu lai sau.", topic)
		}
		var lines []string
		lines = append(lines, fmt.Sprintf("Topic: %s\nScore: %.2f\nVolume: %s\nSource: %s",
			detail.Topic, detail.Score, formatVolume(detail.Volume), detail.Source))
		if len(detail.Keywords) > 0 {
			lines = append(lines, fmt.Sprintf("Related Keywords: %s", strings.Join(detail.Keywords, ", ")))
		}
		return strings.Join(lines, "\n")
	}

	return fmt.Sprintf("Topic: %s\n\nTrending service chua san sang. Thu lai sau.", topic)
}

func formatVolume(v int) string {
	if v >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(v)/1000000)
	}
	if v >= 1000 {
		return fmt.Sprintf("%.1fK", float64(v)/1000)
	}
	return fmt.Sprintf("%d", v)
}

func parseInt(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}
