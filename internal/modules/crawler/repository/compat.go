package repository

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"erg.ninja/internal/modules/crawler/entities"
	"erg.ninja/pkg/database"
)

const (
	legacyRSSFeedCollection        = "crawler_rss_feeds"
	legacyCrawlHistoryCollection   = "crawler_history"
	legacyScraperConfigCollection  = "crawler_scraper_configs"
	currentScraperConfigCollection = "scraper_configs"
)

func crawlerIDFilter(id string) bson.M {
	if objID, ok := database.ParseObjectID(id); ok {
		return bson.M{
			"$or": []bson.M{
				{"_id": id},
				{"_id": objID},
			},
		}
	}
	return bson.M{"_id": id}
}

func normalizeLegacyFeedDoc(doc bson.M) *entities.RSSFeed {
	if len(doc) == 0 {
		return nil
	}

	createdAt := crawlerTime(doc, "created_at", "createdAt")
	updatedAt := crawlerTime(doc, "updated_at", "updatedAt")
	lastRunAt := crawlerTime(doc, "last_fetch_at", "lastFetchAt", "updatedAt", "createdAt")
	lastRunAtPtr := crawlerTimePtr(lastRunAt)

	return &entities.RSSFeed{
		ID:          crawlerIDString(doc["_id"]),
		Name:        crawlerString(doc, "name"),
		URL:         crawlerString(doc, "url"),
		Type:        crawlerDefault(crawlerString(doc, "type"), "rss"),
		Category:    crawlerString(doc, "targetCategoryId", "target_category_id", "category"),
		Frequency:   crawlerString(doc, "cronExpression", "autoSchedule", "schedule", "frequency"),
		Enabled:     crawlerBoolDefault(doc, true, "enabled", "isActive"),
		LastFetchAt: lastRunAtPtr,
		LastItemAt:  crawlerTimePtr(crawlerTime(doc, "last_item_at", "lastItemAt")),
		ItemCount:   crawlerInt(doc, "item_count"),
		ErrorCount:  crawlerInt(doc, "error_count"),
		LastError:   crawlerString(doc, "last_error", "errorMessage"),
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}
}

func normalizeLegacyHistoryDoc(doc bson.M) *entities.CrawlHistory {
	if len(doc) == 0 {
		return nil
	}

	createdAt := crawlerTime(doc, "created_at", "createdAt")
	updatedAt := crawlerTime(doc, "updated_at", "updatedAt")
	crawledAt := crawlerTime(doc, "crawled_at", "crawledAt", "updatedAt", "createdAt")

	return &entities.CrawlHistory{
		ID:           crawlerIDString(doc["_id"]),
		URL:          crawlerString(doc, "url"),
		FeedID:       crawlerString(doc, "feed_id", "feedId", "sourceId"),
		JobID:        crawlerString(doc, "job_id", "jobId"),
		Status:       normalizeCrawlStatus(crawlerString(doc, "status")),
		HTTPStatus:   crawlerInt(doc, "http_status", "httpStatus"),
		ResponseSize: crawlerInt64(doc, "response_size", "responseSize"),
		DurationMS:   crawlerInt64(doc, "duration_ms", "durationMs"),
		ErrorMsg:     crawlerString(doc, "error_msg", "errorMessage"),
		QualityScore: crawlerFloat(doc, "quality_score", "qualityScore"),
		Title:        crawlerString(doc, "title"),
		Description:  crawlerString(doc, "description"),
		ContentHash:  crawlerString(doc, "content_hash", "contentHash"),
		Language:     crawlerString(doc, "language"),
		CrawledAt:    crawledAt,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}
}

func normalizeCrawlStatus(raw string) entities.CrawlStatus {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "success":
		return entities.CrawlStatusSuccess
	case "failed", "error":
		return entities.CrawlStatusFailed
	case "pending":
		return entities.CrawlStatusPending
	case "running":
		return entities.CrawlStatusRunning
	case "duplicate":
		return entities.CrawlStatusDuplicate
	case "blacklisted":
		return entities.CrawlStatusBlacklisted
	case "canceled", "cancelled":
		return entities.CrawlStatusCanceled
	default:
		return entities.CrawlStatus(raw)
	}
}

func normalizeConfigDoc(doc bson.M) map[string]any {
	if len(doc) == 0 {
		return nil
	}

	config := map[string]any{
		"id":             crawlerIDString(doc["_id"]),
		"domain":         crawlerString(doc, "domain"),
		"type":           crawlerDefault(crawlerString(doc, "type"), "STATIC"),
		"selectorConfig": crawlerMap(doc, "selectorConfig", "selector_config"),
		"createdAt":      crawlerTime(doc, "created_at", "createdAt"),
		"updatedAt":      crawlerTime(doc, "updated_at", "updatedAt"),
	}
	if handler := crawlerString(doc, "handler"); handler != "" {
		config["handler"] = handler
	}
	if schedule := crawlerString(doc, "schedule"); schedule != "" {
		config["schedule"] = schedule
	}
	if maxRPS, ok := crawlerInt64Value(doc, "maxRequestsPerSecond", "max_requests_per_second"); ok {
		config["maxRequestsPerSecond"] = maxRPS
	}
	if active, ok := crawlerBoolValue(doc, "isActive", "is_active"); ok {
		config["isActive"] = active
	}
	if autoPublish, ok := crawlerBoolValue(doc, "autoPublish", "auto_publish"); ok {
		config["autoPublish"] = autoPublish
	}
	return config
}

func dedupeFeeds(feeds []*entities.RSSFeed) []*entities.RSSFeed {
	seen := make(map[string]struct{}, len(feeds))
	out := make([]*entities.RSSFeed, 0, len(feeds))
	for _, feed := range feeds {
		if feed == nil {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(feed.URL))
		if key == "" {
			key = feed.ID
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, feed)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

func dedupeConfigs(configs []map[string]any) []map[string]any {
	seen := make(map[string]struct{}, len(configs))
	out := make([]map[string]any, 0, len(configs))
	for _, cfg := range configs {
		if cfg == nil {
			continue
		}
		key, _ := cfg["domain"].(string)
		if key == "" {
			key, _ = cfg["id"].(string)
		}
		key = strings.ToLower(strings.TrimSpace(key))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, cfg)
	}
	sort.Slice(out, func(i, j int) bool {
		ti, _ := out[i]["createdAt"].(time.Time)
		tj, _ := out[j]["createdAt"].(time.Time)
		return ti.After(tj)
	})
	return out
}

func crawlerIDString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case bson.ObjectID:
		return v.Hex()
	case fmt.Stringer:
		return v.String()
	default:
		return ""
	}
}

func crawlerString(doc bson.M, keys ...string) string {
	for _, key := range keys {
		raw, ok := doc[key]
		if !ok || raw == nil {
			continue
		}
		switch v := raw.(type) {
		case string:
			return v
		case []byte:
			return string(v)
		default:
			return fmt.Sprintf("%v", v)
		}
	}
	return ""
}

func crawlerBoolDefault(doc bson.M, def bool, keys ...string) bool {
	if v, ok := crawlerBoolValue(doc, keys...); ok {
		return v
	}
	return def
}

func crawlerBoolValue(doc bson.M, keys ...string) (bool, bool) {
	for _, key := range keys {
		raw, ok := doc[key]
		if !ok || raw == nil {
			continue
		}
		switch v := raw.(type) {
		case bool:
			return v, true
		case string:
			parsed, err := strconv.ParseBool(v)
			if err == nil {
				return parsed, true
			}
		case int:
			return v != 0, true
		case int32:
			return v != 0, true
		case int64:
			return v != 0, true
		case float64:
			return v != 0, true
		}
	}
	return false, false
}

func crawlerInt(doc bson.M, keys ...string) int {
	if v, ok := crawlerInt64Value(doc, keys...); ok {
		return int(v)
	}
	return 0
}

func crawlerInt64(doc bson.M, keys ...string) int64 {
	if v, ok := crawlerInt64Value(doc, keys...); ok {
		return v
	}
	return 0
}

func crawlerInt64Value(doc bson.M, keys ...string) (int64, bool) {
	for _, key := range keys {
		raw, ok := doc[key]
		if !ok || raw == nil {
			continue
		}
		switch v := raw.(type) {
		case int:
			return int64(v), true
		case int32:
			return int64(v), true
		case int64:
			return v, true
		case float32:
			return int64(v), true
		case float64:
			return int64(v), true
		case string:
			parsed, err := strconv.ParseInt(v, 10, 64)
			if err == nil {
				return parsed, true
			}
		}
	}
	return 0, false
}

func crawlerFloat(doc bson.M, keys ...string) float64 {
	for _, key := range keys {
		raw, ok := doc[key]
		if !ok || raw == nil {
			continue
		}
		switch v := raw.(type) {
		case float32:
			return float64(v)
		case float64:
			return v
		case int:
			return float64(v)
		case int32:
			return float64(v)
		case int64:
			return float64(v)
		case string:
			parsed, err := strconv.ParseFloat(v, 64)
			if err == nil {
				return parsed
			}
		}
	}
	return 0
}

func crawlerMap(doc bson.M, keys ...string) map[string]any {
	for _, key := range keys {
		raw, ok := doc[key]
		if !ok || raw == nil {
			continue
		}
		switch v := raw.(type) {
		case map[string]any:
			return v
		case bson.M:
			return map[string]any(v)
		}
	}
	return map[string]any{}
}

func crawlerTime(doc bson.M, keys ...string) time.Time {
	for _, key := range keys {
		raw, ok := doc[key]
		if !ok || raw == nil {
			continue
		}
		switch v := raw.(type) {
		case time.Time:
			return v.UTC()
		case *time.Time:
			if v != nil {
				return v.UTC()
			}
		case bson.DateTime:
			return v.Time().UTC()
		case string:
			if parsed, err := time.Parse(time.RFC3339, v); err == nil {
				return parsed.UTC()
			}
			if parsed, err := time.Parse("2006-01-02 15:04:05", v); err == nil {
				return parsed.UTC()
			}
		case int64:
			return time.UnixMilli(v).UTC()
		}
	}
	return time.Time{}
}

func crawlerTimePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	value := t.UTC()
	return &value
}

func crawlerDefault(value, def string) string {
	if strings.TrimSpace(value) == "" {
		return def
	}
	return value
}
