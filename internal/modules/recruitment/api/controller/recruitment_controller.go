// Package controller handles HTTP requests for the recruitment module.
package controller

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"erg.ninja/internal/dto/response"
	"erg.ninja/internal/middleware"
	"erg.ninja/internal/modules/recruitment/api/dto"
	"erg.ninja/pkg/auth"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/storage"
	"erg.ninja/pkg/tenant"
)

// Controller handles HTTP requests for recruitment.
type Controller struct {
	svc  Service
	log  *logger.Logger
	val  *validator.Validate
	auth *auth.JWTValidator
}

// NewController creates a new recruitment controller.
func NewController(svc Service, log *logger.Logger, jwtValidator *auth.JWTValidator) *Controller {
	return &Controller{
		svc:  svc,
		log:  log,
		val:  validator.New(),
		auth: jwtValidator,
	}
}

// Service is the recruitment service interface used by the controller.
type Service interface {
	CreateJob(ctx context.Context, req *dto.CreateJobRequest) (*dto.JobItemResponse, error)
	GetJobBySlug(ctx context.Context, slug string) (*dto.JobDetailResponse, error)
	ListJobs(ctx context.Context, params dto.JobQueryParams) (*dto.JobListResponse, error)
	UpdateJob(ctx context.Context, id string, req *dto.UpdateJobRequest) (*dto.JobItemResponse, error)
	DeleteJob(ctx context.Context, id string) error
	ToggleJobFlag(ctx context.Context, id, flag string) (*dto.JobItemResponse, error)
	UpdateJobStatus(ctx context.Context, id string, isActive bool) (*dto.JobItemResponse, error)
	Apply(ctx context.Context, tenantID string, req *dto.ApplyRequest, cvBuf []byte, cvFilename, cvMime string) (*dto.ApplyResponse, error)
	TrackApplication(ctx context.Context, code string) (*dto.TrackingResponse, error)
	ListCandidates(ctx context.Context, jobID string) (*dto.CandidateListResponse, error)
	UpdateCandidateStatus(ctx context.Context, id string, req *dto.UpdateCandidateStatusRequest) (*dto.CandidateItemResponse, error)
}

// RegisterRoutes mounts the recruitment REST API routes.
func (c *Controller) RegisterRoutes(r *gin.Engine) {
	api := r.Group("/api/recruitment")
	// Public routes — no auth required.
	api.GET("/jobs", c.ListJobs)
	api.GET("/jobs/:slug", c.GetJobBySlug)
	api.GET("/tracking/:code", c.TrackApplication)
	api.POST("/apply", c.Apply)

	// Admin routes — JWT auth applied via middleware.
	admin := api.Group("")
	admin.Use(middleware.JWTMiddleware(c.auth), middleware.RequireRoles("admin"))
	admin.POST("/jobs", c.CreateJob)
	admin.PUT("/jobs/:id", c.UpdateJob)
	admin.DELETE("/jobs/:id", c.DeleteJob)
	admin.PATCH("/jobs/:id/toggle-hot", c.ToggleHot)
	admin.PATCH("/jobs/:id/toggle-urgent", c.ToggleUrgent)
	admin.PATCH("/jobs/:id/status", c.UpdateJobStatus)

	// Candidate management.
	admin.GET("/admin/candidates", c.ListCandidates)
	admin.PATCH("/admin/candidates/:id/status", c.UpdateCandidateStatus)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func (c *Controller) getTenant(ctx *gin.Context) string {
	return tenant.FromContext(ctx.Request.Context())
}

// parseQueryInt parses a query param as int, returns default if absent or invalid.
func parseQueryInt(ctx *gin.Context, key string, deflt int) int {
	s := ctx.Query(key)
	if s == "" {
		return deflt
	}
	i, err := strconv.Atoi(s)
	if err != nil || i < 1 {
		return deflt
	}
	return i
}

// parseQueryStrings parses a repeated query param into a deduplicated slice.
func parseQueryStrings(ctx *gin.Context, key string) []string {
	vals := ctx.QueryArray(key)
	if len(vals) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	var out []string
	for _, v := range vals {
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

// ─── Public Handlers ──────────────────────────────────────────────────────────

// ListJobs handles GET /api/recruitment/jobs.
// @Summary List jobs
// @Description Returns paginated job listings.
// @Tags Recruitment
// @Produce json
// @Param page query int false "Page"
// @Param limit query int false "Limit"
// @Param search query string false "Search"
// @Success 200 {object} map[string]any
// @Router /api/recruitment/jobs [get]
func (c *Controller) ListJobs(ctx *gin.Context) {
	params := dto.JobQueryParams{
		Page:     parseQueryInt(ctx, "page", 1),
		Limit:    parseQueryInt(ctx, "limit", 10),
		Search:   ctx.Query("search"),
		Salary:   parseQueryStrings(ctx, "salary"),
		WorkType: parseQueryStrings(ctx, "work_type"),
		Location: parseQueryStrings(ctx, "location"),
		Sort:     ctx.Query("sort"),
	}
	if params.Sort == "" {
		params.Sort = "newest"
	}

	result, err := c.svc.ListJobs(ctx.Request.Context(), params)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("recruitment: ListJobs failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, result)
}

// GetJobBySlug handles GET /api/recruitment/jobs/:slug.
// @Summary Get job by slug
// @Description Returns a job post by its slug.
// @Tags Recruitment
// @Produce json
// @Param slug path string true "Job slug"
// @Success 200 {object} map[string]any
// @Router /api/recruitment/jobs/{slug} [get]
func (c *Controller) GetJobBySlug(ctx *gin.Context) {
	slug := ctx.Param("slug")
	if slug == "" {
		response.BadRequestGin(ctx, fmt.Errorf("slug is required"))
		return
	}

	result, err := c.svc.GetJobBySlug(ctx.Request.Context(), slug)
	if err != nil {
		if IsNotFound(err) {
			response.NotFoundGin(ctx, "job not found")
			return
		}
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Str("slug", slug).Msg("recruitment: GetJobBySlug failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, result)
}

// Apply handles POST /api/recruitment/apply (multipart/form-data, CV upload).
// @Summary Apply for a job
// @Description Submit a job application with CV upload.
// @Tags Recruitment
// @Accept multipart/form-data
// @Produce json
// @Success 201 {object} map[string]any
// @Router /api/recruitment/apply [post]
func (c *Controller) Apply(ctx *gin.Context) {
	tenantID := c.getTenant(ctx)

	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, storage.MaxRequestBytes(storage.UploadKindDocument, 2<<20))
	if err := ctx.Request.ParseMultipartForm(storage.MultipartMemoryLimit); err != nil {
		response.BadRequestGin(ctx, fmt.Errorf("parse multipart form: %w", err))
		return
	}

	fullName := ctx.Request.FormValue("full_name")
	email := ctx.Request.FormValue("email")
	phone := ctx.Request.FormValue("phone")
	coverLetter := ctx.Request.FormValue("cover_letter")
	trackingURL := ctx.Request.FormValue("tracking_url")
	jobID := ctx.Request.FormValue("job_id")

	if fullName == "" || email == "" || phone == "" {
		response.BadRequestGin(ctx, fmt.Errorf("full_name, email, and phone are required"))
		return
	}

	file, header, err := ctx.Request.FormFile("file")
	if err != nil {
		response.BadRequestGin(ctx, fmt.Errorf("CV file (field 'file') is required"))
		return
	}
	defer file.Close()

	cvBuf, err := io.ReadAll(file)
	if err != nil {
		response.BadRequestGin(ctx, fmt.Errorf("read CV file: %w", err))
		return
	}

	cvMime := header.Header.Get("Content-Type")
	if cvMime == "" {
		cvMime = "application/octet-stream"
	}

	req := &dto.ApplyRequest{
		JobID:       jobID,
		FullName:    fullName,
		Email:       email,
		Phone:       phone,
		CoverLetter: coverLetter,
		TrackingURL: trackingURL,
	}

	result, err := c.svc.Apply(ctx.Request.Context(), tenantID, req, cvBuf, header.Filename, cvMime)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("recruitment: Apply failed")
		response.BadRequestGin(ctx, fmt.Errorf("%s", err.Error()))
		return
	}

	response.CreatedGin(ctx, result)
}

// TrackApplication handles GET /api/recruitment/tracking/:code.
// @Summary Track application
// @Description Track a job application by its tracking code.
// @Tags Recruitment
// @Produce json
// @Param code path string true "Tracking code"
// @Success 200 {object} map[string]any
// @Router /api/recruitment/tracking/{code} [get]
func (c *Controller) TrackApplication(ctx *gin.Context) {
	code := ctx.Param("code")
	if code == "" {
		response.BadRequestGin(ctx, fmt.Errorf("tracking code is required"))
		return
	}

	result, err := c.svc.TrackApplication(ctx.Request.Context(), code)
	if err != nil {
		if IsNotFound(err) {
			response.NotFoundGin(ctx, "Không tìm thấy hồ sơ ứng tuyển")
			return
		}
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Str("code", code).Msg("recruitment: TrackApplication failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, result)
}

// ─── Admin Handlers ──────────────────────────────────────────────────────────

// CreateJob handles POST /api/recruitment/jobs.
// @Summary Create a job
// @Description Creates a new job posting.
// @Tags Recruitment Admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 201 {object} map[string]any
// @Router /api/recruitment/jobs [post]
func (c *Controller) CreateJob(ctx *gin.Context) {
	tenantID := c.getTenant(ctx)

	var req dto.CreateJobRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	if err := dto.Validate(&req); err != nil {
		response.ValidationErrorGin(ctx, err)
		return
	}

	req.TenantID = tenantID
	if claims := middleware.GetClaims(ctx.Request.Context()); claims != nil {
		req.CreatedBy = claims.Subject
	}

	job, err := c.svc.CreateJob(ctx.Request.Context(), &req)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("recruitment: CreateJob failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	response.CreatedGin(ctx, job)
}

// UpdateJob handles PUT /api/recruitment/jobs/:id.
// @Summary Update a job
// @Description Updates an existing job posting.
// @Tags Recruitment Admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Job ID"
// @Success 200 {object} map[string]any
// @Router /api/recruitment/jobs/{id} [put]
func (c *Controller) UpdateJob(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		response.BadRequestGin(ctx, fmt.Errorf("job id is required"))
		return
	}

	var req dto.UpdateJobRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}

	job, err := c.svc.UpdateJob(ctx.Request.Context(), id, &req)
	if err != nil {
		if IsNotFound(err) {
			response.NotFoundGin(ctx, "job not found")
			return
		}
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Str("id", id).Msg("recruitment: UpdateJob failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, job)
}

// DeleteJob handles DELETE /api/recruitment/jobs/:id (soft delete).
// @Summary Delete a job
// @Description Soft-deletes a job posting.
// @Tags Recruitment Admin
// @Produce json
// @Security BearerAuth
// @Param id path string true "Job ID"
// @Success 200 {object} map[string]any
// @Router /api/recruitment/jobs/{id} [delete]
func (c *Controller) DeleteJob(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		response.BadRequestGin(ctx, fmt.Errorf("job id is required"))
		return
	}

	if err := c.svc.DeleteJob(ctx.Request.Context(), id); err != nil {
		if IsNotFound(err) {
			response.NotFoundGin(ctx, "job not found")
			return
		}
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Str("id", id).Msg("recruitment: DeleteJob failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, map[string]string{"id": id, "status": "deleted"})
}

// ToggleHot handles PATCH /api/recruitment/jobs/:id/toggle-hot.
// @Summary Toggle hot flag
// @Description Toggles the hot flag on a job.
// @Tags Recruitment Admin
// @Produce json
// @Security BearerAuth
// @Param id path string true "Job ID"
// @Success 200 {object} map[string]any
// @Router /api/recruitment/jobs/{id}/toggle-hot [patch]
func (c *Controller) ToggleHot(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		response.BadRequestGin(ctx, fmt.Errorf("job id is required"))
		return
	}

	job, err := c.svc.ToggleJobFlag(ctx.Request.Context(), id, "isHot")
	if err != nil {
		if IsNotFound(err) {
			response.NotFoundGin(ctx, "job not found")
			return
		}
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Str("id", id).Msg("recruitment: ToggleHot failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, job)
}

// ToggleUrgent handles PATCH /api/recruitment/jobs/:id/toggle-urgent.
// @Summary Toggle urgent flag
// @Description Toggles the urgent flag on a job.
// @Tags Recruitment Admin
// @Produce json
// @Security BearerAuth
// @Param id path string true "Job ID"
// @Success 200 {object} map[string]any
// @Router /api/recruitment/jobs/{id}/toggle-urgent [patch]
func (c *Controller) ToggleUrgent(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		response.BadRequestGin(ctx, fmt.Errorf("job id is required"))
		return
	}

	job, err := c.svc.ToggleJobFlag(ctx.Request.Context(), id, "isUrgent")
	if err != nil {
		if IsNotFound(err) {
			response.NotFoundGin(ctx, "job not found")
			return
		}
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Str("id", id).Msg("recruitment: ToggleUrgent failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, job)
}

// UpdateJobStatus handles PATCH /api/recruitment/jobs/:id/status.
// @Summary Update job status
// @Description Activates or deactivates a job.
// @Tags Recruitment Admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Job ID"
// @Success 200 {object} map[string]any
// @Router /api/recruitment/jobs/{id}/status [patch]
func (c *Controller) UpdateJobStatus(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		response.BadRequestGin(ctx, fmt.Errorf("job id is required"))
		return
	}

	var body struct {
		IsActive *bool `json:"is_active"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	if body.IsActive == nil {
		response.BadRequestGin(ctx, fmt.Errorf("is_active is required"))
		return
	}

	job, err := c.svc.UpdateJobStatus(ctx.Request.Context(), id, *body.IsActive)
	if err != nil {
		if IsNotFound(err) {
			response.NotFoundGin(ctx, "job not found")
			return
		}
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Str("id", id).Msg("recruitment: UpdateJobStatus failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, job)
}

// ListCandidates handles GET /api/recruitment/admin/candidates.
// @Summary List candidates
// @Description Returns candidates for a job.
// @Tags Recruitment Admin
// @Produce json
// @Security BearerAuth
// @Param jobId query string false "Job ID filter"
// @Success 200 {object} map[string]any
// @Router /api/recruitment/admin/candidates [get]
func (c *Controller) ListCandidates(ctx *gin.Context) {
	jobID := ctx.Query("jobId")

	result, err := c.svc.ListCandidates(ctx.Request.Context(), jobID)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("recruitment: ListCandidates failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, result)
}

// UpdateCandidateStatus handles PATCH /api/recruitment/admin/candidates/:id/status.
// @Summary Update candidate status
// @Description Updates the status of a candidate application.
// @Tags Recruitment Admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Candidate ID"
// @Success 200 {object} map[string]any
// @Router /api/recruitment/admin/candidates/{id}/status [patch]
func (c *Controller) UpdateCandidateStatus(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		response.BadRequestGin(ctx, fmt.Errorf("candidate id is required"))
		return
	}

	var req dto.UpdateCandidateStatusRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	if err := dto.Validate(&req); err != nil {
		response.ValidationErrorGin(ctx, err)
		return
	}

	candidate, err := c.svc.UpdateCandidateStatus(ctx.Request.Context(), id, &req)
	if err != nil {
		if IsNotFound(err) {
			response.NotFoundGin(ctx, "candidate not found")
			return
		}
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Str("id", id).Msg("recruitment: UpdateCandidateStatus failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	response.OKGin(ctx, candidate)
}

// IsNotFound returns true if err is a repository not-found sentinel.
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "not found")
}
