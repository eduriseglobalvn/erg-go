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

	"erg.ninja/internal/modules/analytics/entities"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
)

// VisitRepository provides data access for visits.
type VisitRepository struct {
	coll *mongo.Collection
	log  *logger.Logger
}

// NewVisitRepository creates a new visit repository.
func NewVisitRepository(mongo *database.MongoClient, log *logger.Logger) *VisitRepository {
	return &VisitRepository{
		coll: mongo.Collection(entities.VisitCollection),
		log:  log,
	}
}

// Create inserts a new visit record.
func (r *VisitRepository) Create(ctx context.Context, v *entities.Visit) error {
	v.CreatedAt = time.Now().UTC()
	v.UpdatedAt = v.CreatedAt
	if v.DurationSeconds == 0 {
		v.DurationSeconds = 0
	}
	if v.PageViews == 0 {
		v.PageViews = 1
	}
	_, err := r.coll.InsertOne(ctx, v)
	if err != nil {
		return fmt.Errorf("analytics.visit.Create: %w", err)
	}
	return nil
}

// GetByID retrieves a visit by its ID.
func (r *VisitRepository) GetByID(ctx context.Context, id string) (*entities.Visit, error) {
	var doc bson.M
	err := r.coll.FindOne(ctx, analyticsIDFilter(id)).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("analytics.visit.GetByID: %w", err)
	}
	return normalizeVisitDoc(doc), nil
}

// GetBySessionID retrieves a visit by session ID.
func (r *VisitRepository) GetBySessionID(ctx context.Context, sessionID string) (*entities.Visit, error) {
	filter := bson.M{
		"$or": []bson.M{
			{"session_id": sessionID},
			{"sessionInternalId": sessionID},
		},
	}
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	cur, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("analytics.visit.GetBySessionID: %w", err)
	}
	defer cur.Close(ctx)

	var results []*entities.Visit
	for cur.Next(ctx) {
		var doc bson.M
		if err := cur.Decode(&doc); err != nil {
			continue
		}
		if visit := normalizeVisitDoc(doc); visit != nil {
			results = append(results, visit)
		}
	}
	if len(results) == 0 {
		return nil, nil
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.After(results[j].CreatedAt)
	})
	return results[0], nil
}

// UpdateDuration updates the duration and page views for a visit.
func (r *VisitRepository) UpdateDuration(ctx context.Context, id string, durationSeconds, pageViews int) error {
	_, err := r.coll.UpdateOne(ctx, bson.M{"_id": id}, bson.M{
		"$set": bson.M{
			"duration_seconds": durationSeconds,
			"page_views":       pageViews,
			"updated_at":       time.Now().UTC(),
		},
	})
	if err != nil {
		return fmt.Errorf("analytics.visit.UpdateDuration: %w", err)
	}
	return nil
}

// UpdateUserID associates a user ID with a visit.
func (r *VisitRepository) UpdateUserID(ctx context.Context, id string, userID int64) error {
	_, err := r.coll.UpdateOne(ctx, bson.M{"_id": id}, bson.M{
		"$set": bson.M{
			"user_id":    userID,
			"updated_at": time.Now().UTC(),
		},
	})
	if err != nil {
		return fmt.Errorf("analytics.visit.UpdateUserID: %w", err)
	}
	return nil
}

// FindByDateRange retrieves all visits within a date range.
func (r *VisitRepository) FindByDateRange(ctx context.Context, from, to time.Time) ([]*entities.Visit, error) {
	filter := analyticsDateRangeFilter(from, to)
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	cur, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("analytics.visit.FindByDateRange: %w", err)
	}
	defer cur.Close(ctx)

	var results []*entities.Visit
	for cur.Next(ctx) {
		var doc bson.M
		if err := cur.Decode(&doc); err != nil {
			continue
		}
		if visit := normalizeVisitDoc(doc); visit != nil {
			results = append(results, visit)
		}
	}
	if err := cur.Err(); err != nil {
		return nil, fmt.Errorf("analytics.visit.FindByDateRange decode: %w", err)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.After(results[j].CreatedAt)
	})
	return results, nil
}

// CountByDateRange returns the total number of visits in a date range.
func (r *VisitRepository) CountByDateRange(ctx context.Context, from, to time.Time) (int64, error) {
	filter := analyticsDateRangeFilter(from, to)
	count, err := r.coll.CountDocuments(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("analytics.visit.CountByDateRange: %w", err)
	}
	return count, nil
}

// AggregateVisitorStats groups visits by date and device type.
func (r *VisitRepository) AggregateVisitorStats(ctx context.Context, from, to time.Time) (map[string]map[string]int, error) {
	visits, err := r.FindByDateRange(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("analytics.visit.AggregateVisitorStats: %w", err)
	}

	stats := make(map[string]map[string]int)
	for _, visit := range visits {
		dateKey := visit.CreatedAt.Format("2006-01-02")
		device := visit.DeviceType
		if device == "" {
			device = "desktop"
		}
		if _, ok := stats[dateKey]; !ok {
			stats[dateKey] = make(map[string]int)
		}
		stats[dateKey][device]++
	}
	return stats, nil
}

// IncrementPageViews increments the page views counter for a visit.
func (r *VisitRepository) IncrementPageViews(ctx context.Context, id string) error {
	_, err := r.coll.UpdateOne(ctx, bson.M{"_id": id}, bson.M{
		"$inc": bson.M{"page_views": 1},
		"$set": bson.M{"updated_at": time.Now().UTC()},
	})
	if err != nil {
		return fmt.Errorf("analytics.visit.IncrementPageViews: %w", err)
	}
	return nil
}
