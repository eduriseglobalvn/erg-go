// Package repository provides MongoDB data access for the reviews module.
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"erg.ninja/internal/modules/reviews/entities"
	"erg.ninja/pkg/database"
)

var ErrReviewNotFound = errors.New("review not found")

// Repository provides MongoDB data access for reviews.
type Repository struct {
	coll *mongo.Collection
}

// NewRepository creates a new reviews repository.
func NewRepository(mongo *database.MongoClient) *Repository {
	return &Repository{
		coll: mongo.Collection(entities.ReviewCollection),
	}
}

// ─── Create ───────────────────────────────────────────────────────────────────

// Create inserts a new review.
func (r *Repository) Create(ctx context.Context, review *entities.Review) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if review.ID.IsZero() {
		review.ID = bson.NewObjectID()
	}
	review.CreatedAt = time.Now().UTC()
	review.UpdatedAt = review.CreatedAt
	review.HelpfulCount = 0

	_, err := r.coll.InsertOne(ctx, review)
	if err != nil {
		return fmt.Errorf("reviews.Create: %w", err)
	}
	return nil
}

// ─── Read ──────────────────────────────────────────────────────────────────────

// GetByID retrieves a review by its ObjectID string.
func (r *Repository) GetByID(ctx context.Context, id string) (*entities.Review, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	objID, ok := database.ParseObjectID(id)
	if !ok {
		return nil, ErrReviewNotFound
	}

	var review entities.Review
	err := r.coll.FindOne(ctx, bson.M{"_id": objID}).Decode(&review)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrReviewNotFound
		}
		return nil, fmt.Errorf("reviews.GetByID: %w", err)
	}
	return &review, nil
}

// ListParams controls public review listing.
type ListParams struct {
	TargetID   string
	TargetType string
	Page       int
	Limit      int
	Sort       string // newest | oldest | highest | lowest
}

// List returns paginated approved reviews.
func (r *Repository) List(ctx context.Context, p ListParams) ([]*entities.Review, int64, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if p.Limit <= 0 || p.Limit > 100 {
		p.Limit = 10
	}
	if p.Page < 1 {
		p.Page = 1
	}

	filter := bson.M{"status": entities.ReviewStatusApproved}
	if p.TargetID != "" {
		filter["target_id"] = p.TargetID
	}
	if p.TargetType != "" {
		filter["target_type"] = p.TargetType
	}

	order, err := sortOrder(p.Sort)
	if err != nil {
		order = -1 // newest first default
	}

	skip := int64((p.Page - 1) * p.Limit)
	opts := options.Find().
		SetSort(bson.D{{Key: "is_featured", Value: -1}, {Key: "created_at", Value: order}}).
		SetSkip(skip).
		SetLimit(int64(p.Limit))

	total, err := r.coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("reviews.List count: %w", err)
	}

	cur, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("reviews.List: %w", err)
	}
	defer cur.Close(ctx)

	var reviews []*entities.Review
	if err := cur.All(ctx, &reviews); err != nil {
		return nil, 0, fmt.Errorf("reviews.List decode: %w", err)
	}
	return reviews, total, nil
}

// AdminListParams controls admin review listing.
type AdminListParams struct {
	Status     string
	TargetType string
	TargetID   string
	Page       int
	Limit      int
	Sort       string
}

// AdminList returns paginated reviews with optional filters (all statuses).
func (r *Repository) AdminList(ctx context.Context, p AdminListParams) ([]*entities.Review, int64, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if p.Limit <= 0 || p.Limit > 100 {
		p.Limit = 10
	}
	if p.Page < 1 {
		p.Page = 1
	}

	filter := bson.M{}
	if p.Status != "" {
		filter["status"] = p.Status
	}
	if p.TargetType != "" {
		filter["target_type"] = p.TargetType
	}
	if p.TargetID != "" {
		filter["target_id"] = p.TargetID
	}

	order := -1 // newest first
	if p.Sort == "oldest" {
		order = 1
	}

	skip := int64((p.Page - 1) * p.Limit)
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: order}}).
		SetSkip(skip).
		SetLimit(int64(p.Limit))

	total, err := r.coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("reviews.AdminList count: %w", err)
	}

	cur, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("reviews.AdminList: %w", err)
	}
	defer cur.Close(ctx)

	var reviews []*entities.Review
	if err := cur.All(ctx, &reviews); err != nil {
		return nil, 0, fmt.Errorf("reviews.AdminList decode: %w", err)
	}
	return reviews, total, nil
}

// AggregateStats computes rating statistics for a target.
func (r *Repository) AggregateStats(ctx context.Context, targetID, targetType string) (avg float64, count int64, dist map[string]int64, err error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	match := bson.M{"status": entities.ReviewStatusApproved}
	if targetID != "" {
		match["target_id"] = targetID
	}
	if targetType != "" {
		match["target_type"] = targetType
	}

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: match}},
		{{Key: "$group", Value: bson.M{
			"_id": nil,
			"avg": bson.M{"$avg": "$rating"},
			"cnt": bson.M{"$sum": 1},
			"r1":  bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$rating", 1}}, 1, 0}}},
			"r2":  bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$rating", 2}}, 1, 0}}},
			"r3":  bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$rating", 3}}, 1, 0}}},
			"r4":  bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$rating", 4}}, 1, 0}}},
			"r5":  bson.M{"$sum": bson.M{"$cond": bson.A{bson.M{"$eq": bson.A{"$rating", 5}}, 1, 0}}},
		}}},
	}

	cur, err := r.coll.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("reviews.AggregateStats: %w", err)
	}
	defer cur.Close(ctx)

	if !cur.Next(ctx) {
		// No reviews found — return zero stats
		return 0, 0, map[string]int64{"1": 0, "2": 0, "3": 0, "4": 0, "5": 0}, nil
	}

	var result struct {
		Avg float64 `bson:"avg"`
		Cnt int64   `bson:"cnt"`
		R1  int64   `bson:"r1"`
		R2  int64   `bson:"r2"`
		R3  int64   `bson:"r3"`
		R4  int64   `bson:"r4"`
		R5  int64   `bson:"r5"`
	}
	if err := cur.Decode(&result); err != nil {
		return 0, 0, nil, fmt.Errorf("reviews.AggregateStats decode: %w", err)
	}

	dist = map[string]int64{
		"1": result.R1,
		"2": result.R2,
		"3": result.R3,
		"4": result.R4,
		"5": result.R5,
	}
	return result.Avg, result.Cnt, dist, nil
}

// ─── Update ────────────────────────────────────────────────────────────────────

// UpdateFields updates specific fields on a review.
func (r *Repository) UpdateFields(ctx context.Context, id string, updates map[string]any) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	objID, ok := database.ParseObjectID(id)
	if !ok {
		return ErrReviewNotFound
	}

	updates["updated_at"] = time.Now().UTC()
	result, err := r.coll.UpdateOne(ctx, bson.M{"_id": objID}, bson.M{"$set": updates})
	if err != nil {
		return fmt.Errorf("reviews.UpdateFields: %w", err)
	}
	if result.MatchedCount == 0 {
		return ErrReviewNotFound
	}
	return nil
}

// IncrementHelpful increments the helpful_count for a review.
func (r *Repository) IncrementHelpful(ctx context.Context, id string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	objID, ok := database.ParseObjectID(id)
	if !ok {
		return ErrReviewNotFound
	}

	result, err := r.coll.UpdateOne(ctx, bson.M{"_id": objID}, bson.M{
		"$inc": bson.M{"helpful_count": 1},
		"$set": bson.M{"updated_at": time.Now().UTC()},
	})
	if err != nil {
		return fmt.Errorf("reviews.IncrementHelpful: %w", err)
	}
	if result.MatchedCount == 0 {
		return ErrReviewNotFound
	}
	return nil
}

// ─── Helpers ───────────────────────────────────────────────────────────────────

// sortOrder returns MongoDB sort value from a sort string.
func sortOrder(s string) (int, error) {
	switch s {
	case "oldest":
		return 1, nil
	case "highest":
		return -1, nil // sorted by rating desc; handled specially in query
	case "lowest":
		return 1, nil // sorted by rating asc; handled specially in query
	default: // newest
		return -1, nil
	}
}
