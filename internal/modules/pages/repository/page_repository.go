package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"erg.ninja/internal/modules/pages/entities"
	"erg.ninja/internal/persistence/postgrescore"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
)

var ErrPageNotFound = errors.New("pages: not found")

// Repository provides PostgreSQL data access for pages.
type Repository struct {
	db  *gorm.DB
	log *logger.Logger
}

// RepositoryOption configures the Repository.
type RepositoryOption func(*Repository)

// WithPagesLogger sets the logger.
func WithPagesLogger(log *logger.Logger) RepositoryOption {
	return func(r *Repository) { r.log = log }
}

// NewRepository creates a new pages repository.
func NewRepository(gormClient *database.GORMPostgresClient, opts ...RepositoryOption) *Repository {
	r := &Repository{log: logger.NoOp()}
	if gormClient != nil {
		r.db = gormClient.DB()
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

func (r *Repository) ensureDB() error {
	if r.db == nil {
		return errors.New("pages: postgres client unavailable")
	}
	return nil
}

// FindBySlug returns the published page matching the given tenant, domain, and slug.
func (r *Repository) FindBySlug(ctx context.Context, tenantID, domain, slug string) (*entities.Page, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}

	var record postgrescore.Page
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND domain = ? AND slug = ? AND status = ?", tenantID, domain, slug, entities.StatusPublished).
		First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPageNotFound
		}
		return nil, err
	}
	return fromRecord(&record), nil
}

// Create inserts a new page.
func (r *Repository) Create(ctx context.Context, page *entities.Page) error {
	if err := r.ensureDB(); err != nil {
		return err
	}

	if page.ID == "" {
		page.ID = database.NewID()
	}
	page.CreatedAt = time.Now().UTC()
	page.UpdatedAt = page.CreatedAt

	return r.db.WithContext(ctx).Create(toRecord(page)).Error
}

// UpdateFields updates only the provided fields of a page.
func (r *Repository) UpdateFields(ctx context.Context, tenantID, domain, slug string, updates map[string]any) error {
	if err := r.ensureDB(); err != nil {
		return err
	}

	updates["updated_at"] = time.Now().UTC()
	result := r.db.WithContext(ctx).
		Model(&postgrescore.Page{}).
		Where("tenant_id = ? AND domain = ? AND slug = ?", tenantID, domain, slug).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrPageNotFound
	}
	return nil
}

// SetStatus atomically sets the status and optionally the published_at timestamp.
func (r *Repository) SetStatus(ctx context.Context, tenantID, domain, slug, status string) error {
	if err := r.ensureDB(); err != nil {
		return err
	}

	set := map[string]any{
		"status":     status,
		"updated_at": time.Now().UTC(),
	}
	if status == entities.StatusPublished {
		now := time.Now().UTC()
		set["published_at"] = &now
	}

	result := r.db.WithContext(ctx).
		Model(&postgrescore.Page{}).
		Where("tenant_id = ? AND domain = ? AND slug = ?", tenantID, domain, slug).
		Updates(set)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrPageNotFound
	}
	return nil
}

// Delete removes a page by tenant + domain + slug.
func (r *Repository) Delete(ctx context.Context, tenantID, domain, slug string) error {
	if err := r.ensureDB(); err != nil {
		return err
	}

	result := r.db.WithContext(ctx).
		Where("tenant_id = ? AND domain = ? AND slug = ?", tenantID, domain, slug).
		Delete(&postgrescore.Page{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrPageNotFound
	}
	return nil
}

// List returns pages for a tenant with cursor-based pagination.
func (r *Repository) List(ctx context.Context, tenantID, domain, cursor string, limit int) ([]entities.Page, string, error) {
	if err := r.ensureDB(); err != nil {
		return nil, "", err
	}

	query := r.db.WithContext(ctx).Model(&postgrescore.Page{}).Where("tenant_id = ?", tenantID)
	if domain != "" {
		query = query.Where("domain = ?", domain)
	}

	if cursor != "" {
		var cursorPage postgrescore.Page
		err := r.db.WithContext(ctx).Select("id", "updated_at").Where("id = ?", cursor).First(&cursorPage).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, "", err
		}
		if err == nil {
			query = query.Where("(updated_at < ?) OR (updated_at = ? AND id < ?)", cursorPage.UpdatedAt, cursorPage.UpdatedAt, cursorPage.ID)
		}
	}

	var records []postgrescore.Page
	if err := query.Order("updated_at DESC, id DESC").Limit(limit + 1).Find(&records).Error; err != nil {
		return nil, "", err
	}

	pages := make([]entities.Page, 0, len(records))
	for i := range records {
		pages = append(pages, *fromRecord(&records[i]))
	}

	var nextCursor string
	if len(pages) > limit {
		pages = pages[:limit]
		nextCursor = pages[len(pages)-1].ID
	}
	return pages, nextCursor, nil
}

func toRecord(page *entities.Page) *postgrescore.Page {
	return &postgrescore.Page{
		ID:              page.ID,
		TenantID:        page.TenantID,
		Domain:          page.Domain,
		Slug:            page.Slug,
		Title:           page.Title,
		Content:         page.Content,
		MetaTitle:       page.MetaTitle,
		MetaDescription: page.MetaDescription,
		FAQJSON:         page.FaqJSON,
		Status:          page.Status,
		PublishedAt:     page.PublishedAt,
		CreatedAt:       page.CreatedAt,
		UpdatedAt:       page.UpdatedAt,
	}
}

func fromRecord(record *postgrescore.Page) *entities.Page {
	return &entities.Page{
		ID:              record.ID,
		TenantID:        record.TenantID,
		Slug:            record.Slug,
		Domain:          record.Domain,
		Title:           record.Title,
		Content:         record.Content,
		MetaTitle:       record.MetaTitle,
		MetaDescription: record.MetaDescription,
		FaqJSON:         record.FAQJSON,
		Status:          record.Status,
		PublishedAt:     record.PublishedAt,
		CreatedAt:       record.CreatedAt,
		UpdatedAt:       record.UpdatedAt,
	}
}
