// Package controller handles HTTP requests for the courses module.
package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/dto/response"
	"erg.ninja/internal/modules/courses/api/dto"
	"erg.ninja/internal/modules/courses/application/service"
	"erg.ninja/internal/modules/courses/infrastructure/repository"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/tenant"
)

// Controller handles HTTP requests for courses.
type Controller struct {
	svc *service.Service
	log *logger.Logger
}

// NewController creates a new courses controller.
func NewController(svc *service.Service, log *logger.Logger) *Controller {
	return &Controller{svc: svc, log: log}
}

// RegisterPublicRoutes mounts the public courses REST API routes.
func (c *Controller) RegisterPublicRoutes(rg *gin.RouterGroup) {
	rg.GET("", c.List)
	rg.GET("/subdomain/:sub", c.GetBySubdomain)
	rg.GET("/:id", c.Get)
	rg.GET("/:id/schema", c.GetSchema)
}

// RegisterAdminRoutes mounts the admin courses REST API routes.
func (c *Controller) RegisterAdminRoutes(rg *gin.RouterGroup) {
	rg.POST("", c.Create)
	rg.PATCH("/:id", c.Update)
	rg.PATCH("/:id/theme", c.UpdateTheme)
	rg.POST("/:id/lessons/reorder", c.ReorderLessons)
	rg.DELETE("/:id", c.Delete)
}

// List handles GET /api/courses.
// @Summary List courses
// @Description Fetch a paginated list of courses with optional status and search filtering.
// @Tags Courses
// @Accept json
// @Produce json
// @Param page query int false "Page number (default: 1)"
// @Param limit query int false "Items per page (default: 20, max: 100)"
// @Param status query string false "Filter by status (draft|published)"
// @Param search query string false "Search in title or description"
// @Success 200 {object} response.Response{data=[]dto.CourseResponse}
// @Failure 500 {object} response.Response
// @Router /api/courses [get]
func (c *Controller) List(ctx *gin.Context) {
	tenantID := tenant.FromContext(ctx.Request.Context())

	page, _ := strconv.Atoi(ctx.Query("page"))
	limit, _ := strconv.Atoi(ctx.Query("limit"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if page < 1 {
		page = 1
	}
	offset := int64((page - 1) * limit)

	courses, total, err := c.svc.List(ctx.Request.Context(), tenantID, repository.CourseListParams{
		Status: ctx.Query("status"),
		Limit:  int64(limit),
		Offset: offset,
		Search: ctx.Query("search"),
	})
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("courses: List failed")
		response.InternalErrorGin(ctx, err)
		return
	}

	items := dto.ToResponses(courses)
	response.PaginatedGin(ctx, items, total, page, limit)
}

// GetBySubdomain handles GET /api/courses/subdomain/:sub.
// @Summary Get course by subdomain
// @Description Fetch course details and lessons by the tenant's slug/subdomain.
// @Tags Courses
// @Accept json
// @Produce json
// @Param sub path string true "Subdomain/Slug"
// @Success 200 {object} dto.CourseResponse
// @Failure 404 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /api/courses/subdomain/{sub} [get]
func (c *Controller) GetBySubdomain(ctx *gin.Context) {
	sub := ctx.Param("sub")

	course, err := c.svc.GetBySubdomain(ctx.Request.Context(), sub)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("courses: GetBySubdomain failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	if course == nil {
		response.NotFoundGin(ctx, "course not found")
		return
	}

	detail, err := c.svc.GetDetail(ctx.Request.Context(), course.ID.Hex())
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("courses: GetDetail failed")
		response.InternalErrorGin(ctx, err)
		return
	}

	response.SuccessGin(ctx, detail)
}

// Get handles GET /api/courses/:id.
// @Summary Get course by ID
// @Description Fetch full course details including lessons and settings.
// @Tags Courses
// @Accept json
// @Produce json
// @Param id path string true "Course ID"
// @Success 200 {object} dto.CourseResponse
// @Failure 404 {object} response.Response
// @Failure 500 {object} response.Response
// @Router /api/courses/{id} [get]
func (c *Controller) Get(ctx *gin.Context) {
	id := ctx.Param("id")

	detail, err := c.svc.GetDetail(ctx.Request.Context(), id)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("courses: Get failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	if detail == nil {
		response.NotFoundGin(ctx, "course not found")
		return
	}

	response.SuccessGin(ctx, detail)
}

// GetSchema handles GET /api/courses/:id/schema — returns JSON-LD schema.org markup.
// @Summary Get course schema
// @Description Returns JSON-LD schema.org markup for a course.
// @Tags Courses
// @Produce application/ld+json
// @Param id path string true "Course ID"
// @Success 200 {object} map[string]any
// @Router /api/courses/{id}/schema [get]
func (c *Controller) GetSchema(ctx *gin.Context) {
	id := ctx.Param("id")

	schema, err := c.svc.GetSchemaMarkup(ctx.Request.Context(), id)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("courses: GetSchema failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	if schema == nil {
		response.NotFoundGin(ctx, "course not found")
		return
	}

	ctx.Header("Content-Type", "application/ld+json; charset=utf-8")
	ctx.JSON(http.StatusOK, schema)
}

// Create handles POST /api/courses.
// @Summary Create a new course
// @Description Create a new course for the current tenant. Admin only.
// @Tags Courses
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param payload body dto.CreateCourseRequest true "Course Data"
// @Success 201 {object} dto.CourseResponse
// @Failure 400 {object} response.Response
// @Failure 401 {object} response.Response
// @Failure 403 {object} response.Response
// @Router /api/courses [post]
func (c *Controller) Create(ctx *gin.Context) {
	tenantID := tenant.FromContext(ctx.Request.Context())

	var req dto.CreateCourseRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	if err := dto.Validate(&req); err != nil {
		response.ValidationErrorGin(ctx, err)
		return
	}

	course, err := c.svc.Create(ctx.Request.Context(), tenantID, req)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("courses: Create failed")
		response.InternalErrorGin(ctx, err)
		return
	}

	response.CreatedGin(ctx, dto.ToResponse(course))
}

// Update handles PATCH /api/courses/:id.
// @Summary Update an existing course
// @Description Update course fields (title, description, status, etc.). Admin only.
// @Tags Courses
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Course ID"
// @Param payload body dto.UpdateCourseRequest true "Update Data"
// @Success 200 {object} dto.CourseResponse
// @Failure 400 {object} response.Response
// @Failure 404 {object} response.Response
// @Router /api/courses/{id} [patch]
func (c *Controller) Update(ctx *gin.Context) {
	id := ctx.Param("id")

	var req dto.UpdateCourseRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	if err := dto.Validate(&req); err != nil {
		response.ValidationErrorGin(ctx, err)
		return
	}

	course, err := c.svc.Update(ctx.Request.Context(), id, req)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("courses: Update failed")
		response.InternalErrorGin(ctx, err)
		return
	}

	response.SuccessGin(ctx, dto.ToResponse(course))
}

// UpdateTheme handles PATCH /api/courses/:id/theme.
// @Summary Update course theme
// @Description Updates the visual theme configuration for a course.
// @Tags Courses
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Course ID"
// @Success 200 {object} map[string]any
// @Router /api/courses/{id}/theme [patch]
func (c *Controller) UpdateTheme(ctx *gin.Context) {
	id := ctx.Param("id")

	var req dto.UpdateThemeRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	if err := dto.Validate(&req); err != nil {
		response.ValidationErrorGin(ctx, err)
		return
	}

	if err := c.svc.UpdateTheme(ctx.Request.Context(), id, req.ThemeConfig); err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("courses: UpdateTheme failed")
		response.InternalErrorGin(ctx, err)
		return
	}

	response.SuccessGin(ctx, map[string]any{"id": id, "theme_config": req.ThemeConfig})
}

// ReorderLessons handles POST /api/courses/:id/lessons/reorder.
// @Summary Reorder lessons
// @Description Reorders lessons within a course.
// @Tags Courses
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Course ID"
// @Success 200 {object} map[string]any
// @Router /api/courses/{id}/lessons/reorder [post]
func (c *Controller) ReorderLessons(ctx *gin.Context) {
	courseID := ctx.Param("id")

	var req dto.LessonReorderRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	if err := dto.Validate(&req); err != nil {
		response.ValidationErrorGin(ctx, err)
		return
	}

	if err := c.svc.ReorderLessons(ctx.Request.Context(), courseID, req.OrderedLessonIDs); err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("courses: ReorderLessons failed")
		response.InternalErrorGin(ctx, err)
		return
	}

	response.SuccessGin(ctx, map[string]any{"course_id": courseID, "status": "reordered"})
}

// Delete handles DELETE /api/courses/:id.
// @Summary Delete a course
// @Description Permanently delete a course and its associated lessons. Admin only.
// @Tags Courses
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Course ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} response.Response
// @Router /api/courses/{id} [delete]
func (c *Controller) Delete(ctx *gin.Context) {
	id := ctx.Param("id")

	if err := c.svc.Delete(ctx.Request.Context(), id); err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("courses: Delete failed")
		response.InternalErrorGin(ctx, err)
		return
	}

	response.SuccessGin(ctx, map[string]string{"id": id, "status": "deleted"})
}
