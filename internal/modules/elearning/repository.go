package elearning

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"erg.ninja/internal/modules/elearning/entities"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
)

// Repository handles all MongoDB operations for the elearning module.
type Repository struct {
	log        *logger.Logger
	categories *mongo.Collection
	levels     *mongo.Collection
	units      *mongo.Collection
}

// RepositoryOption configures the Repository.
type RepositoryOption func(*Repository)

// WithRepositoryLogger sets the logger.
func WithRepositoryLogger(log *logger.Logger) RepositoryOption {
	return func(r *Repository) { r.log = log }
}

// NewRepository creates a new elearning repository.
func NewRepository(mongoClient *database.MongoClient, opts ...RepositoryOption) *Repository {
	r := &Repository{
		log:        logger.NoOp(),
		categories: mongoClient.Collection(entities.ElearningCategoryCollection),
		levels:     mongoClient.Collection(entities.ElearningLevelCollection),
		units:      mongoClient.Collection(entities.ElearningUnitCollection),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// EnsureIndexes creates MongoDB indexes for all three collections.
func (r *Repository) EnsureIndexes(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// categories: slug unique + tenant_id + is_active
	_, err := r.categories.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "slug", Value: 1}, {Key: "tenant_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{Keys: bson.D{{Key: "tenant_id", Value: 1}}},
		{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "is_active", Value: 1}}},
	})
	if err != nil {
		return fmt.Errorf("elearning.repo.EnsureIndexes categories: %w", err)
	}

	// levels: category_id + tenant_id
	_, err = r.levels.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "slug", Value: 1}, {Key: "tenant_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "category_id", Value: 1}}},
	})
	if err != nil {
		return fmt.Errorf("elearning.repo.EnsureIndexes levels: %w", err)
	}

	// units: level_id + tenant_id
	_, err = r.units.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "slug", Value: 1}, {Key: "tenant_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "level_id", Value: 1}}},
	})
	if err != nil {
		return fmt.Errorf("elearning.repo.EnsureIndexes units: %w", err)
	}

	return nil
}

// ─── Categories ─────────────────────────────────────────────────────────────

// ListCategories returns categories filtered by tenant, optionally active only.
func (r *Repository) ListCategories(ctx context.Context, tenantID string, activeOnly bool) ([]*entities.ElearningCategory, error) {
	filter := entities.CatFilter(tenantID, activeOnly)
	cursor, err := r.categories.Find(ctx, filter,
		options.Find().SetSort(bson.D{{Key: "order", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("elearning.repo.ListCategories: %w", err)
	}
	defer cursor.Close(ctx)

	var out []*entities.ElearningCategory
	if err := cursor.All(ctx, &out); err != nil {
		return nil, fmt.Errorf("elearning.repo.ListCategories decode: %w", err)
	}
	return out, nil
}

// GetCategoryBySlug returns a single category by slug.
func (r *Repository) GetCategoryBySlug(ctx context.Context, tenantID, slug string) (*entities.ElearningCategory, error) {
	var cat entities.ElearningCategory
	err := r.categories.FindOne(ctx, bson.M{
		"slug":      slug,
		"tenant_id": tenantID,
	}).Decode(&cat)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("elearning.repo.GetCategoryBySlug: %w", err)
	}
	return &cat, nil
}

// GetCategoryByID returns a single category by ID.
func (r *Repository) GetCategoryByID(ctx context.Context, tenantID, id string) (*entities.ElearningCategory, error) {
	var cat entities.ElearningCategory
	err := r.categories.FindOne(ctx, bson.M{
		"_id":       id,
		"tenant_id": tenantID,
	}).Decode(&cat)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("elearning.repo.GetCategoryByID: %w", err)
	}
	return &cat, nil
}

// CreateCategory inserts a new category.
func (r *Repository) CreateCategory(ctx context.Context, cat *entities.ElearningCategory) error {
	if cat.ID == "" {
		cat.ID = database.NewID()
	}
	cat.CreatedAt = time.Now().UTC()
	cat.UpdatedAt = cat.CreatedAt
	_, err := r.categories.InsertOne(ctx, cat)
	if err != nil {
		return fmt.Errorf("elearning.repo.CreateCategory: %w", err)
	}
	return nil
}

// UpdateCategory updates an existing category document.
func (r *Repository) UpdateCategory(ctx context.Context, cat *entities.ElearningCategory) error {
	cat.UpdatedAt = time.Now().UTC()
	_, err := r.categories.ReplaceOne(ctx, bson.M{
		"_id":       cat.ID,
		"tenant_id": cat.TenantID,
	}, cat)
	if err != nil {
		return fmt.Errorf("elearning.repo.UpdateCategory: %w", err)
	}
	return nil
}

// DeleteCategory removes a category by ID.
func (r *Repository) DeleteCategory(ctx context.Context, tenantID, id string) error {
	result, err := r.categories.DeleteOne(ctx, bson.M{
		"_id":       id,
		"tenant_id": tenantID,
	})
	if err != nil {
		return fmt.Errorf("elearning.repo.DeleteCategory: %w", err)
	}
	if result.DeletedCount == 0 {
		return fmt.Errorf("elearning.repo.DeleteCategory: not found")
	}
	return nil
}

// ─── Levels ─────────────────────────────────────────────────────────────────

// ListLevelsByCategory returns all levels for a given category.
func (r *Repository) ListLevelsByCategory(ctx context.Context, tenantID, categoryID string) ([]*entities.ElearningLevel, error) {
	cursor, err := r.levels.Find(ctx, entities.LevelFilter(tenantID, categoryID),
		options.Find().SetSort(bson.D{{Key: "order", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("elearning.repo.ListLevelsByCategory: %w", err)
	}
	defer cursor.Close(ctx)

	var out []*entities.ElearningLevel
	if err := cursor.All(ctx, &out); err != nil {
		return nil, fmt.Errorf("elearning.repo.ListLevelsByCategory decode: %w", err)
	}
	return out, nil
}

// ListAllLevels returns all levels for a tenant (admin).
func (r *Repository) ListAllLevels(ctx context.Context, tenantID string) ([]*entities.ElearningLevel, error) {
	cursor, err := r.levels.Find(ctx, bson.M{"tenant_id": tenantID},
		options.Find().SetSort(bson.D{{Key: "order", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("elearning.repo.ListAllLevels: %w", err)
	}
	defer cursor.Close(ctx)

	var out []*entities.ElearningLevel
	if err := cursor.All(ctx, &out); err != nil {
		return nil, fmt.Errorf("elearning.repo.ListAllLevels decode: %w", err)
	}
	return out, nil
}

// GetLevelBySlug returns a level by slug.
func (r *Repository) GetLevelBySlug(ctx context.Context, tenantID, slug string) (*entities.ElearningLevel, error) {
	var lvl entities.ElearningLevel
	err := r.levels.FindOne(ctx, bson.M{
		"slug":      slug,
		"tenant_id": tenantID,
	}).Decode(&lvl)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("elearning.repo.GetLevelBySlug: %w", err)
	}
	return &lvl, nil
}

// GetLevelByID returns a level by ID.
func (r *Repository) GetLevelByID(ctx context.Context, tenantID, id string) (*entities.ElearningLevel, error) {
	var lvl entities.ElearningLevel
	err := r.levels.FindOne(ctx, bson.M{
		"_id":       id,
		"tenant_id": tenantID,
	}).Decode(&lvl)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("elearning.repo.GetLevelByID: %w", err)
	}
	return &lvl, nil
}

// CreateLevel inserts a new level.
func (r *Repository) CreateLevel(ctx context.Context, lvl *entities.ElearningLevel) error {
	if lvl.ID == "" {
		lvl.ID = database.NewID()
	}
	lvl.CreatedAt = time.Now().UTC()
	lvl.UpdatedAt = lvl.CreatedAt
	_, err := r.levels.InsertOne(ctx, lvl)
	if err != nil {
		return fmt.Errorf("elearning.repo.CreateLevel: %w", err)
	}
	return nil
}

// UpdateLevel updates an existing level document.
func (r *Repository) UpdateLevel(ctx context.Context, lvl *entities.ElearningLevel) error {
	lvl.UpdatedAt = time.Now().UTC()
	_, err := r.levels.ReplaceOne(ctx, bson.M{
		"_id":       lvl.ID,
		"tenant_id": lvl.TenantID,
	}, lvl)
	if err != nil {
		return fmt.Errorf("elearning.repo.UpdateLevel: %w", err)
	}
	return nil
}

// DeleteLevel removes a level and all its units.
func (r *Repository) DeleteLevel(ctx context.Context, tenantID, id string) error {
	// Delete units first.
	if _, err := r.units.DeleteMany(ctx, bson.M{
		"level_id":  id,
		"tenant_id": tenantID,
	}); err != nil {
		return fmt.Errorf("elearning.repo.DeleteLevel units: %w", err)
	}
	// Delete level.
	result, err := r.levels.DeleteOne(ctx, bson.M{
		"_id":       id,
		"tenant_id": tenantID,
	})
	if err != nil {
		return fmt.Errorf("elearning.repo.DeleteLevel: %w", err)
	}
	if result.DeletedCount == 0 {
		return fmt.Errorf("elearning.repo.DeleteLevel: not found")
	}
	return nil
}

// ─── Units ──────────────────────────────────────────────────────────────────

// ListUnitsByLevel returns all units for a given level.
func (r *Repository) ListUnitsByLevel(ctx context.Context, tenantID, levelID string) ([]*entities.ElearningUnit, error) {
	cursor, err := r.units.Find(ctx, entities.UnitFilter(tenantID, levelID),
		options.Find().SetSort(bson.D{{Key: "order", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("elearning.repo.ListUnitsByLevel: %w", err)
	}
	defer cursor.Close(ctx)

	var out []*entities.ElearningUnit
	if err := cursor.All(ctx, &out); err != nil {
		return nil, fmt.Errorf("elearning.repo.ListUnitsByLevel decode: %w", err)
	}
	return out, nil
}

// GetUnitBySlug returns a unit by slug.
func (r *Repository) GetUnitBySlug(ctx context.Context, tenantID, slug string) (*entities.ElearningUnit, error) {
	var unit entities.ElearningUnit
	err := r.units.FindOne(ctx, bson.M{
		"slug":      slug,
		"tenant_id": tenantID,
	}).Decode(&unit)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("elearning.repo.GetUnitBySlug: %w", err)
	}
	return &unit, nil
}

// GetUnitByID returns a unit by ID.
func (r *Repository) GetUnitByID(ctx context.Context, tenantID, id string) (*entities.ElearningUnit, error) {
	var unit entities.ElearningUnit
	err := r.units.FindOne(ctx, bson.M{
		"_id":       id,
		"tenant_id": tenantID,
	}).Decode(&unit)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("elearning.repo.GetUnitByID: %w", err)
	}
	return &unit, nil
}

// CreateUnit inserts a new unit.
func (r *Repository) CreateUnit(ctx context.Context, unit *entities.ElearningUnit) error {
	if unit.ID == "" {
		unit.ID = database.NewID()
	}
	unit.CreatedAt = time.Now().UTC()
	unit.UpdatedAt = unit.CreatedAt
	_, err := r.units.InsertOne(ctx, unit)
	if err != nil {
		return fmt.Errorf("elearning.repo.CreateUnit: %w", err)
	}
	return nil
}

// UpdateUnit updates an existing unit document.
func (r *Repository) UpdateUnit(ctx context.Context, unit *entities.ElearningUnit) error {
	unit.UpdatedAt = time.Now().UTC()
	_, err := r.units.ReplaceOne(ctx, bson.M{
		"_id":       unit.ID,
		"tenant_id": unit.TenantID,
	}, unit)
	if err != nil {
		return fmt.Errorf("elearning.repo.UpdateUnit: %w", err)
	}
	return nil
}

// DeleteUnit removes a unit by ID.
func (r *Repository) DeleteUnit(ctx context.Context, tenantID, id string) error {
	result, err := r.units.DeleteOne(ctx, bson.M{
		"_id":       id,
		"tenant_id": tenantID,
	})
	if err != nil {
		return fmt.Errorf("elearning.repo.DeleteUnit: %w", err)
	}
	if result.DeletedCount == 0 {
		return fmt.Errorf("elearning.repo.DeleteUnit: not found")
	}
	return nil
}
