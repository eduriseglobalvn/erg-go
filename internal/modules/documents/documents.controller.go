package documents

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/dto/response"
	"erg.ninja/internal/modules/documents/dto"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/tenant"
)

// Controller handles HTTP requests for the documents module.
type Controller struct {
	svc *Service
	log *logger.Logger
}

// NewController creates a new documents controller.
func NewController(svc *Service, log *logger.Logger) *Controller {
	return &Controller{svc: svc, log: log}
}

// RegisterRoutes mounts the documents REST API routes.
func (c *Controller) RegisterRoutes(r *gin.Engine) {
	docs := r.Group("/documents")
	docs.POST("/", c.Upload)
	docs.GET("/", c.List)
	docs.GET("/:id", c.GetByID)
	docs.GET("/:id/file", c.StreamFile)
	docs.PATCH("/:id", c.Update)
	docs.DELETE("/:id", c.Delete)
}

// getTenant extracts the tenant ID from context.
func (c *Controller) getTenant(ctx *gin.Context) string {
	return tenant.FromContext(ctx.Request.Context())
}

// Upload handles multipart PDF upload with watermark.
// @Summary Upload document
// @Description Uploads a PDF document with optional watermark.
// @Tags Documents
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Success 201 {object} map[string]any
// @Router /documents [post]
func (c *Controller) Upload(ctx *gin.Context) {
	tenantID := c.getTenant(ctx)

	if err := ctx.Request.ParseMultipartForm(50 << 20); err != nil { // 50 MB max in-memory
		response.BadRequestGin(ctx, fmt.Errorf("parse multipart: %w", err))
		return
	}

	file, header, err := ctx.Request.FormFile("file")
	if err != nil {
		response.BadRequestGin(ctx, fmt.Errorf("read file form field: %w", err))
		return
	}
	defer file.Close()

	if !ValidateFileExtension(header.Filename) {
		response.BadRequestGin(ctx, fmt.Errorf("only PDF files are allowed"))
		return
	}

	// Collect form fields.
	uploadedBy := ctx.Request.FormValue("uploaded_by")
	if uploadedBy == "" {
		response.BadRequestGin(ctx, fmt.Errorf("uploaded_by is required"))
		return
	}

	wmJSON := ctx.Request.FormValue("watermark")
	var wmCfg dto.WatermarkConfigDTO
	if wmJSON != "" {
		if err := json.Unmarshal([]byte(wmJSON), &wmCfg); err != nil {
			response.BadRequestGin(ctx, fmt.Errorf("invalid watermark JSON: %w", err))
			return
		}
	}
	// Apply defaults.
	if wmCfg.Position == "" {
		wmCfg.Position = "CENTER"
	}
	if wmCfg.Opacity == 0 {
		wmCfg.Opacity = 0.3
	}
	if wmCfg.Color == "" {
		wmCfg.Color = "#888888"
	}
	if wmCfg.FontSize == 0 {
		wmCfg.FontSize = 48
	}
	useDrive := ctx.Request.FormValue("use_drive") == "true"

	doc, err := c.svc.Upload(ctx.Request.Context(), tenantID, header, uploadedBy, wmCfg, useDrive)
	if err != nil {
		c.log.Error().Err(err).Msg("documents.controller.upload")
		response.InternalErrorGin(ctx, err)
		return
	}

	response.CreatedGin(ctx, doc)
}

// List returns paginated document metadata.
// @Summary List documents
// @Description Returns paginated list of documents.
// @Tags Documents
// @Produce json
// @Security BearerAuth
// @Param cursor query string false "Cursor"
// @Param limit query int false "Limit"
// @Success 200 {object} map[string]any
// @Router /documents [get]
func (c *Controller) List(ctx *gin.Context) {
	tenantID := c.getTenant(ctx)
	cursor := ctx.Query("cursor")
	limit, _ := strconv.Atoi(ctx.Query("limit"))
	if limit == 0 {
		limit = 20
	}

	result, err := c.svc.List(ctx.Request.Context(), tenantID, cursor, limit)
	if err != nil {
		c.log.Error().Err(err).Msg("documents.controller.list")
		response.InternalErrorGin(ctx, err)
		return
	}

	response.OKGin(ctx, result)
}

// GetByID returns document metadata.
// @Summary Get document by ID
// @Description Returns a single document by ID.
// @Tags Documents
// @Produce json
// @Security BearerAuth
// @Param id path string true "Document ID"
// @Success 200 {object} map[string]any
// @Router /documents/{id} [get]
func (c *Controller) GetByID(ctx *gin.Context) {
	tenantID := c.getTenant(ctx)
	id := ctx.Param("id")

	doc, err := c.svc.GetByID(ctx.Request.Context(), tenantID, id)
	if err != nil {
		if IsNotFound(err) {
			response.NotFoundGin(ctx, "document not found")
			return
		}
		c.log.Error().Err(err).Str("id", id).Msg("documents.controller.get_by_id")
		response.InternalErrorGin(ctx, err)
		return
	}

	response.OKGin(ctx, doc)
}

// StreamFile streams the PDF file from R2 with security headers.
// @Summary Stream document file
// @Description Streams the PDF file with security headers.
// @Tags Documents
// @Produce application/pdf
// @Security BearerAuth
// @Param id path string true "Document ID"
// @Success 200
// @Router /documents/{id}/file [get]
// StreamFile streams the PDF file with security headers.
func (c *Controller) StreamFile(ctx *gin.Context) {
	tenantID := c.getTenant(ctx)
	id := ctx.Param("id")

	r2URL, stream, err := c.svc.StreamFile(ctx.Request.Context(), tenantID, id)
	if err != nil {
		if IsNotFound(err) {
			response.NotFoundGin(ctx, "document not found")
			return
		}
		c.log.Error().Err(err).Str("id", id).Msg("documents.controller.stream_file")
		response.InternalErrorGin(ctx, err)
		return
	}

	// ── Security headers ───────────────────────────────────────────────────
	// Prevent embedding in iframes from other domains
	ctx.Header("X-Frame-Options", "SAMEORIGIN")
	ctx.Header("X-Content-Type-Options", "nosniff")
	// Content security policy to prevent execution of scripts within PDF
	ctx.Header("Content-Security-Policy", "default-src 'none'; frame-ancestors 'self'")
	// Suggest browser not to store
	ctx.Header("Cache-Control", "private, no-cache, no-store, must-revalidate")
	ctx.Header("Content-Type", "application/pdf")
	// Disable download/print via headers where supported
	ctx.Header("Content-Disposition", "inline")

	// ── Stream Content ────────────────────────────────────────────────────
	if stream != nil {
		defer stream.Close()
		_, err = io.Copy(ctx.Writer, stream)
		if err != nil {
			c.log.Warn().Err(err).Msg("documents.controller.stream: write from stream failed")
		}
		return
	}

	if r2URL != "" {
		buf, _, err := c.svc.r2.GetFileBuffer(ctx.Request.Context(), r2URL)
		if err != nil {
			c.log.Error().Err(err).Str("id", id).Msg("documents.controller.stream: r2 read failed")
			response.InternalErrorGin(ctx, err)
			return
		}

		ctx.Header("Content-Length", strconv.Itoa(len(buf)))
		ctx.Writer.WriteHeader(http.StatusOK)
		if _, err := ctx.Writer.Write(buf); err != nil {
			c.log.Warn().Err(err).Msg("documents.controller.stream: write failed")
		}
	}
}

// Update updates document metadata.
// @Summary Update document
// @Description Updates document metadata.
// @Tags Documents
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Document ID"
// @Success 200 {object} map[string]any
// @Router /documents/{id} [patch]
func (c *Controller) Update(ctx *gin.Context) {
	tenantID := c.getTenant(ctx)
	id := ctx.Param("id")

	var req dto.UpdateDocumentRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}

	doc, err := c.svc.Update(ctx.Request.Context(), tenantID, id, &req)
	if err != nil {
		if IsNotFound(err) {
			response.NotFoundGin(ctx, "document not found")
			return
		}
		c.log.Error().Err(err).Str("id", id).Msg("documents.controller.update")
		response.InternalErrorGin(ctx, err)
		return
	}

	response.OKGin(ctx, doc)
}

// Delete removes a document.
// @Summary Delete document
// @Description Removes a document and its file.
// @Tags Documents
// @Produce json
// @Security BearerAuth
// @Param id path string true "Document ID"
// @Success 200 {object} map[string]any
// @Router /documents/{id} [delete]
func (c *Controller) Delete(ctx *gin.Context) {
	tenantID := c.getTenant(ctx)
	id := ctx.Param("id")

	if err := c.svc.Delete(ctx.Request.Context(), tenantID, id); err != nil {
		if IsNotFound(err) {
			response.NotFoundGin(ctx, "document not found")
			return
		}
		c.log.Error().Err(err).Str("id", id).Msg("documents.controller.delete")
		response.InternalErrorGin(ctx, err)
		return
	}

	response.OKGin(ctx, map[string]string{"message": "document deleted"})
}

// getFileBuffer is a helper that delegates to R2 client (exposed for testing).
func (c *Controller) getFileBuffer(ctx context.Context, r2URL string) ([]byte, string, error) {
	return c.svc.r2.GetFileBuffer(ctx, r2URL)
}
