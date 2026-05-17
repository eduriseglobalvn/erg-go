package service

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	"erg.ninja/internal/modules/elearning/api/dto"
	entities "erg.ninja/internal/modules/elearning/domain/entity"
	elearningrepo "erg.ninja/internal/modules/elearning/infrastructure/repository"
	"erg.ninja/pkg/logger"
)

// Service handles business logic for the elearning module.
type Service struct {
	repo *elearningrepo.Repository
	log  *logger.Logger
}

// NewService creates a new elearning service.
func NewService(repo *elearningrepo.Repository, log *logger.Logger) *Service {
	if log == nil {
		log = logger.NoOp()
	}
	return &Service{repo: repo, log: log}
}

// ─── Category helpers ─────────────────────────────────────────────────────────

func (s *Service) assembleCategories(ctx context.Context, tenantID string, cats []*entities.ElearningCategory) ([]dto.CategoryResponse, error) {
	if len(cats) == 0 {
		return nil, nil
	}
	catIDs := make([]string, len(cats))
	for i, c := range cats {
		catIDs[i] = c.ID
	}

	// Fetch all levels for all categories in one query.
	var allLevels []*entities.ElearningLevel
	for _, catID := range catIDs {
		levels, err := s.repo.ListLevelsByCategory(ctx, tenantID, catID)
		if err != nil {
			return nil, fmt.Errorf("elearning.svc.assembleCategories levels: %w", err)
		}
		allLevels = append(allLevels, levels...)
	}

	// Fetch all units for all levels.
	var allUnits []*entities.ElearningUnit
	for _, lvl := range allLevels {
		units, err := s.repo.ListUnitsByLevel(ctx, tenantID, lvl.ID)
		if err != nil {
			return nil, fmt.Errorf("elearning.svc.assembleCategories units: %w", err)
		}
		allUnits = append(allUnits, units...)
	}

	out := make([]dto.CategoryResponse, 0, len(cats))
	for _, cat := range cats {
		catLevels := make([]dto.LevelResponse, 0)
		for _, lvl := range allLevels {
			if lvl.CategoryID != cat.ID {
				continue
			}
			lvlUnits := make([]dto.UnitResponse, 0)
			for _, unit := range allUnits {
				if unit.LevelID != lvl.ID {
					continue
				}
				lvlUnits = append(lvlUnits, toUnitResponse(unit))
			}
			catLevels = append(catLevels, dto.LevelResponse{
				ID:          lvl.ID,
				CategoryID:  lvl.CategoryID,
				Name:        lvl.Name,
				Slug:        lvl.Slug,
				Description: lvl.Description,
				Order:       lvl.Order,
				Units:       lvlUnits,
				CreatedAt:   lvl.CreatedAt,
				UpdatedAt:   lvl.UpdatedAt,
			})
		}
		out = append(out, dto.CategoryResponse{
			ID:          cat.ID,
			Name:        cat.Name,
			Slug:        cat.Slug,
			Description: cat.Description,
			Order:       cat.Order,
			IsActive:    cat.IsActive,
			Levels:      catLevels,
			CreatedAt:   cat.CreatedAt,
			UpdatedAt:   cat.UpdatedAt,
		})
	}
	return out, nil
}

func (s *Service) assembleLevel(ctx context.Context, tenantID string, lvl *entities.ElearningLevel) (*dto.LevelResponse, error) {
	units, err := s.repo.ListUnitsByLevel(ctx, tenantID, lvl.ID)
	if err != nil {
		return nil, fmt.Errorf("elearning.svc.assembleLevel units: %w", err)
	}
	unitDTOs := make([]dto.UnitResponse, 0, len(units))
	for _, u := range units {
		unitDTOs = append(unitDTOs, toUnitResponse(u))
	}
	return &dto.LevelResponse{
		ID:          lvl.ID,
		CategoryID:  lvl.CategoryID,
		Name:        lvl.Name,
		Slug:        lvl.Slug,
		Description: lvl.Description,
		Order:       lvl.Order,
		Units:       unitDTOs,
		CreatedAt:   lvl.CreatedAt,
		UpdatedAt:   lvl.UpdatedAt,
	}, nil
}

// ─── Public endpoints ─────────────────────────────────────────────────────────

// ListCategories returns all active categories with nested levels+units.
func (s *Service) ListCategories(ctx context.Context, tenantID string) ([]dto.CategoryResponse, error) {
	cats, err := s.repo.ListCategories(ctx, tenantID, true)
	if err != nil {
		return nil, fmt.Errorf("elearning.svc.ListCategories: %w", err)
	}
	return s.assembleCategories(ctx, tenantID, cats)
}

// GetCategoryBySlug returns a single category tree by slug.
func (s *Service) GetCategoryBySlug(ctx context.Context, tenantID, slug string) (*dto.CategoryResponse, error) {
	cat, err := s.repo.GetCategoryBySlug(ctx, tenantID, slug)
	if err != nil {
		return nil, fmt.Errorf("elearning.svc.GetCategoryBySlug: %w", err)
	}
	if cat == nil {
		return nil, nil
	}
	out, err := s.assembleCategories(ctx, tenantID, []*entities.ElearningCategory{cat})
	if err != nil {
		return nil, fmt.Errorf("elearning.svc.GetCategoryBySlug assemble: %w", err)
	}
	if len(out) == 0 {
		return nil, nil
	}
	return &out[0], nil
}

// GetLevelBySlug returns a level with nested units by slug.
func (s *Service) GetLevelBySlug(ctx context.Context, tenantID, slug string) (*dto.LevelResponse, error) {
	lvl, err := s.repo.GetLevelBySlug(ctx, tenantID, slug)
	if err != nil {
		return nil, fmt.Errorf("elearning.svc.GetLevelBySlug: %w", err)
	}
	if lvl == nil {
		return nil, nil
	}
	return s.assembleLevel(ctx, tenantID, lvl)
}

// ─── Admin endpoints ─────────────────────────────────────────────────────────

// ListCategoriesAdmin returns all categories (active + inactive) with full tree.
func (s *Service) ListCategoriesAdmin(ctx context.Context, tenantID string) ([]dto.CategoryResponse, error) {
	cats, err := s.repo.ListCategories(ctx, tenantID, false)
	if err != nil {
		return nil, fmt.Errorf("elearning.svc.ListCategoriesAdmin: %w", err)
	}
	return s.assembleCategories(ctx, tenantID, cats)
}

// CreateCategory creates a new category.
func (s *Service) CreateCategory(ctx context.Context, req *dto.CreateCategoryRequest) (*entities.ElearningCategory, error) {
	slug := req.Slug
	if slug == "" {
		slug = slugify(req.Name)
	}
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	cat := &entities.ElearningCategory{
		ID:          "",
		TenantID:    req.TenantID,
		Name:        req.Name,
		Slug:        slug,
		Description: req.Description,
		ParentID:    nilStr(req.ParentID),
		Order:       req.Order,
		IsActive:    isActive,
		Levels:      nil,
	}
	if err := s.repo.CreateCategory(ctx, cat); err != nil {
		return nil, fmt.Errorf("elearning.svc.CreateCategory: %w", err)
	}
	return cat, nil
}

// UpdateCategory updates an existing category.
func (s *Service) UpdateCategory(ctx context.Context, tenantID, id string, req *dto.UpdateCategoryRequest) (*entities.ElearningCategory, error) {
	cat, err := s.repo.GetCategoryByID(ctx, tenantID, id)
	if err != nil {
		return nil, fmt.Errorf("elearning.svc.UpdateCategory get: %w", err)
	}
	if cat == nil {
		return nil, nil
	}
	if req.Name != "" {
		cat.Name = req.Name
	}
	if req.Slug != "" {
		cat.Slug = req.Slug
	}
	if req.Description != "" {
		cat.Description = req.Description
	}
	if req.ParentID != "" {
		cat.ParentID = &req.ParentID
	}
	if req.Order != 0 {
		cat.Order = req.Order
	}
	if req.IsActive != nil {
		cat.IsActive = *req.IsActive
	}
	if err := s.repo.UpdateCategory(ctx, cat); err != nil {
		return nil, fmt.Errorf("elearning.svc.UpdateCategory: %w", err)
	}
	return cat, nil
}

// DeleteCategory removes a category and cascades to levels and units.
func (s *Service) DeleteCategory(ctx context.Context, tenantID, id string) error {
	cat, err := s.repo.GetCategoryByID(ctx, tenantID, id)
	if err != nil {
		return fmt.Errorf("elearning.svc.DeleteCategory get: %w", err)
	}
	if cat == nil {
		return fmt.Errorf("elearning.svc.DeleteCategory: not found")
	}

	// Delete all levels and their units.
	levels, err := s.repo.ListLevelsByCategory(ctx, tenantID, id)
	if err != nil {
		return fmt.Errorf("elearning.svc.DeleteCategory list levels: %w", err)
	}
	for _, lvl := range levels {
		if err := s.repo.DeleteLevel(ctx, tenantID, lvl.ID); err != nil {
			s.log.WarnContext(ctx).Err(err).
				Str("level_id", lvl.ID).Msg("elearning.svc.DeleteCategory: level delete failed")
		}
	}

	if err := s.repo.DeleteCategory(ctx, tenantID, id); err != nil {
		return fmt.Errorf("elearning.svc.DeleteCategory: %w", err)
	}
	return nil
}

// ─── Level admin ──────────────────────────────────────────────────────────────

// CreateLevel creates a new level.
func (s *Service) CreateLevel(ctx context.Context, req *dto.CreateLevelRequest) (*entities.ElearningLevel, error) {
	// Verify category exists.
	cat, err := s.repo.GetCategoryByID(ctx, req.TenantID, req.CategoryID)
	if err != nil {
		return nil, fmt.Errorf("elearning.svc.CreateLevel category check: %w", err)
	}
	if cat == nil {
		return nil, fmt.Errorf("elearning.svc.CreateLevel: category %s not found", req.CategoryID)
	}

	slug := req.Slug
	if slug == "" {
		slug = slugify(req.Name)
	}
	lvl := &entities.ElearningLevel{
		ID:          "",
		TenantID:    req.TenantID,
		CategoryID:  req.CategoryID,
		Name:        req.Name,
		Slug:        slug,
		Description: req.Description,
		Order:       req.Order,
		Units:       nil,
	}
	if err := s.repo.CreateLevel(ctx, lvl); err != nil {
		return nil, fmt.Errorf("elearning.svc.CreateLevel: %w", err)
	}
	return lvl, nil
}

// UpdateLevel updates an existing level.
func (s *Service) UpdateLevel(ctx context.Context, tenantID, id string, req *dto.UpdateLevelRequest) (*entities.ElearningLevel, error) {
	lvl, err := s.repo.GetLevelByID(ctx, tenantID, id)
	if err != nil {
		return nil, fmt.Errorf("elearning.svc.UpdateLevel get: %w", err)
	}
	if lvl == nil {
		return nil, nil
	}
	if req.CategoryID != "" {
		lvl.CategoryID = req.CategoryID
	}
	if req.Name != "" {
		lvl.Name = req.Name
	}
	if req.Slug != "" {
		lvl.Slug = req.Slug
	}
	if req.Description != "" {
		lvl.Description = req.Description
	}
	if req.Order != 0 {
		lvl.Order = req.Order
	}
	if err := s.repo.UpdateLevel(ctx, lvl); err != nil {
		return nil, fmt.Errorf("elearning.svc.UpdateLevel: %w", err)
	}
	return lvl, nil
}

// DeleteLevel removes a level and its units.
func (s *Service) DeleteLevel(ctx context.Context, tenantID, id string) error {
	if err := s.repo.DeleteLevel(ctx, tenantID, id); err != nil {
		return fmt.Errorf("elearning.svc.DeleteLevel: %w", err)
	}
	return nil
}

// ─── Unit admin ───────────────────────────────────────────────────────────────

// CreateUnit creates a new unit.
func (s *Service) CreateUnit(ctx context.Context, req *dto.CreateUnitRequest) (*entities.ElearningUnit, error) {
	// Verify level exists.
	lvl, err := s.repo.GetLevelByID(ctx, req.TenantID, req.LevelID)
	if err != nil {
		return nil, fmt.Errorf("elearning.svc.CreateUnit level check: %w", err)
	}
	if lvl == nil {
		return nil, fmt.Errorf("elearning.svc.CreateUnit: level %s not found", req.LevelID)
	}

	slug := req.Slug
	if slug == "" {
		slug = slugify(req.Name)
	}
	unit := &entities.ElearningUnit{
		ID:          "",
		TenantID:    req.TenantID,
		LevelID:     req.LevelID,
		Name:        req.Name,
		Slug:        slug,
		Description: req.Description,
		Content:     req.Content,
		Order:       req.Order,
	}
	if err := s.repo.CreateUnit(ctx, unit); err != nil {
		return nil, fmt.Errorf("elearning.svc.CreateUnit: %w", err)
	}
	return unit, nil
}

// UpdateUnit updates an existing unit.
func (s *Service) UpdateUnit(ctx context.Context, tenantID, id string, req *dto.UpdateUnitRequest) (*entities.ElearningUnit, error) {
	unit, err := s.repo.GetUnitByID(ctx, tenantID, id)
	if err != nil {
		return nil, fmt.Errorf("elearning.svc.UpdateUnit get: %w", err)
	}
	if unit == nil {
		return nil, nil
	}
	if req.LevelID != "" {
		unit.LevelID = req.LevelID
	}
	if req.Name != "" {
		unit.Name = req.Name
	}
	if req.Slug != "" {
		unit.Slug = req.Slug
	}
	if req.Description != "" {
		unit.Description = req.Description
	}
	if req.Content != "" {
		unit.Content = req.Content
	}
	if req.Order != 0 {
		unit.Order = req.Order
	}
	if err := s.repo.UpdateUnit(ctx, unit); err != nil {
		return nil, fmt.Errorf("elearning.svc.UpdateUnit: %w", err)
	}
	return unit, nil
}

// DeleteUnit removes a unit.
func (s *Service) DeleteUnit(ctx context.Context, tenantID, id string) error {
	if err := s.repo.DeleteUnit(ctx, tenantID, id); err != nil {
		return fmt.Errorf("elearning.svc.DeleteUnit: %w", err)
	}
	return nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func toUnitResponse(u *entities.ElearningUnit) dto.UnitResponse {
	return dto.UnitResponse{
		ID:          u.ID,
		LevelID:     u.LevelID,
		Name:        u.Name,
		Slug:        u.Slug,
		Description: u.Description,
		Content:     u.Content,
		Order:       u.Order,
		CreatedAt:   u.CreatedAt,
		UpdatedAt:   u.UpdatedAt,
	}
}

func nilStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// slugify converts a string to a URL-safe slug.
func slugify(input string) string {
	input = strings.ToLower(strings.TrimSpace(input))
	var b strings.Builder
	lastDash := false
	for _, r := range input {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		case !lastDash:
			b.WriteRune('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func ptrBool(b bool) *bool { return &b }
func ptrInt(i int) *int    { return &i }

// Time-based check for unit/unit list.
func recentUnits(units []*entities.ElearningUnit) []*entities.ElearningUnit {
	if len(units) == 0 {
		return units
	}
	cutoff := time.Now().Add(-24 * time.Hour)
	filtered := units[:0]
	for _, u := range units {
		if u.CreatedAt.After(cutoff) {
			filtered = append(filtered, u)
		}
	}
	return filtered
}
