package repository

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"erg.ninja/internal/modules/documents/entities"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
)

var ErrDocumentNotFound = errors.New("documents: not found")

// Repository provides MongoDB data access for documents.
type Repository struct {
	coll *mongo.Collection
	log  *logger.Logger
}

// RepositoryOption configures the Repository.
type RepositoryOption func(*Repository)

// WithDocumentsLogger sets the logger.
func WithDocumentsLogger(log *logger.Logger) RepositoryOption {
	return func(r *Repository) { r.log = log }
}

// NewRepository creates a new documents repository.
func NewRepository(mongo *database.MongoClient, opts ...RepositoryOption) *Repository {
	r := &Repository{
		coll: mongo.Collection(entities.DocumentCollection),
		log:  logger.NoOp(),
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

// FindByID returns a document by its ID.
func (r *Repository) FindByID(ctx context.Context, tenantID, id string) (*entities.Document, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	filter := bson.M{"_id": id, "tenant_id": tenantID}
	return r.findOne(ctx, filter)
}

// FindByIDForUser returns a document by ID scoped to the owner unless the caller is an admin.
func (r *Repository) FindByIDForUser(ctx context.Context, tenantID, id, userID string, isAdmin bool) (*entities.Document, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	filter := bson.M{"_id": id, "tenant_id": tenantID}
	if !isAdmin {
		filter["uploaded_by"] = userID
	}
	return r.findOne(ctx, filter)
}

func (r *Repository) findOne(ctx context.Context, filter bson.M) (*entities.Document, error) {
	var doc entities.Document
	err := r.coll.FindOne(ctx, filter).Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrDocumentNotFound
		}
		return nil, err
	}
	return &doc, nil
}

// Create inserts a new document record.
func (r *Repository) Create(ctx context.Context, doc *entities.Document) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if doc.ID == "" {
		doc.ID = database.NewID()
	}
	doc.CreatedAt = time.Now()
	doc.UpdatedAt = doc.CreatedAt

	_, err := r.coll.InsertOne(ctx, doc)
	return err
}

// UpdateStatus atomically updates the status field.
func (r *Repository) UpdateStatus(ctx context.Context, tenantID, id, status string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	filter := bson.M{"_id": id, "tenant_id": tenantID}
	update := bson.M{"$set": bson.M{"status": status, "updated_at": time.Now()}}

	result, err := r.coll.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return ErrDocumentNotFound
	}
	return nil
}

// UpdateFields updates only the non-nil fields of a document.
func (r *Repository) UpdateFields(ctx context.Context, tenantID, id string, updates map[string]any) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	filter := bson.M{"_id": id, "tenant_id": tenantID}
	return r.updateFields(ctx, filter, updates)
}

// UpdateFieldsForUser updates a document scoped to the owner unless the caller is an admin.
func (r *Repository) UpdateFieldsForUser(ctx context.Context, tenantID, id, userID string, isAdmin bool, updates map[string]any) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	filter := bson.M{"_id": id, "tenant_id": tenantID}
	if !isAdmin {
		filter["uploaded_by"] = userID
	}
	return r.updateFields(ctx, filter, updates)
}

func (r *Repository) updateFields(ctx context.Context, filter bson.M, updates map[string]any) error {
	updates["updated_at"] = time.Now()
	update := bson.M{"$set": updates}

	result, err := r.coll.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return ErrDocumentNotFound
	}
	return nil
}

// Delete removes a document by tenant + ID.
func (r *Repository) Delete(ctx context.Context, tenantID, id string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	filter := bson.M{"_id": id, "tenant_id": tenantID}
	return r.deleteOne(ctx, filter)
}

// DeleteForUser deletes a document scoped to the owner unless the caller is an admin.
func (r *Repository) DeleteForUser(ctx context.Context, tenantID, id, userID string, isAdmin bool) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	filter := bson.M{"_id": id, "tenant_id": tenantID}
	if !isAdmin {
		filter["uploaded_by"] = userID
	}
	return r.deleteOne(ctx, filter)
}

func (r *Repository) deleteOne(ctx context.Context, filter bson.M) error {
	result, err := r.coll.DeleteOne(ctx, filter)
	if err != nil {
		return err
	}
	if result.DeletedCount == 0 {
		return ErrDocumentNotFound
	}
	return nil
}

// List returns documents for a tenant with cursor pagination.
func (r *Repository) List(ctx context.Context, tenantID string, cursor string, limit int) ([]entities.Document, string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	filter := bson.M{"tenant_id": tenantID}
	return r.list(ctx, filter, cursor, limit)
}

// ListForUser lists documents scoped to the owner unless the caller is an admin.
func (r *Repository) ListForUser(ctx context.Context, tenantID, userID string, isAdmin bool, cursor string, limit int) ([]entities.Document, string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	filter := bson.M{"tenant_id": tenantID}
	if !isAdmin {
		filter["uploaded_by"] = userID
	}
	return r.list(ctx, filter, cursor, limit)
}

func (r *Repository) list(ctx context.Context, filter bson.M, cursor string, limit int) ([]entities.Document, string, error) {
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(limit + 1))

	if cursor != "" {
		objID, ok := database.ParseObjectID(cursor)
		if ok {
			filter["_id"] = bson.M{"$lt": objID}
		}
	}

	cur, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, "", err
	}
	defer cur.Close(ctx)

	var docs []entities.Document
	if err := cur.All(ctx, &docs); err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(docs) > limit {
		docs = docs[:limit]
		nextCursor = docs[len(docs)-1].ID
	}
	return docs, nextCursor, nil
}

// Count returns the total number of documents for a tenant.
func (r *Repository) Count(ctx context.Context, tenantID string) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return r.coll.CountDocuments(ctx, bson.M{"tenant_id": tenantID})
}

// CountForUser counts documents scoped to the owner unless the caller is an admin.
func (r *Repository) CountForUser(ctx context.Context, tenantID, userID string, isAdmin bool) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	filter := bson.M{"tenant_id": tenantID}
	if !isAdmin {
		filter["uploaded_by"] = userID
	}
	return r.coll.CountDocuments(ctx, filter)
}
