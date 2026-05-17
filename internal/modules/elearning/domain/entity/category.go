package entities

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Collection names.
const (
	ElearningCategoryCollection = "elearning_categories"
	ElearningLevelCollection    = "elearning_levels"
	ElearningUnitCollection     = "elearning_units"
)

// ElearningCategory represents a top-level elearning content category.
type ElearningCategory struct {
	ID          string              `bson:"_id,omitempty" json:"id"`
	TenantID    string              `bson:"tenant_id" json:"tenant_id"`
	Name        string              `bson:"name" json:"name"`
	Slug        string              `bson:"slug" json:"slug"`
	Description string              `bson:"description" json:"description"`
	ParentID    *string             `bson:"parent_id,omitempty" json:"parent_id,omitempty"`
	Order       int                 `bson:"order" json:"order"`
	IsActive    bool                `bson:"is_active" json:"is_active"`
	Levels      []ElearningLevelRef `bson:"levels,omitempty" json:"levels,omitempty"`
	CreatedAt   time.Time           `bson:"created_at" json:"createdAt"`
	UpdatedAt   time.Time           `bson:"updated_at" json:"updatedAt"`
}

// ElearningLevelRef is a lightweight reference embedded inside a category.
type ElearningLevelRef struct {
	LevelID string `bson:"level_id" json:"level_id"`
	Name    string `bson:"name" json:"name"`
}

// ElearningLevel represents a level within a category (e.g., Beginner, Intermediate).
type ElearningLevel struct {
	ID          string             `bson:"_id,omitempty" json:"id"`
	TenantID    string             `bson:"tenant_id" json:"tenant_id"`
	CategoryID  string             `bson:"category_id" json:"category_id"`
	Name        string             `bson:"name" json:"name"`
	Slug        string             `bson:"slug" json:"slug"`
	Description string             `bson:"description" json:"description"`
	Order       int                `bson:"order" json:"order"`
	Units       []ElearningUnitRef `bson:"units,omitempty" json:"units,omitempty"`
	CreatedAt   time.Time          `bson:"created_at" json:"createdAt"`
	UpdatedAt   time.Time          `bson:"updated_at" json:"updatedAt"`
}

// ElearningUnitRef is a lightweight reference embedded inside a level.
type ElearningUnitRef struct {
	UnitID string `bson:"unit_id" json:"unit_id"`
	Name   string `bson:"name" json:"name"`
}

// ElearningUnit represents the leaf node — actual learning content.
type ElearningUnit struct {
	ID          string    `bson:"_id,omitempty" json:"id"`
	TenantID    string    `bson:"tenant_id" json:"tenant_id"`
	LevelID     string    `bson:"level_id" json:"level_id"`
	Name        string    `bson:"name" json:"name"`
	Slug        string    `bson:"slug" json:"slug"`
	Description string    `bson:"description" json:"description"`
	Content     string    `bson:"content" json:"content"`
	Order       int       `bson:"order" json:"order"`
	CreatedAt   time.Time `bson:"created_at" json:"createdAt"`
	UpdatedAt   time.Time `bson:"updated_at" json:"updatedAt"`
}

// CatFilter returns a MongoDB filter for categories.
func CatFilter(tenantID string, activeOnly bool) bson.M {
	f := bson.M{"tenant_id": tenantID}
	if activeOnly {
		f["is_active"] = true
	}
	return f
}

// LevelFilter returns a MongoDB filter for levels.
func LevelFilter(tenantID, categoryID string) bson.M {
	return bson.M{
		"tenant_id":   tenantID,
		"category_id": categoryID,
	}
}

// UnitFilter returns a MongoDB filter for units.
func UnitFilter(tenantID, levelID string) bson.M {
	return bson.M{
		"tenant_id": tenantID,
		"level_id":  levelID,
	}
}
