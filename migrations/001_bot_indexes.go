// Package migrations contains MongoDB index definitions for all services.
// Each migration file targets a specific service's collections.
package migrations

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Run001BotIndexes creates indexes for the bot-service collections.
func Run001BotIndexes(ctx context.Context, db *mongo.Database) error {
	collections := []struct {
		name       string
		indexes    []mongo.IndexModel
		indexNames []string
	}{
		{
			name: "bot_conversations",
			indexes: []mongo.IndexModel{
				{
					Options: options.Index().
						SetExpireAfterSeconds(30 * 24 * 60 * 60).
						SetName("ttl_updated_at"),
					Keys: bson.D{{Key: "updated_at", Value: 1}},
				},
				{
					Options: options.Index().SetName("idx_user_platform"),
					Keys: bson.D{
						{Key: "user_id", Value: 1},
						{Key: "platform", Value: 1},
					},
				},
				{
					Options: options.Index().SetName("idx_status_updated"),
					Keys: bson.D{
						{Key: "status", Value: 1},
						{Key: "updated_at", Value: -1},
					},
				},
				{
					Options: options.Index().SetName("idx_platform"),
					Keys:    bson.D{{Key: "platform", Value: 1}},
				},
			},
			indexNames: []string{
				"ttl_updated_at",
				"idx_user_platform",
				"idx_status_updated",
				"idx_platform",
			},
		},
		{
			name: "bot_commands",
			indexes: []mongo.IndexModel{
				{
					Options: options.Index().SetName("idx_command_name").SetUnique(true),
					Keys:    bson.D{{Key: "name", Value: 1}},
				},
				{
					Options: options.Index().SetName("idx_guild"),
					Keys:    bson.D{{Key: "guild_id", Value: 1}},
				},
			},
			indexNames: []string{
				"idx_command_name",
				"idx_guild",
			},
		},
		{
			name: "bot_workflows",
			indexes: []mongo.IndexModel{
				{
					Options: options.Index().SetName("idx_user_status"),
					Keys: bson.D{
						{Key: "user_id", Value: 1},
						{Key: "status", Value: 1},
					},
				},
				{
					Options: options.Index().SetName("idx_next_run"),
					Keys:    bson.D{{Key: "next_run_at", Value: 1}},
				},
			},
			indexNames: []string{
				"idx_user_status",
				"idx_next_run",
			},
		},
	}

	return createIndexes(ctx, db, collections)
}

// Run002NotificationIndexes creates indexes for the notification-service collections.
func Run002NotificationIndexes(ctx context.Context, db *mongo.Database) error {
	collections := []struct {
		name       string
		indexes    []mongo.IndexModel
		indexNames []string
	}{
		{
			name: "notifications",
			indexes: []mongo.IndexModel{
				{
					Options: options.Index().SetName("idx_recipient"),
					Keys:    bson.D{{Key: "recipient", Value: 1}},
				},
				{
					Options: options.Index().SetName("idx_channel"),
					Keys:    bson.D{{Key: "channel", Value: 1}},
				},
				{
					Options: options.Index().SetName("idx_status"),
					Keys:    bson.D{{Key: "status", Value: 1}},
				},
				{
					Options: options.Index().SetName("idx_status_created"),
					Keys: bson.D{
						{Key: "status", Value: 1},
						{Key: "created_at", Value: -1},
					},
				},
				{
					Options: options.Index().
						SetExpireAfterSeconds(0).
						SetName("ttl_expires_at"),
					Keys: bson.D{{Key: "expires_at", Value: 1}},
				},
				{
					Options: options.Index().SetName("idx_recipient_channel_status"),
					Keys: bson.D{
						{Key: "recipient", Value: 1},
						{Key: "channel", Value: 1},
						{Key: "status", Value: 1},
					},
				},
			},
			indexNames: []string{
				"idx_recipient",
				"idx_channel",
				"idx_status",
				"idx_status_created",
				"ttl_expires_at",
				"idx_recipient_channel_status",
			},
		},
		{
			name: "notification_templates",
			indexes: []mongo.IndexModel{
				{
					Options: options.Index().
						SetName("idx_channel_name").
						SetUnique(true),
					Keys: bson.D{
						{Key: "channel", Value: 1},
						{Key: "name", Value: 1},
					},
				},
			},
			indexNames: []string{
				"idx_channel_name",
			},
		},
	}

	return createIndexes(ctx, db, collections)
}

// Run003CrawlerIndexes creates indexes for the crawler-service collections.
func Run003CrawlerIndexes(ctx context.Context, db *mongo.Database) error {
	collections := []struct {
		name       string
		indexes    []mongo.IndexModel
		indexNames []string
	}{
		{
			name: "crawl_history",
			indexes: []mongo.IndexModel{
				{
					Options: options.Index().
						SetName("idx_url_unique").
						SetUnique(true),
					Keys: bson.D{{Key: "url", Value: 1}},
				},
				{
					Options: options.Index().
						SetName("idx_completed_score").
						SetPartialFilterExpression(bson.D{
							{Key: "status", Value: "completed"},
						}),
					Keys: bson.D{
						{Key: "status", Value: 1},
						{Key: "score", Value: -1},
					},
				},
				{
					Options: options.Index().SetName("idx_simhash_bucket"),
					Keys:    bson.D{{Key: "simhash_bucket", Value: 1}},
				},
				{
					Options: options.Index().SetName("idx_domain"),
					Keys:    bson.D{{Key: "domain", Value: 1}},
				},
				{
					Options: options.Index().SetName("idx_crawled_at"),
					Keys:    bson.D{{Key: "crawled_at", Value: -1}},
				},
				{
					Options: options.Index().SetName("idx_feed_url"),
					Keys:    bson.D{{Key: "feed_url", Value: 1}},
				},
			},
			indexNames: []string{
				"idx_url_unique",
				"idx_completed_score",
				"idx_simhash_bucket",
				"idx_domain",
				"idx_crawled_at",
				"idx_feed_url",
			},
		},
		{
			name: "crawl_feeds",
			indexes: []mongo.IndexModel{
				{
					Options: options.Index().
						SetName("idx_feed_url_unique").
						SetUnique(true),
					Keys: bson.D{{Key: "url", Value: 1}},
				},
				{
					Options: options.Index().SetName("idx_enabled"),
					Keys:    bson.D{{Key: "enabled", Value: 1}},
				},
				{
					Options: options.Index().SetName("idx_last_fetch"),
					Keys:    bson.D{{Key: "last_fetch_at", Value: 1}},
				},
			},
			indexNames: []string{
				"idx_feed_url_unique",
				"idx_enabled",
				"idx_last_fetch",
			},
		},
	}

	return createIndexes(ctx, db, collections)
}

// Run004TrendingIndexes creates indexes for the trending-service collections.
func Run004TrendingIndexes(ctx context.Context, db *mongo.Database) error {
	collections := []struct {
		name       string
		indexes    []mongo.IndexModel
		indexNames []string
	}{
		{
			name: "trending_topics",
			indexes: []mongo.IndexModel{
				{
					Options: options.Index().SetName("idx_score"),
					Keys:    bson.D{{Key: "score", Value: -1}},
				},
				{
					Options: options.Index().SetName("idx_category_score"),
					Keys: bson.D{
						{Key: "category", Value: 1},
						{Key: "score", Value: -1},
					},
				},
				{
					Options: options.Index().SetName("idx_region"),
					Keys:    bson.D{{Key: "region", Value: 1}},
				},
				{
					Options: options.Index().SetName("idx_timestamp"),
					Keys:    bson.D{{Key: "timestamp", Value: -1}},
				},
				{
					Options: options.Index().SetName("idx_region_timestamp_score"),
					Keys: bson.D{
						{Key: "region", Value: 1},
						{Key: "timestamp", Value: -1},
						{Key: "score", Value: -1},
					},
				},
			},
			indexNames: []string{
				"idx_score",
				"idx_category_score",
				"idx_region",
				"idx_timestamp",
				"idx_region_timestamp_score",
			},
		},
		{
			name: "trending_sources",
			indexes: []mongo.IndexModel{
				{
					Options: options.Index().
						SetName("idx_source_name_unique").
						SetUnique(true),
					Keys: bson.D{{Key: "name", Value: 1}},
				},
				{
					Options: options.Index().SetName("idx_enabled"),
					Keys:    bson.D{{Key: "enabled", Value: 1}},
				},
			},
			indexNames: []string{
				"idx_source_name_unique",
				"idx_enabled",
			},
		},
		{
			name: "trending_aggregations",
			indexes: []mongo.IndexModel{
				{
					Options: options.Index().
						SetName("idx_agg_unique").
						SetUnique(true),
					Keys: bson.D{
						{Key: "region", Value: 1},
						{Key: "category", Value: 1},
						{Key: "time_window", Value: 1},
					},
				},
			},
			indexNames: []string{
				"idx_agg_unique",
			},
		},
	}

	return createIndexes(ctx, db, collections)
}

// Run005NotificationReadIndexes creates indexes for markAsRead + unreadCount queries.
// These indexes optimize:
// - CountUnread: COUNT where user_id=X AND read=false
// - MarkAsRead: UPDATE _id=X where user_id=X
func Run005NotificationReadIndexes(ctx context.Context, db *mongo.Database) error {
	collections := []struct {
		name       string
		indexes    []mongo.IndexModel
		indexNames []string
	}{
		{
			name: "notifications",
			indexes: []mongo.IndexModel{
				// Optimizes CountUnread(ctx, userID) + List filtering by user_id
				{
					Options: options.Index().SetName("idx_user_id"),
					Keys:    bson.D{{Key: "user_id", Value: 1}},
				},
				// Optimizes CountUnread: COUNT WHERE user_id=X AND read=false
				{
					Options: options.Index().SetName("idx_user_id_read"),
					Keys: bson.D{
						{Key: "user_id", Value: 1},
						{Key: "read", Value: 1},
					},
				},
				// Optimizes MarkAsRead: UPDATE WHERE _id=X AND user_id=X
				// (already covered by primary key, but explicit for compound queries)
				{
					Options: options.Index().SetName("idx_user_id_created"),
					Keys: bson.D{
						{Key: "user_id", Value: 1},
						{Key: "created_at", Value: -1},
					},
				},
			},
			indexNames: []string{
				"idx_user_id",
				"idx_user_id_read",
				"idx_user_id_created",
			},
		},
	}

	return createIndexes(ctx, db, collections)
}

// createIndexes safely creates indexes, ignoring the "already exists" error.
func createIndexes(ctx context.Context, db *mongo.Database, collections []struct {
	name       string
	indexes    []mongo.IndexModel
	indexNames []string
}) error {
	for _, coll := range collections {
		collection := db.Collection(coll.name)
		for i, idx := range coll.indexes {
			ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			_, err := collection.Indexes().CreateOne(ctx, idx)
			cancel()
			if err != nil {
				if !isIndexExistsError(err) {
					var idxName string
					if i < len(coll.indexNames) {
						idxName = coll.indexNames[i]
					}
					return fmt.Errorf("create index %s on %s: %w", idxName, coll.name, err)
				}
			}
		}
	}
	return nil
}

// isIndexExistsError checks if the error is a MongoDB "index already exists" error.
func isIndexExistsError(err error) bool {
	if err == nil {
		return false
	}
	return containsString(err.Error(), "index already exists") ||
		containsString(err.Error(), "E11000")
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
