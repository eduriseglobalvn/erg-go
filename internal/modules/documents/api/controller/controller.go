package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/dto/response"
	"erg.ninja/internal/middleware"
	"erg.ninja/internal/modules/documents/api/dto"
	documentservice "erg.ninja/internal/modules/documents/application/service"
	"erg.ninja/pkg/auth"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/storage"
	"erg.ninja/pkg/tenant"
)

// Controller handles HTTP requests for the documents module.
type Controller struct {
	svc          *documentservice.Service
	log          *logger.Logger
	jwtValidator *auth.JWTValidator
}

// NewController creates a new documents controller.
func NewController(svc *documentservice.Service, log *logger.Logger, jwtValidator *auth.JWTValidator) *Controller {
	return &Controller{svc: svc, log: log, jwtValidator: jwtValidator}
}

// RegisterRoutes mounts the documents REST API routes.
func (c *Controller) RegisterRoutes(r *gin.Engine) {
	c.registerDocumentRoutes(r.Group("/documents"))
	c.registerDocumentRoutes(r.Group("/api/documents"))
}

func (c *Controller) registerDocumentRoutes(docs *gin.RouterGroup) {
	docs.Use(middleware.JWTMiddleware(c.jwtValidator))
	docs.POST("/", c.Upload)
	docs.GET("/", c.List)
	docs.GET("/:id", c.GetByID)
	docs.GET("/:id/file", c.StreamFile)
	docs.PATCH("/:id", c.Update)
	docs.DELETE("/:id", c.Delete)
}

func documentActor(ctx *gin.Context) (string, bool) {
	claims := middleware.GetClaims(ctx.Request.Context())
	if claims == nil {
		return "", false
	}
	for _, role := range claims.Roles {
		if strings.EqualFold(role, "admin") {
			return claims.UserID, true
		}
	}
	return claims.UserID, false
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

	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, documentservice.MaxFileSize+(1<<20))
	if err := ctx.Request.ParseMultipartForm(storage.MultipartMemoryLimit); err != nil {
		response.BadRequestGin(ctx, fmt.Errorf("parse multipart: %w", err))
		return
	}

	file, header, err := ctx.Request.FormFile("file")
	if err != nil {
		response.BadRequestGin(ctx, fmt.Errorf("read file form field: %w", err))
		return
	}
	defer file.Close()

	if !documentservice.ValidateFileExtension(header.Filename) {
		response.BadRequestGin(ctx, fmt.Errorf("only PDF files are allowed"))
		return
	}
	if header.Size > documentservice.MaxFileSize {
		response.BadRequestGin(ctx, fmt.Errorf("file exceeds max size"))
		return
	}

	uploadedBy, _ := documentActor(ctx)
	if uploadedBy == "" {
		response.BadRequestGin(ctx, fmt.Errorf("authenticated user is required"))
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
	userID, isAdmin := documentActor(ctx)
	cursor := ctx.Query("cursor")
	limit, _ := strconv.Atoi(ctx.Query("limit"))
	if limit == 0 {
		limit = 20
	}

	result, err := c.svc.ListForActor(ctx.Request.Context(), tenantID, userID, isAdmin, cursor, limit)
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
	userID, isAdmin := documentActor(ctx)

	doc, err := c.svc.GetByIDForActor(ctx.Request.Context(), tenantID, id, userID, isAdmin)
	if err != nil {
		if documentservice.IsNotFound(err) {
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
	userID, isAdmin := documentActor(ctx)

	_, stream, err := c.svc.StreamFileForActor(ctx.Request.Context(), tenantID, id, userID, isAdmin)
	if err != nil {
		if documentservice.IsNotFound(err) {
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

	response.InternalErrorGin(ctx, fmt.Errorf("document stream unavailable"))
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
	userID, isAdmin := documentActor(ctx)

	var req dto.UpdateDocumentRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}

	doc, err := c.svc.UpdateForActor(ctx.Request.Context(), tenantID, id, userID, isAdmin, &req)
	if err != nil {
		if documentservice.IsNotFound(err) {
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
	userID, isAdmin := documentActor(ctx)

	if err := c.svc.DeleteForActor(ctx.Request.Context(), tenantID, id, userID, isAdmin); err != nil {
		if documentservice.IsNotFound(err) {
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
	return c.svc.GetFileBuffer(ctx, r2URL)
}
