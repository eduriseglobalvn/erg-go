// Package repository provides MongoDB data access for the audit module.
package repository

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"erg.ninja/internal/modules/audit/entities"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
)

// Repository provides data access for audit logs.
type Repository struct {
	coll *mongo.Collection
	log  *logger.Logger
}

// NewRepository creates a new audit repository.
func NewRepository(mongo *database.MongoClient, log *logger.Logger) *Repository {
	return &Repository{
		coll: mongo.Collection(entities.AuditLogCollection),
		log:  log,
	}
}

// ─── Audit Log CRUD ─────────────────────────────────────────────────────────

// Create inserts a new audit log entry.
func (r *Repository) Create(ctx context.Context, log *entities.AuditLog) error {
	if log.ID.IsZero() {
		log.ID = bson.NewObjectID()
	}
	log.Timestamp = time.Now().UTC()
	_, err := r.coll.InsertOne(ctx, log)
	if err != nil {
		return fmt.Errorf("audit.repo.Create: %w", err)
	}
	return nil
}

// GetByID retrieves a single audit log by ID.
func (r *Repository) GetByID(ctx context.Context, id string) (*entities.AuditLog, error) {
	var entry entities.AuditLog
	err := r.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&entry)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("audit.repo.GetByID: %w", err)
	}
	return &entry, nil
}

// ListQuery holds filter options for listing audit logs.
type ListQuery struct {
	Page         int
	Limit        int
	Action       string
	UserID       string
	ResourceType string
	StartDate    *time.Time
	EndDate      *time.Time
}

// List returns a paginated list of audit logs matching the given filters.
func (r *Repository) List(ctx context.Context, q ListQuery) ([]*entities.AuditLog, int64, error) {
	filter := bson.M{}

	if q.Action != "" {
		filter["action"] = q.Action
	}
	if q.UserID != "" {
		filter["user_id"] = q.UserID
	}
	if q.ResourceType != "" {
		filter["resource_type"] = q.ResourceType
	}
	if q.StartDate != nil || q.EndDate != nil {
		tsFilter := bson.M{}
		if q.StartDate != nil {
			tsFilter["$gte"] = *q.StartDate
		}
		if q.EndDate != nil {
			tsFilter["$lte"] = *q.EndDate
		}
		filter["timestamp"] = tsFilter
	}

	// Pagination.
	page := q.Page
	if page < 1 {
		page = 1
	}
	limit := q.Limit
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	skip := int64((page - 1) * limit)

	// Count total.
	total, err := r.coll.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("audit.repo.List.Count: %w", err)
	}

	// Find with pagination and sort descending by timestamp.
	opts := options.Find().
		SetSort(bson.D{{Key: "timestamp", Value: -1}}).
		SetSkip(skip).
		SetLimit(int64(limit))

	cur, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("audit.repo.List.Find: %w", err)
	}
	defer cur.Close(ctx)

	var logs []*entities.AuditLog
	if err := cur.All(ctx, &logs); err != nil {
		return nil, 0, fmt.Errorf("audit.repo.List.Decode: %w", err)
	}

	return logs, total, nil
}
