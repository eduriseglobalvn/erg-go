package controller

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/dto/response"
	"erg.ninja/internal/middleware"
	"erg.ninja/internal/modules/elearning/api/dto"
	elearningservice "erg.ninja/internal/modules/elearning/application/service"
	"erg.ninja/pkg/logger"
)

// Controller handles HTTP requests for the elearning module.
type Controller struct {
	svc *elearningservice.Service
	log *logger.Logger
}

// NewController creates a new elearning controller.
func NewController(svc *elearningservice.Service, log *logger.Logger) *Controller {
	return &Controller{svc: svc, log: log}
}

// RegisterPublicRoutes registers public read-only endpoints.
func (c *Controller) RegisterPublicRoutes(r *gin.Engine) {
	r.GET("/api/elearning/categories", c.ListCategories)
	r.GET("/api/elearning/categories/:slug", c.GetCategoryBySlug)
	r.GET("/api/elearning/levels/:slug", c.GetLevelBySlug)
}

// RegisterAdminRoutes registers authenticated admin CRUD endpoints.
func (c *Controller) RegisterAdminRoutes(r *gin.RouterGroup) {
	// Categories.
	r.GET("/admin/elearning/categories", c.ListCategoriesAdmin)
	r.POST("/admin/elearning/categories", c.CreateCategory)
	r.PATCH("/admin/elearning/categories/:id", c.UpdateCategory)
	r.DELETE("/admin/elearning/categories/:id", c.DeleteCategory)

	// Levels.
	r.POST("/admin/elearning/levels", c.CreateLevel)
	r.PATCH("/admin/elearning/levels/:id", c.UpdateLevel)
	r.DELETE("/admin/elearning/levels/:id", c.DeleteLevel)

	// Units.
	r.POST("/admin/elearning/units", c.CreateUnit)
	r.PATCH("/admin/elearning/units/:id", c.UpdateUnit)
	r.DELETE("/admin/elearning/units/:id", c.DeleteUnit)
}

// ─── Public handlers ───────────────────────────────────────────────────────────

// ListCategories handles GET /api/elearning/categories.
// @Summary List categories
// @Description Fetch all categories for the current tenant.
// @Tags Elearning
// @Accept json
// @Produce json
// @Success 200 {array} dto.CategoryResponse
// @Failure 500 {object} map[string]string
// @Router /api/elearning/categories [get]
func (c *Controller) ListCategories(ctx *gin.Context) {
	tenant := c.tenantFromGin(ctx)

	cats, err := c.svc.ListCategories(ctx.Request.Context(), tenant)
	if err != nil {
		c.writeError(ctx, http.StatusInternalServerError, "LIST_CATEGORIES_FAILED", err.Error())
		return
	}
	if cats == nil {
		cats = []dto.CategoryResponse{}
	}
	c.json(ctx, http.StatusOK, cats)
}

// GetCategoryBySlug handles GET /api/elearning/categories/:slug.
// @Summary Get category by slug
// @Description Fetch category details by slug.
// @Tags Elearning
// @Accept json
// @Produce json
// @Param slug path string true "Category Slug"
// @Success 200 {object} dto.CategoryResponse
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/elearning/categories/{slug} [get]
func (c *Controller) GetCategoryBySlug(ctx *gin.Context) {
	tenant := c.tenantFromGin(ctx)
	slug := ctx.Param("slug")

	cat, err := c.svc.GetCategoryBySlug(ctx.Request.Context(), tenant, slug)
	if err != nil {
		c.writeError(ctx, http.StatusInternalServerError, "GET_CATEGORY_FAILED", err.Error())
		return
	}
	if cat == nil {
		c.json(ctx, http.StatusNotFound, map[string]string{"message": "category not found"})
		return
	}
	c.json(ctx, http.StatusOK, cat)
}

// GetLevelBySlug handles GET /api/elearning/levels/:slug.
// @Summary Get level by slug
// @Description Fetch level details by slug.
// @Tags Elearning
// @Accept json
// @Produce json
// @Param slug path string true "Level Slug"
// @Success 200 {object} dto.LevelResponse
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/elearning/levels/{slug} [get]
func (c *Controller) GetLevelBySlug(ctx *gin.Context) {
	tenant := c.tenantFromGin(ctx)
	slug := ctx.Param("slug")

	lvl, err := c.svc.GetLevelBySlug(ctx.Request.Context(), tenant, slug)
	if err != nil {
		c.writeError(ctx, http.StatusInternalServerError, "GET_LEVEL_FAILED", err.Error())
		return
	}
	if lvl == nil {
		c.json(ctx, http.StatusNotFound, map[string]string{"message": "level not found"})
		return
	}
	c.json(ctx, http.StatusOK, lvl)
}

// ─── Admin category handlers ──────────────────────────────────────────────────

// ListCategoriesAdmin handles GET /admin/elearning/categories.
// @Summary Admin: List categories
// @Description Fetch all categories for admin management.
// @Tags Elearning Admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {array} dto.CategoryResponse
// @Router /admin/elearning/categories [get]
func (c *Controller) ListCategoriesAdmin(ctx *gin.Context) {
	tenant := c.tenantFromGin(ctx)

	cats, err := c.svc.ListCategoriesAdmin(ctx.Request.Context(), tenant)
	if err != nil {
		c.writeError(ctx, http.StatusInternalServerError, "LIST_CATEGORIES_ADMIN_FAILED", err.Error())
		return
	}
	if cats == nil {
		cats = []dto.CategoryResponse{}
	}
	c.json(ctx, http.StatusOK, cats)
}

// CreateCategory handles POST /admin/elearning/categories.
// @Summary Admin: Create category
// @Description Create a new category.
// @Tags Elearning Admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param payload body dto.CreateCategoryRequest true "Category Data"
// @Success 201 {object} dto.CategoryResponse
// @Router /admin/elearning/categories [post]
func (c *Controller) CreateCategory(ctx *gin.Context) {
	reqCtx, cancel := context.WithTimeout(ctx.Request.Context(), 10*time.Second)
	defer cancel()

	var req dto.CreateCategoryRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		c.json(ctx, http.StatusBadRequest, map[string]string{"message": "invalid request body"})
		return
	}
	if req.Name == "" {
		c.json(ctx, http.StatusBadRequest, map[string]string{"message": "name is required"})
		return
	}
	if req.TenantID == "" {
		req.TenantID = c.tenantFromGin(ctx)
	}

	cat, err := c.svc.CreateCategory(reqCtx, &req)
	if err != nil {
		c.writeError(ctx, http.StatusInternalServerError, "CREATE_CATEGORY_FAILED", err.Error())
		return
	}
	c.json(ctx, http.StatusCreated, map[string]any{"id": cat.ID, "slug": cat.Slug})
}

// UpdateCategory handles PATCH /admin/elearning/categories/:id.
// @Summary Admin: Update category
// @Description Updates an elearning category.
// @Tags Elearning Admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Category ID"
// @Success 200 {object} map[string]any
// @Router /admin/elearning/categories/{id} [patch]
func (c *Controller) UpdateCategory(ctx *gin.Context) {
	reqCtx, cancel := context.WithTimeout(ctx.Request.Context(), 10*time.Second)
	defer cancel()

	id := ctx.Param("id")
	tenant := c.tenantFromGin(ctx)

	var req dto.UpdateCategoryRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		c.json(ctx, http.StatusBadRequest, map[string]string{"message": "invalid request body"})
		return
	}

	cat, err := c.svc.UpdateCategory(reqCtx, tenant, id, &req)
	if err != nil {
		c.writeError(ctx, http.StatusInternalServerError, "UPDATE_CATEGORY_FAILED", err.Error())
		return
	}
	if cat == nil {
		c.json(ctx, http.StatusNotFound, map[string]string{"message": "category not found"})
		return
	}
	c.json(ctx, http.StatusOK, cat)
}

// DeleteCategory handles DELETE /admin/elearning/categories/:id.
// @Summary Admin: Delete category
// @Description Deletes an elearning category.
// @Tags Elearning Admin
// @Produce json
// @Security BearerAuth
// @Param id path string true "Category ID"
// @Success 200 {object} map[string]any
// @Router /admin/elearning/categories/{id} [delete]
func (c *Controller) DeleteCategory(ctx *gin.Context) {
	reqCtx, cancel := context.WithTimeout(ctx.Request.Context(), 10*time.Second)
	defer cancel()

	id := ctx.Param("id")
	tenant := c.tenantFromGin(ctx)

	if err := c.svc.DeleteCategory(reqCtx, tenant, id); err != nil {
		if contains(err.Error(), "not found") {
			c.json(ctx, http.StatusNotFound, map[string]string{"message": "category not found"})
			return
		}
		c.writeError(ctx, http.StatusInternalServerError, "DELETE_CATEGORY_FAILED", err.Error())
		return
	}
	c.json(ctx, http.StatusOK, map[string]string{"id": id, "status": "deleted"})
}

// ─── Admin level handlers ─────────────────────────────────────────────────────

// CreateLevel handles POST /admin/elearning/levels.
// @Summary Admin: Create level
// @Description Creates a new elearning level.
// @Tags Elearning Admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 201 {object} map[string]any
// @Router /admin/elearning/levels [post]
func (c *Controller) CreateLevel(ctx *gin.Context) {
	reqCtx, cancel := context.WithTimeout(ctx.Request.Context(), 10*time.Second)
	defer cancel()

	var req dto.CreateLevelRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		c.json(ctx, http.StatusBadRequest, map[string]string{"message": "invalid request body"})
		return
	}
	if req.CategoryID == "" {
		c.json(ctx, http.StatusBadRequest, map[string]string{"message": "category_id is required"})
		return
	}
	if req.TenantID == "" {
		req.TenantID = c.tenantFromGin(ctx)
	}

	lvl, err := c.svc.CreateLevel(reqCtx, &req)
	if err != nil {
		if contains(err.Error(), "not found") {
			c.json(ctx, http.StatusNotFound, map[string]string{"message": err.Error()})
			return
		}
		c.writeError(ctx, http.StatusInternalServerError, "CREATE_LEVEL_FAILED", err.Error())
		return
	}
	c.json(ctx, http.StatusCreated, map[string]any{"id": lvl.ID, "slug": lvl.Slug})
}

// UpdateLevel handles PATCH /admin/elearning/levels/:id.
// @Summary Admin: Update level
// @Description Updates an elearning level.
// @Tags Elearning Admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Level ID"
// @Success 200 {object} map[string]any
// @Router /admin/elearning/levels/{id} [patch]
func (c *Controller) UpdateLevel(ctx *gin.Context) {
	reqCtx, cancel := context.WithTimeout(ctx.Request.Context(), 10*time.Second)
	defer cancel()

	id := ctx.Param("id")
	tenant := c.tenantFromGin(ctx)

	var req dto.UpdateLevelRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		c.json(ctx, http.StatusBadRequest, map[string]string{"message": "invalid request body"})
		return
	}

	lvl, err := c.svc.UpdateLevel(reqCtx, tenant, id, &req)
	if err != nil {
		c.writeError(ctx, http.StatusInternalServerError, "UPDATE_LEVEL_FAILED", err.Error())
		return
	}
	if lvl == nil {
		c.json(ctx, http.StatusNotFound, map[string]string{"message": "level not found"})
		return
	}
	c.json(ctx, http.StatusOK, lvl)
}

// DeleteLevel handles DELETE /admin/elearning/levels/:id.
// @Summary Admin: Delete level
// @Description Deletes an elearning level.
// @Tags Elearning Admin
// @Produce json
// @Security BearerAuth
// @Param id path string true "Level ID"
// @Success 200 {object} map[string]any
// @Router /admin/elearning/levels/{id} [delete]
func (c *Controller) DeleteLevel(ctx *gin.Context) {
	reqCtx, cancel := context.WithTimeout(ctx.Request.Context(), 10*time.Second)
	defer cancel()

	id := ctx.Param("id")
	tenant := c.tenantFromGin(ctx)

	if err := c.svc.DeleteLevel(reqCtx, tenant, id); err != nil {
		if contains(err.Error(), "not found") {
			c.json(ctx, http.StatusNotFound, map[string]string{"message": "level not found"})
			return
		}
		c.writeError(ctx, http.StatusInternalServerError, "DELETE_LEVEL_FAILED", err.Error())
		return
	}
	c.json(ctx, http.StatusOK, map[string]string{"id": id, "status": "deleted"})
}

// ─── Admin unit handlers ──────────────────────────────────────────────────────

// CreateUnit handles POST /admin/elearning/units.
// @Summary Admin: Create unit
// @Description Creates a new elearning unit.
// @Tags Elearning Admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 201 {object} map[string]any
// @Router /admin/elearning/units [post]
func (c *Controller) CreateUnit(ctx *gin.Context) {
	reqCtx, cancel := context.WithTimeout(ctx.Request.Context(), 10*time.Second)
	defer cancel()

	var req dto.CreateUnitRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		c.json(ctx, http.StatusBadRequest, map[string]string{"message": "invalid request body"})
		return
	}
	if req.LevelID == "" {
		c.json(ctx, http.StatusBadRequest, map[string]string{"message": "level_id is required"})
		return
	}
	if req.TenantID == "" {
		req.TenantID = c.tenantFromGin(ctx)
	}

	unit, err := c.svc.CreateUnit(reqCtx, &req)
	if err != nil {
		if contains(err.Error(), "not found") {
			c.json(ctx, http.StatusNotFound, map[string]string{"message": err.Error()})
			return
		}
		c.writeError(ctx, http.StatusInternalServerError, "CREATE_UNIT_FAILED", err.Error())
		return
	}
	c.json(ctx, http.StatusCreated, map[string]any{"id": unit.ID, "slug": unit.Slug})
}

// UpdateUnit handles PATCH /admin/elearning/units/:id.
// @Summary Admin: Update unit
// @Description Updates an elearning unit.
// @Tags Elearning Admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Unit ID"
// @Success 200 {object} map[string]any
// @Router /admin/elearning/units/{id} [patch]
func (c *Controller) UpdateUnit(ctx *gin.Context) {
	reqCtx, cancel := context.WithTimeout(ctx.Request.Context(), 10*time.Second)
	defer cancel()

	id := ctx.Param("id")
	tenant := c.tenantFromGin(ctx)

	var req dto.UpdateUnitRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		c.json(ctx, http.StatusBadRequest, map[string]string{"message": "invalid request body"})
		return
	}

	unit, err := c.svc.UpdateUnit(reqCtx, tenant, id, &req)
	if err != nil {
		c.writeError(ctx, http.StatusInternalServerError, "UPDATE_UNIT_FAILED", err.Error())
		return
	}
	if unit == nil {
		c.json(ctx, http.StatusNotFound, map[string]string{"message": "unit not found"})
		return
	}
	c.json(ctx, http.StatusOK, unit)
}

// DeleteUnit handles DELETE /admin/elearning/units/:id.
// @Summary Admin: Delete unit
// @Description Deletes an elearning unit.
// @Tags Elearning Admin
// @Produce json
// @Security BearerAuth
// @Param id path string true "Unit ID"
// @Success 200 {object} map[string]any
// @Router /admin/elearning/units/{id} [delete]
func (c *Controller) DeleteUnit(ctx *gin.Context) {
	reqCtx, cancel := context.WithTimeout(ctx.Request.Context(), 10*time.Second)
	defer cancel()

	id := ctx.Param("id")
	tenant := c.tenantFromGin(ctx)

	if err := c.svc.DeleteUnit(reqCtx, tenant, id); err != nil {
		if contains(err.Error(), "not found") {
			c.json(ctx, http.StatusNotFound, map[string]string{"message": "unit not found"})
			return
		}
		c.writeError(ctx, http.StatusInternalServerError, "DELETE_UNIT_FAILED", err.Error())
		return
	}
	c.json(ctx, http.StatusOK, map[string]string{"id": id, "status": "deleted"})
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

// tenantFromGin extracts tenant ID from JWT claims or returns "default".
func (c *Controller) tenantFromGin(ctx *gin.Context) string {
	if claims := middleware.GetClaims(ctx.Request.Context()); claims != nil {
		if claims.UserID != "" {
			return claims.UserID
		}
	}
	return "default"
}

// json writes a JSON response with the given status code.
func (c *Controller) json(ctx *gin.Context, status int, v any) {
	response.WriteGin(ctx, status, v, nil, nil)
}

// writeError writes a structured JSON error response.
func (c *Controller) writeError(ctx *gin.Context, status int, code, message string) {
	response.ErrorGin(ctx, status, code, message)
}

// contains reports whether substr is in s.
func contains(s, substr string) bool {
	if len(substr) == 0 {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
