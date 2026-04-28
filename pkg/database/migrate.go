package database

import (
	"fmt"

	"gorm.io/gorm"
)

// AutoMigrate runs gorm.DB.AutoMigrate for the given models.
// It is a no-op for MongoDB (where schema is schema-less).
// Safe to call multiple times — GORM skips already-migrated tables.
//
// Usage:
//
//	// PostgreSQL — migrates tables
//	if err := AutoMigrate(postgres.DB(), &User{}, &Post{}); err != nil {
//	    log.Fatal(err)
//	}
//
//	// MongoDB — no-op (call EnsureIndexes separately if needed)
//	_ = AutoMigrate(mongoGORM.DB()) // MongoDB: schema-less, AutoMigrate is a no-op
func AutoMigrate(db *gorm.DB, dst ...interface{}) error {
	if db == nil {
		return fmt.Errorf("auto-migrate: db is nil")
	}
	if len(dst) == 0 {
		return nil
	}
	if err := db.AutoMigrate(dst...); err != nil {
		return fmt.Errorf("auto-migrate: %w", err)
	}
	return nil
}

// MigrateModel is a convenience alias for AutoMigrate with a single model.
func MigrateModel(db *gorm.DB, model interface{}) error {
	return AutoMigrate(db, model)
}

// MigrateBatch runs AutoMigrate for multiple models at once.
// Returns as soon as the first error is encountered.
func MigrateBatch(db *gorm.DB, models []interface{}) error {
	if db == nil {
		return fmt.Errorf("auto-migrate: db is nil")
	}
	if len(models) == 0 {
		return nil
	}
	if err := db.AutoMigrate(models...); err != nil {
		return fmt.Errorf("auto-migrate batch: %w", err)
	}
	return nil
}
