package postgrescore

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SeedCenters ensures the core education units exist in the system.
func SeedCenters(ctx context.Context, db *gorm.DB) error {
	now := time.Now().UTC()
	
	// Default ERG System node - The root of everything
	systemNode := Center{
		ID:          "erg_system_root_00000001",
		Name:        "Hệ thống ERG",
		Slug:        "he-thong-erg",
		Type:        "system",
		LogoURL:     "/assets/images/erg-logo-circle.png",
		Description: "Nút gốc quản trị toàn bộ hệ thống ERG",
		Status:      "active",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	return db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "logo_url", "description", "updated_at"}),
	}).Create(&systemNode).Error
}
