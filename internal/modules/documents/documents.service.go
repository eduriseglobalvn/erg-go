package documents

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"erg.ninja/internal/modules/documents/dto"
	"erg.ninja/internal/modules/documents/entities"
	"erg.ninja/internal/modules/documents/repository"
	"erg.ninja/internal/modules/documents/watermark"
	"erg.ninja/pkg/auth"
	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/storage"
	"erg.ninja/pkg/tenant"
)

const (
	uploadTimeout = 10 * time.Second
	maxFileSize   = 100 << 20 // 100 MB
)

// Deps holds the documents module's dependencies.
type Deps struct {
	Mongo             *database.MongoClient
	Redis             *cache.RedisClient
	Log               *logger.Logger
	Cfg               *config.Config
	JWTValidator      *auth.JWTValidator
	TenantMongoClient *tenant.TenantMongoClient
	R2                *storage.R2Client
	GDrive            *storage.GDriveClient
}

// Module implements the erg-go module pattern.
type Module struct {
	deps     Deps
	repo     *repository.Repository
	svc      *Service
	ctrl     *Controller
	wmRender *watermark.Renderer
}

// NewModule creates a new documents module.
func NewModule(deps Deps) *Module {
	return &Module{deps: deps}
}

// Name implements plugin.Module.
func (m *Module) Name() string { return "documents" }

// Setup initialises the module (like NestJS onModuleInit).
func (m *Module) Setup() error {
	m.deps.Log.Info().Msg("documents: module setup")

	m.wmRender = watermark.NewRenderer(m.deps.Log)

	repo := repository.NewRepository(m.deps.Mongo,
		repository.WithDocumentsLogger(m.deps.Log),
	)
	m.repo = repo

	m.svc = NewService(repo, m.deps.Redis, m.wmRender, m.deps.R2, m.deps.GDrive,
		WithDocumentsLogger(m.deps.Log),
	)

	m.ctrl = NewController(m.svc, m.deps.Log, m.deps.JWTValidator)
	return nil
}

// RegisterRoutes mounts the documents HTTP routes.
func (m *Module) RegisterRoutes(r *gin.Engine) {
	if m.ctrl != nil {
		m.ctrl.RegisterRoutes(r)
	}
}

// Service exposes the module service for dependent modules.
func (m *Module) Service() *Service {
	return m.svc
}

// Stop performs graceful shutdown.
func (m *Module) Stop(ctx context.Context) error {
	m.deps.Log.Info().Msg("documents: module stopped")
	return nil
}

// ─── Service ──────────────────────────────────────────────────────────────────

// Service handles business logic for documents.
type Service struct {
	repo  *repository.Repository
	redis *cache.RedisClient
	wm    *watermark.Renderer
	r2    *storage.R2Client
	drive *storage.GDriveClient
	log   *logger.Logger
}

// ServiceOption configures the Service.
type ServiceOption func(*Service)

// WithDocumentsLogger sets the logger.
func WithDocumentsLogger(log *logger.Logger) ServiceOption {
	return func(s *Service) { s.log = log }
}

// NewService creates a new documents service.
func NewService(
	repo *repository.Repository,
	redis *cache.RedisClient,
	wm *watermark.Renderer,
	r2 *storage.R2Client,
	drive *storage.GDriveClient,
	opts ...ServiceOption,
) *Service {
	s := &Service{
		repo:  repo,
		redis: redis,
		wm:    wm,
		r2:    r2,
		drive: drive,
		log:   logger.NoOp(),
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// r2Key builds the R2 object key for a document.
func r2Key(tenantID, docUUID, filename string) string {
	return fmt.Sprintf("documents/%s/%s/%s", tenantID, docUUID, filename)
}

func sanitizeDocumentFilename(filename string) string {
	name := filepath.Base(strings.TrimSpace(filename))
	if name == "." || name == string(filepath.Separator) || name == "" {
		name = "document.pdf"
	}
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := strings.Trim(b.String(), "._-")
	if out == "" {
		out = "document.pdf"
	}
	if !strings.EqualFold(filepath.Ext(out), ".pdf") {
		out += ".pdf"
	}
	return out
}

// Upload handles PDF upload: watermark → Storage upload → DB record.
func (s *Service) Upload(ctx context.Context, tenantID string, header *multipart.FileHeader, uploadedBy string, wmDTO dto.WatermarkConfigDTO, useDrive bool) (*dto.DocumentResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, uploadTimeout)
	defer cancel()

	if useDrive && s.drive == nil {
		return nil, fmt.Errorf("documents.service.upload: google drive not configured")
	}
	if !useDrive && s.r2 == nil {
		return nil, fmt.Errorf("documents.service.upload: r2 storage not configured")
	}

	// ── 1. Read uploaded PDF ────────────────────────────────────────────────
	src, err := header.Open()
	if err != nil {
		return nil, fmt.Errorf("documents.service.upload: open file: %w", err)
	}
	defer src.Close()

	pdfBytes, err := io.ReadAll(io.LimitReader(src, maxFileSize+1))
	if err != nil {
		return nil, fmt.Errorf("documents.service.upload: read file: %w", err)
	}
	if int64(len(pdfBytes)) > maxFileSize {
		return nil, fmt.Errorf("documents.service.upload: file exceeds max size")
	}
	if !ValidatePDFHeader(pdfBytes) {
		return nil, fmt.Errorf("documents.service.upload: invalid PDF header")
	}
	safeFilename := sanitizeDocumentFilename(header.Filename)

	// ── 2. Apply watermark ─────────────────────────────────────────────────
	wmCfg := wmDTO.ToEntity()
	if err := watermark.ValidateConfig(wmCfg); err != nil {
		return nil, fmt.Errorf("documents.service.upload: invalid watermark config: %w", err)
	}

	wmPDF, err := s.wm.Apply(pdfBytes, wmCfg)
	if err != nil {
		return nil, fmt.Errorf("documents.service.upload: watermark apply: %w", err)
	}

	// ── 3. Upload to Storage ───────────────────────────────────────────────
	docUUID := uuid.New().String()
	var r2URL, driveID string
	storageType := entities.StorageR2
	if useDrive {
		storageType = entities.StorageGDrive
		id, err := s.drive.Upload(ctx, bytes.NewReader(wmPDF), safeFilename, "application/pdf", "")
		if err != nil {
			return nil, fmt.Errorf("documents.service.upload: gdrive upload: %w", err)
		}
		driveID = id
	} else {
		url, err := s.r2.UploadRaw(ctx, wmPDF, r2Key(tenantID, docUUID, ""), safeFilename, "application/pdf")
		if err != nil {
			return nil, fmt.Errorf("documents.service.upload: r2 upload: %w", err)
		}
		r2URL = url
	}

	// ── 4. Persist DB record ───────────────────────────────────────────────
	doc := &entities.Document{
		ID:              "",
		TenantID:        tenantID,
		Filename:        safeFilename,
		OriginalName:    header.Filename,
		MimeType:        "application/pdf",
		Size:            int64(len(wmPDF)),
		StorageType:     storageType,
		R2URL:           r2URL,
		DriveID:         driveID,
		WatermarkConfig: wmCfg,
		Status:          entities.DocStatusReady,
		UploadedBy:      uploadedBy,
	}

	if err := s.repo.Create(ctx, doc); err != nil {
		// Rollback storage
		if useDrive {
			_ = s.drive.Delete(ctx, driveID)
		} else {
			_ = s.r2.Delete(ctx, r2URL)
		}
		return nil, fmt.Errorf("documents.service.upload: db create: %w", err)
	}

	resp := dto.ToResponse(doc)
	return &resp, nil
}

// GetByID returns document metadata by ID.
func (s *Service) GetByID(ctx context.Context, tenantID, id string) (*dto.DocumentResponse, error) {
	doc, err := s.repo.FindByID(ctx, tenantID, id)
	if err != nil {
		return nil, fmt.Errorf("documents.service.get_by_id: %w", err)
	}
	resp := dto.ToResponse(doc)
	return &resp, nil
}

// GetByIDForActor returns document metadata scoped to the owner unless the caller is an admin.
func (s *Service) GetByIDForActor(ctx context.Context, tenantID, id, userID string, isAdmin bool) (*dto.DocumentResponse, error) {
	doc, err := s.repo.FindByIDForUser(ctx, tenantID, id, userID, isAdmin)
	if err != nil {
		return nil, fmt.Errorf("documents.service.get_by_id: %w", err)
	}
	resp := dto.ToResponse(doc)
	return &resp, nil
}

// List returns paginated document metadata for a tenant.
func (s *Service) List(ctx context.Context, tenantID, cursor string, limit int) (*dto.ListDocumentsResponse, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	docs, nextCursor, err := s.repo.List(ctx, tenantID, cursor, limit)
	return s.listResponse(ctx, tenantID, "", true, docs, nextCursor, err)
}

// ListForActor lists documents scoped to the owner unless the caller is an admin.
func (s *Service) ListForActor(ctx context.Context, tenantID, userID string, isAdmin bool, cursor string, limit int) (*dto.ListDocumentsResponse, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	docs, nextCursor, err := s.repo.ListForUser(ctx, tenantID, userID, isAdmin, cursor, limit)
	return s.listResponse(ctx, tenantID, userID, isAdmin, docs, nextCursor, err)
}

func (s *Service) listResponse(ctx context.Context, tenantID, userID string, isAdmin bool, docs []entities.Document, nextCursor string, listErr error) (*dto.ListDocumentsResponse, error) {
	if listErr != nil {
		return nil, fmt.Errorf("documents.service.list: %w", listErr)
	}

	total, err := s.repo.CountForUser(ctx, tenantID, userID, isAdmin)
	if err != nil {
		s.log.Warn().Err(err).Msg("documents: count failed, returning 0 total")
		total = 0
	}

	items := make([]dto.DocumentResponse, len(docs))
	for i := range docs {
		items[i] = dto.ToResponse(&docs[i])
	}

	return &dto.ListDocumentsResponse{
		Items:      items,
		NextCursor: nextCursor,
		Total:      total,
	}, nil
}

// Update updates document metadata (watermark config).
func (s *Service) Update(ctx context.Context, tenantID, id string, req *dto.UpdateDocumentRequest) (*dto.DocumentResponse, error) {
	return s.UpdateForActor(ctx, tenantID, id, "", true, req)
}

// UpdateForActor updates document metadata scoped to the owner unless the caller is an admin.
func (s *Service) UpdateForActor(ctx context.Context, tenantID, id, userID string, isAdmin bool, req *dto.UpdateDocumentRequest) (*dto.DocumentResponse, error) {
	if req.Watermark != nil {
		wmCfg := req.Watermark.ToEntity()
		if err := watermark.ValidateConfig(wmCfg); err != nil {
			return nil, fmt.Errorf("documents.service.update: invalid watermark config: %w", err)
		}
		updates := map[string]any{
			"watermark_config": wmCfg,
		}
		if err := s.repo.UpdateFieldsForUser(ctx, tenantID, id, userID, isAdmin, updates); err != nil {
			return nil, fmt.Errorf("documents.service.update: %w", err)
		}
	}

	doc, err := s.repo.FindByIDForUser(ctx, tenantID, id, userID, isAdmin)
	if err != nil {
		return nil, fmt.Errorf("documents.service.update: find: %w", err)
	}
	resp := dto.ToResponse(doc)
	return &resp, nil
}

// Delete removes a document: DB record + R2 object.
func (s *Service) Delete(ctx context.Context, tenantID, id string) error {
	return s.DeleteForActor(ctx, tenantID, id, "", true)
}

// DeleteForActor deletes a document scoped to the owner unless the caller is an admin.
func (s *Service) DeleteForActor(ctx context.Context, tenantID, id, userID string, isAdmin bool) error {
	doc, err := s.repo.FindByIDForUser(ctx, tenantID, id, userID, isAdmin)
	if err != nil {
		return fmt.Errorf("documents.service.delete: %w", err)
	}

	// Remove from R2.
	if doc.R2URL != "" && s.r2 != nil {
		if err := s.r2.Delete(ctx, doc.R2URL); err != nil {
			s.log.Warn().Err(err).Str("id", id).Msg("documents: failed to delete R2 object")
		}
	}

	if err := s.repo.DeleteForUser(ctx, tenantID, id, userID, isAdmin); err != nil {
		return fmt.Errorf("documents.service.delete: %w", err)
	}
	return nil
}

// StreamFile returns the R2 URL or a stream for Drive documents.
func (s *Service) StreamFile(ctx context.Context, tenantID, id string) (string, io.ReadCloser, error) {
	return s.StreamFileForActor(ctx, tenantID, id, "", true)
}

// StreamFileForActor returns a storage stream scoped to the owner unless the caller is an admin.
func (s *Service) StreamFileForActor(ctx context.Context, tenantID, id, userID string, isAdmin bool) (string, io.ReadCloser, error) {
	doc, err := s.repo.FindByIDForUser(ctx, tenantID, id, userID, isAdmin)
	if err != nil {
		return "", nil, fmt.Errorf("documents.service.stream_file: %w", err)
	}
	if doc.Status != entities.DocStatusReady {
		return "", nil, fmt.Errorf("documents: document not ready for streaming")
	}

	if doc.StorageType == entities.StorageGDrive && s.drive != nil {
		stream, err := s.drive.Download(ctx, doc.DriveID)
		if err != nil {
			return "", nil, fmt.Errorf("documents.service.stream_file: gdrive: %w", err)
		}
		return "", stream, nil
	}

	if doc.R2URL != "" && s.r2 != nil {
		stream, _, _, err := s.r2.GetFileStream(ctx, doc.R2URL)
		if err != nil {
			return "", nil, fmt.Errorf("documents.service.stream_file: r2: %w", err)
		}
		return "", stream, nil
	}

	return "", nil, fmt.Errorf("documents.service.stream_file: storage object not available")
}

// IsNotFound returns true if err is a document-not-found sentinel.
func IsNotFound(err error) bool {
	return errors.Is(err, repository.ErrDocumentNotFound)
}

// ValidateFileExtension checks that the file extension is .pdf.
func ValidateFileExtension(filename string) bool {
	return strings.EqualFold(filepath.Ext(filename), ".pdf")
}

// ValidatePDFHeader checks the PDF magic bytes.
func ValidatePDFHeader(data []byte) bool {
	return bytes.HasPrefix(bytes.TrimSpace(data), []byte("%PDF-"))
}
