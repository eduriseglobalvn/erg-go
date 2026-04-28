package pages

// Package pages implements the CMS Pages module.
// Pattern: NewModule → Setup → RegisterRoutes → Stop.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"

	"erg.ninja/internal/dto/response"
	"erg.ninja/internal/modules/pages/dto"
	"erg.ninja/internal/modules/pages/entities"
	"erg.ninja/internal/persistence/postgrescore"
	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/tenant"
)

const pageCacheTTL = 5 * time.Minute

var ErrPageNotFound = errors.New("pages: not found")

// Deps holds the pages module's dependencies.
type Deps struct {
	Mongo             *database.MongoClient
	GORMClient        *database.GORMPostgresClient
	Redis             *cache.RedisClient
	Log               *logger.Logger
	Cfg               *config.Config
	TenantMongoClient *tenant.TenantMongoClient
}

// Module implements the erg-go module pattern.
type Module struct {
	deps Deps
	repo *repository
	svc  *Service
	ctrl *Controller
}

// NewModule creates a new pages module.
func NewModule(deps Deps) *Module {
	return &Module{deps: deps}
}

// Name implements plugin.Module.
func (m *Module) Name() string { return "pages" }

// Setup initialises the module (like NestJS onModuleInit).
func (m *Module) Setup() error {
	m.deps.Log.Info().Msg("pages: module setup")

	m.repo = newRepository(m.deps.GORMClient)
	m.svc = newService(m.repo, m.deps.Redis, m.deps.Log)
	m.ctrl = newController(m.svc, m.deps.Log)
	return nil
}

// RegisterRoutes mounts the pages HTTP routes.
func (m *Module) RegisterRoutes(r *gin.Engine) {
	if m.ctrl != nil {
		m.ctrl.registerRoutes(r)
	}
}

// Stop performs graceful shutdown.
func (m *Module) Stop(ctx context.Context) error {
	m.deps.Log.Info().Msg("pages: module stopping")
	return nil
}

// ─── Repository ────────────────────────────────────────────────────────────────

type repository struct {
	db *gorm.DB
}

func newRepository(gormClient *database.GORMPostgresClient) *repository {
	var db *gorm.DB
	if gormClient != nil {
		db = gormClient.DB()
	}
	return &repository{db: db}
}

func (r *repository) ensureDB() error {
	if r.db == nil {
		return fmt.Errorf("pages: postgres client unavailable")
	}
	return nil
}

func (r *repository) findBySlug(ctx context.Context, tenantID, domain, slug string) (*entities.Page, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

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
	return pageFromRecord(&record), nil
}

func (r *repository) create(ctx context.Context, page *entities.Page) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if page.ID == "" {
		page.ID = database.NewID()
	}
	page.CreatedAt = time.Now().UTC()
	page.UpdatedAt = page.CreatedAt
	return r.db.WithContext(ctx).Create(pageToRecord(page)).Error
}

func (r *repository) updateFields(ctx context.Context, tenantID, domain, slug string, updates map[string]any) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
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

func (r *repository) setStatus(ctx context.Context, tenantID, domain, slug, status string) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	set := map[string]any{"status": status, "updated_at": time.Now().UTC()}
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

func (r *repository) delete(ctx context.Context, tenantID, domain, slug string) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
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

func (r *repository) list(ctx context.Context, tenantID, domain, cursor string, limit int) ([]entities.Page, string, error) {
	if err := r.ensureDB(); err != nil {
		return nil, "", err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	query := r.db.WithContext(ctx).Model(&postgrescore.Page{}).Where("tenant_id = ?", tenantID)
	if domain != "" {
		query = query.Where("domain = ?", domain)
	}
	if cursor != "" {
		var cursorPage postgrescore.Page
		err := r.db.WithContext(ctx).
			Select("id", "updated_at").
			Where("id = ?", cursor).
			First(&cursorPage).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, "", err
		}
		if err == nil {
			query = query.Where("(updated_at < ?) OR (updated_at = ? AND id < ?)", cursorPage.UpdatedAt, cursorPage.UpdatedAt, cursorPage.ID)
		}
	}

	var records []postgrescore.Page
	if err := query.
		Order("updated_at DESC, id DESC").
		Limit(limit + 1).
		Find(&records).Error; err != nil {
		return nil, "", err
	}

	pages := make([]entities.Page, 0, len(records))
	for i := range records {
		pages = append(pages, *pageFromRecord(&records[i]))
	}
	var nextCursor string
	if len(pages) > limit {
		pages = pages[:limit]
		nextCursor = pages[len(pages)-1].ID
	}
	return pages, nextCursor, nil
}

func pageToRecord(page *entities.Page) *postgrescore.Page {
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
		CreatedAt:       page.CreatedAt.UTC(),
		UpdatedAt:       page.UpdatedAt.UTC(),
	}
}

func pageFromRecord(record *postgrescore.Page) *entities.Page {
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

// ─── Service ──────────────────────────────────────────────────────────────────

type Service struct {
	repo  *repository
	redis *cache.RedisClient
	log   *logger.Logger
}

func newService(repo *repository, redis *cache.RedisClient, log *logger.Logger) *Service {
	return &Service{repo: repo, redis: redis, log: log}
}

func (s *Service) cacheKey(tenantID, domain, slug string) string {
	return fmt.Sprintf("page:%s:%s:%s", tenantID, domain, slug)
}

func toPageResponse(p *entities.Page) dto.PageResponse {
	r := dto.PageResponse{
		ID:              p.ID,
		Slug:            p.Slug,
		Domain:          p.Domain,
		Title:           p.Title,
		Content:         p.Content,
		MetaTitle:       p.MetaTitle,
		MetaDescription: p.MetaDescription,
		Faqs:            []dto.FaqItem{},
		PublishedAt:     p.PublishedAt,
	}
	if p.FaqJSON != "" {
		_ = json.Unmarshal([]byte(p.FaqJSON), &r.Faqs)
	}
	return r
}

func (s *Service) GetPageBySlug(ctx context.Context, tenantID, domain, slug string) (*dto.PageResponse, error) {
	if s.redis != nil {
		key := s.cacheKey(tenantID, domain, slug)
		cached, err := s.redis.Get(ctx, key)
		if err == nil && cached != "" {
			var r dto.PageResponse
			if json.Unmarshal([]byte(cached), &r) == nil {
				return &r, nil
			}
		}
	}
	page, err := s.repo.findBySlug(ctx, tenantID, domain, slug)
	if err != nil {
		return nil, err
	}
	r := toPageResponse(page)
	if s.redis != nil {
		key := s.cacheKey(tenantID, domain, slug)
		if buf, err := json.Marshal(r); err == nil {
			_ = s.redis.Set(ctx, key, string(buf), pageCacheTTL)
		}
	}
	return &r, nil
}

func (s *Service) invalidateCache(ctx context.Context, tenantID, domain, slug string) {
	if s.redis == nil {
		return
	}
	_ = s.redis.Del(ctx, s.cacheKey(tenantID, domain, slug))
}

func (s *Service) Create(ctx context.Context, req *dto.CreatePageRequest) (*entities.Page, error) {
	page := &entities.Page{
		TenantID:        req.TenantID,
		Slug:            req.Slug,
		Domain:          req.Domain,
		Title:           req.Title,
		Content:         req.Content,
		MetaTitle:       req.MetaTitle,
		MetaDescription: req.MetaDescription,
		FaqJSON:         req.FaqJSON,
		Status:          req.Status,
	}
	if err := s.repo.create(ctx, page); err != nil {
		return nil, fmt.Errorf("pages.service.create: %w", err)
	}
	return page, nil
}

func (s *Service) Update(ctx context.Context, tenantID, domain, slug string, req *dto.UpdatePageRequest) error {
	updates := map[string]any{}
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Content != nil {
		updates["content"] = *req.Content
	}
	if req.MetaTitle != nil {
		updates["meta_title"] = *req.MetaTitle
	}
	if req.MetaDescription != nil {
		updates["meta_description"] = *req.MetaDescription
	}
	if req.FaqJSON != nil {
		updates["faq_json"] = *req.FaqJSON
	}
	if req.Status != nil {
		if err := s.repo.setStatus(ctx, tenantID, domain, slug, *req.Status); err != nil {
			return fmt.Errorf("pages.service.update: %w", err)
		}
		s.invalidateCache(ctx, tenantID, domain, slug)
		return nil
	}
	if len(updates) == 0 {
		return nil
	}
	if err := s.repo.updateFields(ctx, tenantID, domain, slug, updates); err != nil {
		return fmt.Errorf("pages.service.update: %w", err)
	}
	s.invalidateCache(ctx, tenantID, domain, slug)
	return nil
}

func (s *Service) Delete(ctx context.Context, tenantID, domain, slug string) error {
	if err := s.repo.delete(ctx, tenantID, domain, slug); err != nil {
		return fmt.Errorf("pages.service.delete: %w", err)
	}
	s.invalidateCache(ctx, tenantID, domain, slug)
	return nil
}

func (s *Service) List(ctx context.Context, tenantID, domain, cursor string, limit int) ([]dto.PageResponse, string, error) {
	pages, nextCursor, err := s.repo.list(ctx, tenantID, domain, cursor, limit)
	if err != nil {
		return nil, "", fmt.Errorf("pages.service.list: %w", err)
	}
	items := make([]dto.PageResponse, len(pages))
	for i := range pages {
		items[i] = toPageResponse(&pages[i])
	}
	return items, nextCursor, nil
}

func (s *Service) Publish(ctx context.Context, tenantID, domain, slug string) error {
	if err := s.repo.setStatus(ctx, tenantID, domain, slug, entities.StatusPublished); err != nil {
		return fmt.Errorf("pages.service.publish: %w", err)
	}
	s.invalidateCache(ctx, tenantID, domain, slug)
	return nil
}

func (s *Service) Unpublish(ctx context.Context, tenantID, domain, slug string) error {
	if err := s.repo.setStatus(ctx, tenantID, domain, slug, entities.StatusDraft); err != nil {
		return fmt.Errorf("pages.service.unpublish: %w", err)
	}
	s.invalidateCache(ctx, tenantID, domain, slug)
	return nil
}

func NormalizeDomain(raw string) string {
	if len(raw) > 4 && raw[:4] == "www." {
		return raw[4:]
	}
	return raw
}

func IsNotFound(err error) bool {
	return errors.Is(err, ErrPageNotFound)
}

// ─── Controller ────────────────────────────────────────────────────────────────

type Controller struct {
	svc *Service
	log *logger.Logger
	val *validator.Validate
}

func newController(svc *Service, log *logger.Logger) *Controller {
	return &Controller{svc: svc, log: log, val: validator.New()}
}

func (c *Controller) registerRoutes(r *gin.Engine) {
	r.GET("/pages/:slug", c.getBySlug)
	r.GET("/api/pages/:slug", c.getBySlug)
}

func (c *Controller) getTenant(ctx *gin.Context) string {
	return tenant.FromContext(ctx.Request.Context())
}

// getBySlug handles GET /api/pages/:slug.
// @Summary Get page by slug
// @Description Fetch a CMS page by its URL slug.
// @Tags Pages
// @Accept json
// @Produce json
// @Param slug path string true "Page Slug"
// @Success 200 {object} dto.PageResponse
// @Failure 404 {object} response.Response
// @Router /api/pages/{slug} [get]
func (c *Controller) getBySlug(ctx *gin.Context) {
	tenantID := c.getTenant(ctx)
	domain := NormalizeDomain(ctx.Request.Host)
	slug := ctx.Param("slug")
	if slug == "" {
		response.BadRequestGin(ctx, fmt.Errorf("slug is required"))
		return
	}
	page, err := c.svc.GetPageBySlug(ctx.Request.Context(), tenantID, domain, slug)
	if err != nil {
		if IsNotFound(err) {
			response.NotFoundGin(ctx, "page not found")
			return
		}
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, page)
}
