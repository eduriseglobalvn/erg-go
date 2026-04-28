package postgrescore

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"gorm.io/gorm"

	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
)

// BackfillProfilesReport summarizes the Mongo -> PostgreSQL profile migration.
type BackfillProfilesReport struct {
	Seen    int
	Created int
	Updated int
	Skipped int
}

type legacyProfile struct {
	ID          string     `bson:"_id,omitempty"`
	UserID      string     `bson:"user_id"`
	FullName    string     `bson:"full_name"`
	Bio         string     `bson:"bio,omitempty"`
	Phone       string     `bson:"phone,omitempty"`
	DateOfBirth *time.Time `bson:"date_of_birth,omitempty"`
	Gender      string     `bson:"gender,omitempty"`
	Address     string     `bson:"address,omitempty"`
	City        string     `bson:"city,omitempty"`
	District    string     `bson:"district,omitempty"`
	SocialLinks string     `bson:"social_links,omitempty"`
	AvatarURL   string     `bson:"avatar_url,omitempty"`
	IsCompleted bool       `bson:"is_profile_completed,omitempty"`
	CreatedAt   time.Time  `bson:"created_at"`
	UpdatedAt   time.Time  `bson:"updated_at,omitempty"`
}

// BackfillLegacyProfilesFromMongo migrates legacy Mongo profiles into the new
// PostgreSQL profiles table. The backfill is idempotent and safe to run on each
// startup.
func BackfillLegacyProfilesFromMongo(
	ctx context.Context,
	db *gorm.DB,
	mongoClient *database.MongoClient,
	log *logger.Logger,
) (*BackfillProfilesReport, error) {
	if db == nil || mongoClient == nil {
		return &BackfillProfilesReport{}, nil
	}
	if log == nil {
		log = logger.NoOp()
	}

	cursor, err := mongoClient.Collection("profiles").Find(
		ctx,
		bson.M{},
		options.Find().SetSort(bson.D{{Key: "updated_at", Value: 1}, {Key: "_id", Value: 1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("postgrescore.backfill.profiles.find: %w", err)
	}
	defer cursor.Close(ctx)

	report := &BackfillProfilesReport{}
	for cursor.Next(ctx) {
		var legacy legacyProfile
		if err := cursor.Decode(&legacy); err != nil {
			return nil, fmt.Errorf("postgrescore.backfill.profiles.decode: %w", err)
		}
		report.Seen++

		userID := strings.TrimSpace(legacy.UserID)
		if userID == "" {
			report.Skipped++
			continue
		}

		now := time.Now().UTC()
		record := &Profile{
			ID:                 defaultString(strings.TrimSpace(legacy.ID), database.NewID()),
			UserID:             userID,
			FullName:           legacy.FullName,
			Bio:                legacy.Bio,
			Phone:              legacy.Phone,
			DateOfBirth:        legacy.DateOfBirth,
			Gender:             legacy.Gender,
			Address:            legacy.Address,
			City:               legacy.City,
			District:           legacy.District,
			SocialLinksJSON:    legacy.SocialLinks,
			AvatarURL:          legacy.AvatarURL,
			IsProfileCompleted: legacy.IsCompleted || strings.TrimSpace(legacy.FullName) != "",
			CreatedAt:          zeroTo(legacy.CreatedAt, now),
			UpdatedAt:          zeroTo(legacy.UpdatedAt, zeroTo(legacy.CreatedAt, now)),
		}

		var existing Profile
		err := db.WithContext(ctx).Where("user_id = ?", record.UserID).First(&existing).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("postgrescore.backfill.profiles.findExisting[%s]: %w", record.UserID, err)
		}

		if err == nil {
			if err := db.WithContext(ctx).
				Model(&Profile{}).
				Where("user_id = ?", record.UserID).
				Updates(map[string]any{
					"full_name":            record.FullName,
					"bio":                  record.Bio,
					"phone":                record.Phone,
					"date_of_birth":        record.DateOfBirth,
					"gender":               record.Gender,
					"address":              record.Address,
					"city":                 record.City,
					"district":             record.District,
					"social_links_json":    record.SocialLinksJSON,
					"avatar_url":           record.AvatarURL,
					"is_profile_completed": record.IsProfileCompleted,
					"updated_at":           record.UpdatedAt,
				}).Error; err != nil {
				return nil, fmt.Errorf("postgrescore.backfill.profiles.update[%s]: %w", record.UserID, err)
			}
			report.Updated++
			continue
		}

		if err := db.WithContext(ctx).Create(record).Error; err != nil {
			return nil, fmt.Errorf("postgrescore.backfill.profiles.create[%s]: %w", record.UserID, err)
		}
		report.Created++
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("postgrescore.backfill.profiles.cursor: %w", err)
	}

	log.Info().
		Int("profiles_seen", report.Seen).
		Int("profiles_created", report.Created).
		Int("profiles_updated", report.Updated).
		Int("profiles_skipped", report.Skipped).
		Msg("postgrescore: legacy profiles backfill complete")

	return report, nil
}

func zeroTo(value, fallback time.Time) time.Time {
	if value.IsZero() {
		return fallback
	}
	return value.UTC()
}
