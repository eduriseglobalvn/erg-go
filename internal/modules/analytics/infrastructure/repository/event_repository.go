// Package repository provides MongoDB data access for the analytics module.
package repository

import (
	"context"
	"fmt"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"erg.ninja/internal/modules/analytics/domain/entity"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
)

// EventRepository provides data access for analytics events.
type EventRepository struct {
	coll *mongo.Collection
	log  *logger.Logger
}

// NewEventRepository creates a new event repository.
func NewEventRepository(mongo *database.MongoClient, log *logger.Logger) *EventRepository {
	return &EventRepository{
		coll: mongo.Collection(entities.EventCollection),
		log:  log,
	}
}

// Create inserts a new event.
func (r *EventRepository) Create(ctx context.Context, e *entities.Event) error {
	e.CreatedAt = time.Now().UTC()
	_, err := r.coll.InsertOne(ctx, e)
	if err != nil {
		return fmt.Errorf("analytics.event.Create: %w", err)
	}
	return nil
}

// BatchCreate inserts multiple events efficiently.
func (r *EventRepository) BatchCreate(ctx context.Context, events []*entities.Event) error {
	if len(events) == 0 {
		return nil
	}
	docs := make([]any, len(events))
	now := time.Now().UTC()
	for i, e := range events {
		if e.CreatedAt.IsZero() {
			e.CreatedAt = now
		}
		docs[i] = e
	}
	_, err := r.coll.InsertMany(ctx, docs)
	if err != nil {
		return fmt.Errorf("analytics.event.BatchCreate: %w", err)
	}
	return nil
}

// GetBySessionID retrieves all events for a given session ID.
func (r *EventRepository) GetBySessionID(ctx context.Context, sessionID string) ([]*entities.Event, error) {
	filter := bson.M{
		"$or": []bson.M{
			{"session_id": sessionID},
			{"sessionInternalId": sessionID},
		},
	}
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: 1}})
	cur, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("analytics.event.GetBySessionID: %w", err)
	}
	defer cur.Close(ctx)

	var results []*entities.Event
	for cur.Next(ctx) {
		var doc bson.M
		if err := cur.Decode(&doc); err != nil {
			continue
		}
		if event := normalizeEventDoc(doc); event != nil {
			results = append(results, event)
		}
	}
	if err := cur.Err(); err != nil {
		return nil, fmt.Errorf("analytics.event.GetBySessionID decode: %w", err)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.Before(results[j].CreatedAt)
	})
	return results, nil
}

// FindByDateRange retrieves all events within a date range.
func (r *EventRepository) FindByDateRange(ctx context.Context, from, to time.Time) ([]*entities.Event, error) {
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	cur, err := r.coll.Find(ctx, analyticsDateRangeFilter(from, to), opts)
	if err != nil {
		return nil, fmt.Errorf("analytics.event.FindByDateRange: %w", err)
	}
	defer cur.Close(ctx)

	var results []*entities.Event
	for cur.Next(ctx) {
		var doc bson.M
		if err := cur.Decode(&doc); err != nil {
			continue
		}
		if event := normalizeEventDoc(doc); event != nil {
			results = append(results, event)
		}
	}
	if err := cur.Err(); err != nil {
		return nil, fmt.Errorf("analytics.event.FindByDateRange decode: %w", err)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.After(results[j].CreatedAt)
	})
	return results, nil
}

// AggregateEventStats groups events by event type.
func (r *EventRepository) AggregateEventStats(ctx context.Context, from, to time.Time) (map[string]int, error) {
	events, err := r.FindByDateRange(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("analytics.event.AggregateEventStats: %w", err)
	}

	stats := make(map[string]int)
	for _, event := range events {
		name := event.EventType
		if name == "" {
			name = event.EventName
		}
		if name == "" {
			continue
		}
		stats[name]++
	}
	return stats, nil
}

// UpdateUserID associates a user ID with all events for a session.
func (r *EventRepository) UpdateUserID(ctx context.Context, sessionID string, userID int64) error {
	_, err := r.coll.UpdateMany(ctx,
		bson.M{
			"$or": []bson.M{
				{"session_id": sessionID},
				{"sessionInternalId": sessionID},
			},
		},
		bson.M{"$set": bson.M{"user_id": userID, "userId": userID}},
	)
	if err != nil {
		return fmt.Errorf("analytics.event.UpdateUserID: %w", err)
	}
	return nil
}
