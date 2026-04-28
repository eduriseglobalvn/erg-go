// Package repository provides MongoDB data access for the analytics module.
package repository

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"erg.ninja/internal/modules/analytics/entities"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
)

// FirebaseSyncRepository provides data access for Firebase sync events.
type FirebaseSyncRepository struct {
	coll *mongo.Collection
	log  *logger.Logger
}

// NewFirebaseSyncRepository creates a new Firebase sync repository.
func NewFirebaseSyncRepository(mongo *database.MongoClient, log *logger.Logger) *FirebaseSyncRepository {
	return &FirebaseSyncRepository{
		coll: mongo.Collection(entities.FirebaseSyncEventCollection),
		log:  log,
	}
}

// BatchUpsert inserts multiple Firebase events, skipping duplicates.
// A duplicate is detected by the composite key (event_name, app_instance_id, received_at).
func (r *FirebaseSyncRepository) BatchUpsert(ctx context.Context, events []*entities.FirebaseSyncEvent) (inserted int, err error) {
	if len(events) == 0 {
		return 0, nil
	}

	var count int
	now := time.Now().UTC()
	upsertOpts := options.UpdateOne().SetUpsert(true)

	for _, e := range events {
		if e.CreatedAt.IsZero() {
			e.CreatedAt = now
		}
		if e.ProcessedAt.IsZero() {
			e.ProcessedAt = now
		}

		filter := bson.M{
			"event_name":      e.EventName,
			"app_instance_id": e.AppInstanceID,
			"received_at":     e.ReceivedAt,
		}

		update := bson.M{"$setOnInsert": e}
		result, err := r.coll.UpdateOne(ctx, filter, update, upsertOpts)
		if err != nil {
			r.log.ErrorContext(ctx).Err(err).
				Str("event", e.EventName).
				Str("instance", e.AppInstanceID).
				Msg("analytics.fb_sync.BatchUpsert failed")
			continue
		}
		// UpsertedCount == 1 means it was a new insert (not duplicate).
		if result != nil && result.UpsertedCount == 1 {
			count++
		}
	}

	return count, nil
}

// CountByDateRange returns the total number of Firebase sync events in a date range.
func (r *FirebaseSyncRepository) CountByDateRange(ctx context.Context, from, to time.Time) (int64, error) {
	filter := bson.M{
		"received_at": bson.M{"$gte": from, "$lte": to},
	}
	count, err := r.coll.CountDocuments(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("analytics.fb_sync.CountByDateRange: %w", err)
	}
	return count, nil
}

// FindByDateRange retrieves Firebase sync events within a date range.
func (r *FirebaseSyncRepository) FindByDateRange(ctx context.Context, from, to time.Time, limit int64) ([]*entities.FirebaseSyncEvent, error) {
	opts := options.Find().
		SetSort(bson.D{{Key: "received_at", Value: -1}}).
		SetLimit(limit)

	cur, err := r.coll.Find(ctx, bson.M{
		"received_at": bson.M{"$gte": from, "$lte": to},
	}, opts)
	if err != nil {
		return nil, fmt.Errorf("analytics.fb_sync.FindByDateRange: %w", err)
	}
	defer cur.Close(ctx)

	var results []*entities.FirebaseSyncEvent
	if err := cur.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("analytics.fb_sync.FindByDateRange decode: %w", err)
	}
	return results, nil
}

// AggregateByEventName groups Firebase sync events by event name within a date range.
func (r *FirebaseSyncRepository) AggregateByEventName(ctx context.Context, from, to time.Time) (map[string]int64, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"received_at": bson.M{"$gte": from, "$lte": to}}}},
		{{Key: "$group", Value: bson.M{
			"_id":   "$event_name",
			"count": bson.M{"$sum": 1},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "count", Value: -1}}}},
	}

	cur, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("analytics.fb_sync.AggregateByEventName: %w", err)
	}
	defer cur.Close(ctx)

	stats := make(map[string]int64)
	for cur.Next(ctx) {
		var result struct {
			ID    string `bson:"_id"`
			Count int64  `bson:"count"`
		}
		if err := cur.Decode(&result); err != nil {
			continue
		}
		stats[result.ID] = result.Count
	}
	return stats, nil
}
