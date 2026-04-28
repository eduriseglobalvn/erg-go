package dto

import "time"

// ─── Category DTOs ─────────────────────────────────────────────────────────────

// CategoryResponse is the public API shape for a category (nested with levels).
type CategoryResponse struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Slug        string          `json:"slug"`
	Description string          `json:"description"`
	Order       int             `json:"order"`
	IsActive    bool            `json:"is_active"`
	Levels      []LevelResponse `json:"levels,omitempty"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

// CreateCategoryRequest is the payload for POST /admin/elearning/categories.
type CreateCategoryRequest struct {
	TenantID    string `json:"tenant_id"`
	Name        string `json:"name" validate:"required"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	ParentID    string `json:"parent_id,omitempty"`
	Order       int    `json:"order"`
	IsActive    *bool  `json:"is_active"`
}

// UpdateCategoryRequest is the payload for PATCH /admin/elearning/categories/:id.
type UpdateCategoryRequest struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	ParentID    string `json:"parent_id,omitempty"`
	Order       int    `json:"order"`
	IsActive    *bool  `json:"is_active"`
}

// ─── Level DTOs ────────────────────────────────────────────────────────────────

// LevelResponse is the API shape for a level with nested units.
type LevelResponse struct {
	ID          string         `json:"id"`
	CategoryID  string         `json:"category_id"`
	Name        string         `json:"name"`
	Slug        string         `json:"slug"`
	Description string         `json:"description"`
	Order       int            `json:"order"`
	Units       []UnitResponse `json:"units,omitempty"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
}

// CreateLevelRequest is the payload for POST /admin/elearning/levels.
type CreateLevelRequest struct {
	TenantID    string `json:"tenant_id"`
	CategoryID  string `json:"category_id" validate:"required"`
	Name        string `json:"name" validate:"required"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Order       int    `json:"order"`
}

// UpdateLevelRequest is the payload for PATCH /admin/elearning/levels/:id.
type UpdateLevelRequest struct {
	CategoryID  string `json:"category_id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Order       int    `json:"order"`
}

// ─── Unit DTOs ─────────────────────────────────────────────────────────────────

// UnitResponse is the API shape for a unit.
type UnitResponse struct {
	ID          string    `json:"id"`
	LevelID     string    `json:"level_id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	Content     string    `json:"content,omitempty"`
	Order       int       `json:"order"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// CreateUnitRequest is the payload for POST /admin/elearning/units.
type CreateUnitRequest struct {
	TenantID    string `json:"tenant_id"`
	LevelID     string `json:"level_id" validate:"required"`
	Name        string `json:"name" validate:"required"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Content     string `json:"content"`
	Order       int    `json:"order"`
}

// UpdateUnitRequest is the payload for PATCH /admin/elearning/units/:id.
type UpdateUnitRequest struct {
	LevelID     string `json:"level_id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	Content     string `json:"content"`
	Order       int    `json:"order"`
}

// ─── Paginated list wrapper ───────────────────────────────────────────────────

// ListResponse wraps a list with pagination metadata.
type ListResponse[T any] struct {
	Items  []T   `json:"items"`
	Total  int64 `json:"total"`
	Limit  int64 `json:"limit"`
	Offset int64 `json:"offset"`
}
