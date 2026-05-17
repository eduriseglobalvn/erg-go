// Package repository provides data access for the profiles module.
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"gorm.io/gorm"

	entities "erg.ninja/internal/modules/profiles/domain/entity"
	"erg.ninja/internal/persistence/postgrescore"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
)

// Repository provides profile persistence with PostgreSQL as the primary store
// and MongoDB as a fallback for transitional environments.
type Repository struct {
	db   *gorm.DB
	coll *mongo.Collection
	log  *logger.Logger
}

// NewRepository creates a new profiles repository.
func NewRepository(mongoClient *database.MongoClient, pg *database.GORMPostgresClient, log *logger.Logger) *Repository {
	var (
		db   *gorm.DB
		coll *mongo.Collection
	)
	if pg != nil {
		db = pg.DB()
	}
	if mongoClient != nil {
		coll = mongoClient.Collection(entities.ProfileCollection)
	}
	if log == nil {
		log = logger.NoOp()
	}
	return &Repository{db: db, coll: coll, log: log}
}

func (r *Repository) hasPostgres() bool { return r.db != nil }

// BackfillFromMongo copies legacy Mongo profiles into PostgreSQL when both
// stores are present. The migration is idempotent.
func (r *Repository) BackfillFromMongo(ctx context.Context, mongoClient *database.MongoClient) error {
	if r.db == nil || mongoClient == nil {
		return nil
	}
	_, err := postgrescore.BackfillLegacyProfilesFromMongo(ctx, r.db, mongoClient, r.log)
	return err
}

// Create inserts a new profile.
func (r *Repository) Create(ctx context.Context, profile *entities.Profile) error {
	if r.hasPostgres() {
		return r.createPostgres(ctx, profile)
	}
	return r.createMongo(ctx, profile)
}

// GetByUserID retrieves a profile by user ID.
func (r *Repository) GetByUserID(ctx context.Context, userID string) (*entities.Profile, error) {
	if r.hasPostgres() {
		return r.getByUserIDPostgres(ctx, userID)
	}
	return r.getByUserIDMongo(ctx, userID)
}

// GetByID retrieves a profile by its ID.
func (r *Repository) GetByID(ctx context.Context, id string) (*entities.Profile, error) {
	if r.hasPostgres() {
		return r.getByIDPostgres(ctx, id)
	}
	return r.getByIDMongo(ctx, id)
}

// Update updates an existing profile.
func (r *Repository) Update(ctx context.Context, userID string, updates map[string]any) (*entities.Profile, error) {
	if r.hasPostgres() {
		return r.updatePostgres(ctx, userID, updates)
	}
	return r.updateMongo(ctx, userID, updates)
}

// Delete removes a profile by user ID.
func (r *Repository) Delete(ctx context.Context, userID string) error {
	if r.hasPostgres() {
		return r.deletePostgres(ctx, userID)
	}
	return r.deleteMongo(ctx, userID)
}

// List returns paginated profiles.
func (r *Repository) List(ctx context.Context, page, limit int) ([]*entities.Profile, int64, error) {
	if r.hasPostgres() {
		return r.listPostgres(ctx, page, limit)
	}
	return r.listMongo(ctx, page, limit)
}

// Upsert creates or updates a profile.
func (r *Repository) Upsert(ctx context.Context, profile *entities.Profile) (*entities.Profile, error) {
	if r.hasPostgres() {
		return r.upsertPostgres(ctx, profile)
	}
	return r.upsertMongo(ctx, profile)
}

func (r *Repository) createPostgres(ctx context.Context, profile *entities.Profile) error {
	now := time.Now().UTC()
	if profile.ID == "" {
		profile.ID = database.NewID()
	}
	profile.CreatedAt = now
	profile.UpdatedAt = now
	profile.IsCompleted = isProfileComplete(profile)

	record := profileToRecord(profile)
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		return fmt.Errorf("profiles.repo.Create: %w", err)
	}
	return nil
}

func (r *Repository) getByUserIDPostgres(ctx context.Context, userID string) (*entities.Profile, error) {
	var record postgrescore.Profile
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("profiles.repo.GetByUserID: %w", err)
	}
	return recordToProfile(&record), nil
}

func (r *Repository) getByIDPostgres(ctx context.Context, id string) (*entities.Profile, error) {
	var record postgrescore.Profile
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("profiles.repo.GetByID: %w", err)
	}
	return recordToProfile(&record), nil
}

func (r *Repository) updatePostgres(ctx context.Context, userID string, updates map[string]any) (*entities.Profile, error) {
	existing, err := r.getByUserIDPostgres(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("profiles.repo.Update.GetByUserID: %w", err)
	}
	if existing == nil {
		return nil, nil
	}

	merged := mergeProfileUpdates(existing, updates)
	updates["updated_at"] = time.Now().UTC()
	updates["is_profile_completed"] = isProfileComplete(merged)

	if err := r.db.WithContext(ctx).
		Model(&postgrescore.Profile{}).
		Where("user_id = ?", userID).
		Updates(convertProfileUpdates(updates)).Error; err != nil {
		return nil, fmt.Errorf("profiles.repo.Update: %w", err)
	}

	return r.getByUserIDPostgres(ctx, userID)
}

func (r *Repository) deletePostgres(ctx context.Context, userID string) error {
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&postgrescore.Profile{}).Error; err != nil {
		return fmt.Errorf("profiles.repo.Delete: %w", err)
	}
	return nil
}

func (r *Repository) listPostgres(ctx context.Context, page, limit int) ([]*entities.Profile, int64, error) {
	page, limit = normalizePage(page, limit)
	offset := (page - 1) * limit

	var total int64
	if err := r.db.WithContext(ctx).Model(&postgrescore.Profile{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("profiles.repo.List.Count: %w", err)
	}

	var records []postgrescore.Profile
	if err := r.db.WithContext(ctx).
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&records).Error; err != nil {
		return nil, 0, fmt.Errorf("profiles.repo.List.Find: %w", err)
	}

	profiles := make([]*entities.Profile, 0, len(records))
	for i := range records {
		profiles = append(profiles, recordToProfile(&records[i]))
	}
	return profiles, total, nil
}

func (r *Repository) upsertPostgres(ctx context.Context, profile *entities.Profile) (*entities.Profile, error) {
	existing, err := r.getByUserIDPostgres(ctx, profile.UserID)
	if err != nil {
		return nil, fmt.Errorf("profiles.repo.Upsert.GetByUserID: %w", err)
	}

	now := time.Now().UTC()
	if existing == nil {
		if profile.ID == "" {
			profile.ID = database.NewID()
		}
		profile.CreatedAt = now
	} else {
		profile.ID = existing.ID
		profile.CreatedAt = existing.CreatedAt
	}
	profile.UpdatedAt = now
	profile.IsCompleted = isProfileComplete(profile)

	record := profileToRecord(profile)
	if err := r.db.WithContext(ctx).Where("user_id = ?", profile.UserID).Assign(record).FirstOrCreate(record).Error; err != nil {
		return nil, fmt.Errorf("profiles.repo.Upsert: %w", err)
	}
	return r.getByUserIDPostgres(ctx, profile.UserID)
}

func (r *Repository) createMongo(ctx context.Context, profile *entities.Profile) error {
	now := time.Now().UTC()
	profile.CreatedAt = now
	profile.UpdatedAt = now
	profile.IsCompleted = isProfileComplete(profile)
	_, err := r.coll.InsertOne(ctx, profile)
	if err != nil {
		if database.IsDuplicateKey(err) {
			return fmt.Errorf("profiles.repo.Create: profile for user already exists")
		}
		return fmt.Errorf("profiles.repo.Create: %w", err)
	}
	return nil
}

func (r *Repository) getByUserIDMongo(ctx context.Context, userID string) (*entities.Profile, error) {
	var profile entities.Profile
	err := r.coll.FindOne(ctx, bson.M{"user_id": userID}).Decode(&profile)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("profiles.repo.GetByUserID: %w", err)
	}
	return &profile, nil
}

func (r *Repository) getByIDMongo(ctx context.Context, id string) (*entities.Profile, error) {
	var profile entities.Profile
	err := r.coll.FindOne(ctx, bson.M{"_id": id}).Decode(&profile)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("profiles.repo.GetByID: %w", err)
	}
	return &profile, nil
}

func (r *Repository) updateMongo(ctx context.Context, userID string, updates map[string]any) (*entities.Profile, error) {
	updates["updated_at"] = time.Now().UTC()

	existing, err := r.getByUserIDMongo(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("profiles.repo.Update.GetByUserID: %w", err)
	}
	if existing == nil {
		return nil, nil
	}

	merged := mergeProfileUpdates(existing, updates)
	updates["is_profile_completed"] = isProfileComplete(merged)

	result := r.coll.FindOneAndUpdate(
		ctx,
		bson.M{"user_id": userID},
		bson.M{"$set": updates},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)
	var profile entities.Profile
	if err := result.Decode(&profile); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("profiles.repo.Update.Decode: %w", err)
	}
	return &profile, nil
}

func (r *Repository) deleteMongo(ctx context.Context, userID string) error {
	if _, err := r.coll.DeleteOne(ctx, bson.M{"user_id": userID}); err != nil {
		return fmt.Errorf("profiles.repo.Delete: %w", err)
	}
	return nil
}

func (r *Repository) listMongo(ctx context.Context, page, limit int) ([]*entities.Profile, int64, error) {
	page, limit = normalizePage(page, limit)
	skip := int64((page - 1) * limit)

	total, err := r.coll.CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, 0, fmt.Errorf("profiles.repo.List.Count: %w", err)
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetSkip(skip).
		SetLimit(int64(limit))

	cur, err := r.coll.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("profiles.repo.List.Find: %w", err)
	}
	defer cur.Close(ctx)

	var profiles []*entities.Profile
	if err := cur.All(ctx, &profiles); err != nil {
		return nil, 0, fmt.Errorf("profiles.repo.List.Decode: %w", err)
	}
	return profiles, total, nil
}

func (r *Repository) upsertMongo(ctx context.Context, profile *entities.Profile) (*entities.Profile, error) {
	now := time.Now().UTC()
	profile.UpdatedAt = now
	profile.IsCompleted = isProfileComplete(profile)

	filter := bson.M{"user_id": profile.UserID}
	update := bson.M{
		"$set": bson.M{
			"full_name":            profile.FullName,
			"bio":                  profile.Bio,
			"phone":                profile.Phone,
			"date_of_birth":        profile.DateOfBirth,
			"gender":               profile.Gender,
			"address":              profile.Address,
			"city":                 profile.City,
			"district":             profile.District,
			"social_links":         profile.SocialLinks,
			"avatar_url":           profile.AvatarURL,
			"is_profile_completed": profile.IsCompleted,
			"updated_at":           now,
		},
		"$setOnInsert": bson.M{
			"user_id":    profile.UserID,
			"created_at": now,
		},
	}

	opts := options.FindOneAndUpdate().
		SetUpsert(true).
		SetReturnDocument(options.After)

	var result entities.Profile
	if err := r.coll.FindOneAndUpdate(ctx, filter, update, opts).Decode(&result); err != nil {
		return nil, fmt.Errorf("profiles.repo.Upsert: %w", err)
	}
	return &result, nil
}

func profileToRecord(profile *entities.Profile) *postgrescore.Profile {
	return &postgrescore.Profile{
		ID:                 profile.ID,
		UserID:             profile.UserID,
		FullName:           profile.FullName,
		Bio:                profile.Bio,
		Phone:              profile.Phone,
		DateOfBirth:        profile.DateOfBirth,
		Gender:             profile.Gender,
		Address:            profile.Address,
		City:               profile.City,
		District:           profile.District,
		SocialLinksJSON:    profile.SocialLinks,
		AvatarURL:          profile.AvatarURL,
		IsProfileCompleted: profile.IsCompleted,
		CreatedAt:          profile.CreatedAt.UTC(),
		UpdatedAt:          profile.UpdatedAt.UTC(),
	}
}

func recordToProfile(record *postgrescore.Profile) *entities.Profile {
	if record == nil {
		return nil
	}
	return &entities.Profile{
		ID:          record.ID,
		UserID:      record.UserID,
		FullName:    record.FullName,
		Bio:         record.Bio,
		Phone:       record.Phone,
		DateOfBirth: record.DateOfBirth,
		Gender:      record.Gender,
		Address:     record.Address,
		City:        record.City,
		District:    record.District,
		SocialLinks: record.SocialLinksJSON,
		AvatarURL:   record.AvatarURL,
		IsCompleted: record.IsProfileCompleted,
		CreatedAt:   record.CreatedAt,
		UpdatedAt:   record.UpdatedAt,
	}
}

func isProfileComplete(p *entities.Profile) bool {
	return p != nil && p.FullName != ""
}

func mergeProfileUpdates(existing *entities.Profile, updates map[string]any) *entities.Profile {
	if existing == nil {
		return nil
	}
	merged := *existing
	if v, ok := updates["full_name"].(string); ok {
		merged.FullName = v
	}
	if v, ok := updates["bio"].(string); ok {
		merged.Bio = v
	}
	if v, ok := updates["phone"].(string); ok {
		merged.Phone = v
	}
	if v, ok := updates["date_of_birth"].(*time.Time); ok {
		merged.DateOfBirth = v
	}
	if v, ok := updates["gender"].(string); ok {
		merged.Gender = v
	}
	if v, ok := updates["address"].(string); ok {
		merged.Address = v
	}
	if v, ok := updates["city"].(string); ok {
		merged.City = v
	}
	if v, ok := updates["district"].(string); ok {
		merged.District = v
	}
	if v, ok := updates["social_links"].(string); ok {
		merged.SocialLinks = v
	}
	if v, ok := updates["avatar_url"].(string); ok {
		merged.AvatarURL = v
	}
	return &merged
}

func convertProfileUpdates(updates map[string]any) map[string]any {
	out := make(map[string]any, len(updates))
	for key, value := range updates {
		switch key {
		case "social_links":
			out["social_links_json"] = value
		default:
			out[key] = value
		}
	}
	return out
}

func normalizePage(page, limit int) (int, int) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return page, limit
}
