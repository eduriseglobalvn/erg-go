package postgrescore

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// AutoMigrate provisions the relational schema needed by auth, users, sessions,
// and access-control. The migration is idempotent and safe to call on every startup.
func AutoMigrate(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("postgrescore.AutoMigrate: db is nil")
	}
	if err := db.AutoMigrate(
		&AuthUser{},
		&AuthSession{},
		&AuthPin{},
		&AuthLoginAttempt{},
		&FirewallRule{},
		&ACPermission{},
		&ACPermissionGroup{},
		&ACRole{},
		&ACUserPermissionOverride{},
		&UserRole{},
		&RolePermission{},
		&PostCategory{},
		&Post{},
		&Page{},
		&SystemConfig{},
		&Profile{},
		&Certificate{},
		&SocialAccount{},
		&CourseProgress{},
		&WorkShift{},
		&RecruitmentJob{},
		&RecruitmentCandidate{},
		&Center{},
		&UserAccessScope{},
		&CommunityTopic{},
		&CommunityPost{},
		&CommunityMedia{},
		&CommunityComment{},
		&CommunityReaction{},
		&CommunityFollow{},
	); err != nil {
		return fmt.Errorf("postgrescore.AutoMigrate: %w", err)
	}

	// Seed default centers (idempotent)
	if err := SeedCenters(context.Background(), db); err != nil {
		return fmt.Errorf("postgrescore.SeedCenters: %w", err)
	}

	return nil
}
