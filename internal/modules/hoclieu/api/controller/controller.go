package controller

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/dto/response"
	"erg.ninja/internal/middleware"
	. "erg.ninja/internal/modules/hoclieu/api/dto"
	. "erg.ninja/internal/modules/hoclieu/application/service"
)

type Controller struct {
	svc *Service
}

func NewController(svc *Service) *Controller {
	return &Controller{svc: svc}
}

func (c *Controller) RegisterPublicRoutes(rg *gin.RouterGroup) {
	rg.GET("/home", c.Home)
	rg.GET("/programs", c.Programs)
	rg.GET("/programs/:slug", c.Program)
	rg.GET("/taxonomies", c.Taxonomy)
	rg.GET("/portfolio", c.Portfolio)
	rg.GET("/community", c.Community)
}

func (c *Controller) RegisterProtectedRoutes(rg *gin.RouterGroup) {
	teacher := rg.Group("/teacher")
	teacher.GET("/subjects/:subjectId/tree", middleware.RequireAccessPermission("hoclieu.resource.read"), c.TeacherSubjectTree)
	teacher.GET("/recent-opened", middleware.RequireAccessPermission("hoclieu.resource.read"), c.TeacherRecentOpened)
	teacher.GET("/progress", middleware.RequireAccessPermission("hoclieu.resource.read"), c.TeacherProgress)
	teacher.POST("/progress-events", middleware.RequireAccessPermission("hoclieu.resource.read"), c.TrackTeacherProgressEvent)

	rg.GET("/resources", middleware.RequireAccessPermission("hoclieu.resource.read"), c.ListResources)
	rg.POST("/resources", middleware.RequireAccessPermission("hoclieu.resource.create"), c.CreateResource)
	rg.PATCH("/resources/:resourceId", middleware.RequireAccessPermission("hoclieu.resource.update"), c.UpdateResource)
	rg.DELETE("/resources/:resourceId", middleware.RequireAccessPermission("hoclieu.resource.delete"), c.DeleteResource)
	rg.POST("/resources/upload", middleware.RequireAccessPermission("hoclieu.asset.upload"), c.UploadResource)
	rg.GET("/resources/:resourceId", middleware.RequireAccessPermission("hoclieu.resource.read"), c.Resource)
	rg.GET("/resources/:resourceId/items", middleware.RequireAccessPermission("hoclieu.resource.read"), c.ResourceItems)
	rg.GET("/quizzes", middleware.RequireAccessPermission("hoclieu.resource.read"), c.Quizzes)
	rg.POST("/assets", middleware.RequireAccessPermission("hoclieu.asset.upload"), c.CreateAsset)
	rg.POST("/assets/upload", middleware.RequireAccessPermission("hoclieu.asset.upload"), c.UploadAsset)
	rg.PATCH("/assets/:assetId", middleware.RequireAccessPermission("hoclieu.asset.manage"), c.UpdateAsset)
	rg.GET("/assets/:assetId/launch", middleware.RequireAccessPermission("hoclieu.asset.launch"), c.LaunchAsset)
	rg.GET("/assets/:assetId/pages", middleware.RequireAccessPermission("hoclieu.asset.launch"), c.AssetPages)
	rg.GET("/assets/:assetId/stream", middleware.RequireAccessPermission("hoclieu.asset.launch"), c.StreamAsset)
	rg.GET("/assets/:assetId/download", middleware.RequireAccessPermission("hoclieu.asset.download"), c.DownloadAsset)

	admin := rg.Group("/admin")
	admin.GET("/content-model", middleware.RequireAccessPermission("hoclieu.resource.read"), c.Taxonomy)
	admin.GET("/taxonomies", middleware.RequireAccessPermission("hoclieu.resource.read"), c.Taxonomy)
	admin.GET("/taxonomy", middleware.RequireAccessPermission("hoclieu.resource.read"), c.Taxonomy)
	admin.GET("/taxonomies/:kind", middleware.RequireAccessPermission("hoclieu.resource.read"), c.ListTaxonomyOptions)
	admin.GET("/taxonomy/:kind", middleware.RequireAccessPermission("hoclieu.resource.read"), c.ListTaxonomyOptions)
	admin.POST("/taxonomies/:kind", middleware.RequireAccessPermission("hoclieu.resource.create"), c.CreateTaxonomyOption)
	admin.POST("/taxonomy/:kind", middleware.RequireAccessPermission("hoclieu.resource.create"), c.CreateTaxonomyOption)
	admin.PATCH("/taxonomies/:kind/:id", middleware.RequireAccessPermission("hoclieu.resource.update"), c.UpdateTaxonomyOption)
	admin.PATCH("/taxonomy/:kind/:id", middleware.RequireAccessPermission("hoclieu.resource.update"), c.UpdateTaxonomyOption)
	admin.DELETE("/taxonomies/:kind/:id", middleware.RequireAccessPermission("hoclieu.resource.update"), c.DeleteTaxonomyOption)
	admin.DELETE("/taxonomy/:kind/:id", middleware.RequireAccessPermission("hoclieu.resource.update"), c.DeleteTaxonomyOption)
	admin.POST("/resources", middleware.RequireAccessPermission("hoclieu.resource.create"), c.CreateResource)
	admin.PATCH("/resources/:resourceId", middleware.RequireAccessPermission("hoclieu.resource.update"), c.UpdateResource)
	admin.DELETE("/resources/:resourceId", middleware.RequireAccessPermission("hoclieu.resource.delete"), c.DeleteResource)
	admin.POST("/resources/upload", middleware.RequireAccessPermission("hoclieu.asset.upload"), c.UploadResource)
	admin.POST("/assets/upload", middleware.RequireAccessPermission("hoclieu.asset.upload"), c.UploadAsset)
}

func (c *Controller) Home(ctx *gin.Context) {
	response.SuccessGin(ctx, c.svc.Home(ctx.Request.Context()))
}

func (c *Controller) Programs(ctx *gin.Context) {
	response.SuccessGin(ctx, c.svc.Programs(ctx.Request.Context()))
}

func (c *Controller) Program(ctx *gin.Context) {
	program, err := c.svc.Program(ctx.Request.Context(), ctx.Param("slug"))
	c.respond(ctx, program, err, http.StatusOK)
}

func (c *Controller) Taxonomy(ctx *gin.Context) {
	response.SuccessGin(ctx, c.svc.Taxonomy(ctx.Request.Context()))
}

func (c *Controller) ListResources(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.Query("page"))
	limit, _ := strconv.Atoi(ctx.Query("limit"))
	items, total := c.svc.ListResources(ctx.Request.Context(), ListResourceParams{
		GradeID:        ctx.Query("gradeId"),
		SubjectID:      ctx.Query("subjectId"),
		CategoryID:     ctx.Query("categoryId"),
		SectionID:      ctx.Query("sectionId"),
		BookSeriesID:   ctx.Query("bookSeriesId"),
		TopicID:        ctx.Query("topicId"),
		LevelID:        ctx.Query("levelId"),
		DocumentTypeID: ctx.Query("documentTypeId"),
		ProgramSlug:    ctx.Query("programSlug"),
		FileType:       AssetFileType(ctx.Query("fileType")),
		Query:          ctx.Query("query"),
		Page:           page,
		Limit:          limit,
	})
	if page < 1 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	response.PaginatedGin(ctx, items, total, page, limit)
}

func (c *Controller) DeleteResource(ctx *gin.Context) {
	err := c.svc.DeleteResource(ctx.Request.Context(), ctx.Param("resourceId"))
	c.respond(ctx, gin.H{"deleted": err == nil}, err, http.StatusOK)
}

func (c *Controller) CreateResource(ctx *gin.Context) {
	var req CreateResourceRequestDTO
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	req.ActorID = middleware.GetUserID(ctx.Request.Context())
	resource, err := c.svc.CreateResource(ctx.Request.Context(), req)
	c.respond(ctx, resource, err, http.StatusCreated)
}

func (c *Controller) UpdateResource(ctx *gin.Context) {
	var req UpdateResourceRequestDTO
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	req.ActorID = middleware.GetUserID(ctx.Request.Context())
	resource, err := c.svc.UpdateResource(ctx.Request.Context(), ctx.Param("resourceId"), req)
	c.respond(ctx, resource, err, http.StatusOK)
}

func (c *Controller) Resource(ctx *gin.Context) {
	resource, err := c.svc.Resource(ctx.Request.Context(), ctx.Param("resourceId"))
	c.respond(ctx, resource, err, http.StatusOK)
}

func (c *Controller) ResourceItems(ctx *gin.Context) {
	items, err := c.svc.ResourceItems(ctx.Request.Context(), ctx.Param("resourceId"), ctx.Query("query"))
	c.respond(ctx, items, err, http.StatusOK)
}

func (c *Controller) TeacherSubjectTree(ctx *gin.Context) {
	tree, err := c.svc.SubjectTree(
		ctx.Request.Context(),
		ctx.Param("subjectId"),
		ctx.Query("schoolId"),
		ctx.Query("academicYear"),
		ctx.Query("parentId"),
	)
	c.respond(ctx, tree, err, http.StatusOK)
}

func (c *Controller) TeacherRecentOpened(ctx *gin.Context) {
	limit, _ := strconv.Atoi(ctx.Query("limit"))
	items := c.svc.RecentOpened(ctx.Request.Context(), ctx.Query("schoolId"), ctx.Query("academicYear"), limit)
	response.SuccessGin(ctx, items)
}

func (c *Controller) TeacherProgress(ctx *gin.Context) {
	progress, err := c.svc.Progress(
		ctx.Request.Context(),
		ctx.Query("subjectId"),
		ctx.Query("nodeId"),
		ctx.Query("schoolId"),
		ctx.Query("academicYear"),
	)
	c.respond(ctx, progress, err, http.StatusOK)
}

func (c *Controller) TrackTeacherProgressEvent(ctx *gin.Context) {
	var req TrackTeacherProgressEventRequestDTO
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	req.TeacherID = middleware.GetUserID(ctx.Request.Context())
	event, err := c.svc.TrackProgressEvent(ctx.Request.Context(), req)
	c.respond(ctx, event, err, http.StatusCreated)
}

func (c *Controller) Portfolio(ctx *gin.Context) {
	response.SuccessGin(ctx, c.svc.Portfolio(ctx.Request.Context()))
}

func (c *Controller) Community(ctx *gin.Context) {
	response.SuccessGin(ctx, c.svc.Community(ctx.Request.Context()))
}

func (c *Controller) Quizzes(ctx *gin.Context) {
	response.SuccessGin(ctx, c.svc.Quizzes(ctx.Request.Context()))
}

func (c *Controller) CreateAsset(ctx *gin.Context) {
	var req CreateAssetRequestDTO
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	req.ActorID = middleware.GetUserID(ctx.Request.Context())
	asset, err := c.svc.CreateAsset(ctx.Request.Context(), req)
	c.respond(ctx, asset, err, http.StatusCreated)
}

func (c *Controller) UploadAsset(ctx *gin.Context) {
	const maxHocLieuUploadBytes int64 = 250 << 20

	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, maxHocLieuUploadBytes+(1<<20))
	if err := ctx.Request.ParseMultipartForm(1 << 20); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	fileHeader, err := ctx.FormFile("file")
	if err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	defer file.Close()

	buf, err := io.ReadAll(io.LimitReader(file, maxHocLieuUploadBytes+1))
	if err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	if int64(len(buf)) > maxHocLieuUploadBytes {
		response.BadRequestGin(ctx, errors.New("hoclieu upload exceeds 250MB"))
		return
	}

	req := UploadAssetRequestDTO{
		ResourceID:       ctx.PostForm("resourceId"),
		Title:            ctx.PostForm("title"),
		SelectedFileType: AssetFileType(ctx.PostForm("selectedFileType")),
		CanDownload:      ctx.PostForm("canDownload") == "true",
		ActorID:          middleware.GetUserID(ctx.Request.Context()),
	}
	if req.ResourceID == "" || !req.SelectedFileType.Valid() {
		response.BadRequestGin(ctx, errors.New("resourceId and selectedFileType are required"))
		return
	}
	detectedMime := fileHeader.Header.Get("Content-Type")
	if detectedMime == "" {
		detectedMime = http.DetectContentType(buf)
	}
	asset, err := c.svc.UploadAsset(ctx.Request.Context(), req, buf, fileHeader.Filename, detectedMime)
	c.respond(ctx, asset, err, http.StatusCreated)
}

func (c *Controller) UploadResource(ctx *gin.Context) {
	const maxHocLieuUploadBytes int64 = 250 << 20

	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, maxHocLieuUploadBytes+(1<<20))
	if err := ctx.Request.ParseMultipartForm(1 << 20); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	fileHeader, err := ctx.FormFile("file")
	if err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	defer file.Close()

	buf, err := io.ReadAll(io.LimitReader(file, maxHocLieuUploadBytes+1))
	if err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	if int64(len(buf)) > maxHocLieuUploadBytes {
		response.BadRequestGin(ctx, errors.New("hoclieu upload exceeds 250MB"))
		return
	}

	selectedFileType := AssetFileType(ctx.PostForm("selectedFileType"))
	req := CreateResourceRequestDTO{
		Title:            ctx.PostForm("title"),
		Slug:             ctx.PostForm("slug"),
		Subtitle:         ctx.PostForm("subtitle"),
		Description:      ctx.PostForm("description"),
		ThumbnailURL:     ctx.PostForm("thumbnailUrl"),
		ProgramSlug:      ctx.PostForm("programSlug"),
		SubjectID:        ctx.PostForm("subjectId"),
		GradeID:          ctx.PostForm("gradeId"),
		CategoryID:       ctx.PostForm("categoryId"),
		SectionID:        ctx.PostForm("sectionId"),
		BookSeriesID:     ctx.PostForm("bookSeriesId"),
		TopicID:          ctx.PostForm("topicId"),
		LevelID:          ctx.PostForm("levelId"),
		DocumentTypeID:   ctx.PostForm("documentTypeId"),
		SelectedFileType: selectedFileType,
		PriceType:        ctx.PostForm("priceType"),
		Visibility:       ctx.PostForm("visibility"),
		Status:           ctx.PostForm("status"),
		Tags:             splitCSV(ctx.PostForm("tags")),
		ActorID:          middleware.GetUserID(ctx.Request.Context()),
	}
	if ctx.PostForm("canDownload") != "" {
		canDownload := ctx.PostForm("canDownload") == "true"
		req.CanDownload = &canDownload
	}
	if strings.TrimSpace(req.SubjectID) == "" || strings.TrimSpace(req.CategoryID) == "" || !req.SelectedFileType.Valid() {
		response.BadRequestGin(ctx, errors.New("subjectId, categoryId and selectedFileType are required"))
		return
	}
	detectedMime := fileHeader.Header.Get("Content-Type")
	if detectedMime == "" {
		detectedMime = http.DetectContentType(buf)
	}
	result, err := c.svc.CreateResourceWithUpload(ctx.Request.Context(), req, buf, fileHeader.Filename, detectedMime)
	c.respond(ctx, result, err, http.StatusCreated)
}

func (c *Controller) CreateTaxonomyOption(ctx *gin.Context) {
	var req CreateTaxonomyOptionRequestDTO
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	option, err := c.svc.CreateTaxonomyOption(ctx.Request.Context(), ctx.Param("kind"), req)
	c.respond(ctx, option, err, http.StatusCreated)
}

func (c *Controller) ListTaxonomyOptions(ctx *gin.Context) {
	options, err := c.svc.ListTaxonomyOptions(ctx.Request.Context(), ctx.Param("kind"))
	c.respond(ctx, options, err, http.StatusOK)
}

func (c *Controller) UpdateTaxonomyOption(ctx *gin.Context) {
	var req UpdateTaxonomyOptionRequestDTO
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	option, err := c.svc.UpdateTaxonomyOption(ctx.Request.Context(), ctx.Param("kind"), ctx.Param("id"), req)
	c.respond(ctx, option, err, http.StatusOK)
}

func (c *Controller) DeleteTaxonomyOption(ctx *gin.Context) {
	err := c.svc.DeleteTaxonomyOption(ctx.Request.Context(), ctx.Param("kind"), ctx.Param("id"))
	c.respond(ctx, gin.H{"deleted": err == nil}, err, http.StatusOK)
}

func (c *Controller) UpdateAsset(ctx *gin.Context) {
	var req UpdateAssetRequestDTO
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, err)
		return
	}
	req.ActorID = middleware.GetUserID(ctx.Request.Context())
	asset, err := c.svc.UpdateAsset(ctx.Request.Context(), ctx.Param("assetId"), req)
	c.respond(ctx, asset, err, http.StatusOK)
}

func (c *Controller) LaunchAsset(ctx *gin.Context) {
	launch, err := c.svc.Launch(ctx.Request.Context(), ctx.Param("assetId"), middleware.GetUserID(ctx.Request.Context()))
	c.respond(ctx, launch, err, http.StatusOK)
}

func (c *Controller) AssetPages(ctx *gin.Context) {
	pages, err := c.svc.Pages(ctx.Request.Context(), ctx.Param("assetId"))
	c.respond(ctx, pages, err, http.StatusOK)
}

func (c *Controller) StreamAsset(ctx *gin.Context) {
	asset, err := c.svc.Asset(ctx.Request.Context(), ctx.Param("assetId"))
	if err != nil {
		c.respond(ctx, nil, err, http.StatusOK)
		return
	}
	writeAuditHeaders(ctx, "hoclieu.asset.stream", asset, middleware.GetUserID(ctx.Request.Context()))
	writeAssetStream(ctx, asset)
}

func (c *Controller) DownloadAsset(ctx *gin.Context) {
	asset, err := c.svc.Asset(ctx.Request.Context(), ctx.Param("assetId"))
	if err != nil {
		c.respond(ctx, nil, err, http.StatusOK)
		return
	}
	writeAuditHeaders(ctx, "hoclieu.asset.download", asset, middleware.GetUserID(ctx.Request.Context()))
	if !asset.CanDownload {
		ctx.Header("X-Hoclieu-Can-Download", "false")
		response.ForbiddenGin(ctx)
		return
	}
	writeAssetDownload(ctx, asset)
}

func (c *Controller) respond(ctx *gin.Context, data any, err error, successStatus int) {
	if err == nil {
		if successStatus == http.StatusCreated {
			response.CreatedGin(ctx, data)
			return
		}
		response.SuccessGin(ctx, data)
		return
	}
	switch {
	case errors.Is(err, ErrNotFound):
		response.NotFoundGin(ctx, err.Error())
	case errors.Is(err, ErrInvalidFileType), errors.Is(err, ErrInvalidAssetMetadata), errors.Is(err, ErrInvalidRequest):
		response.BadRequestGin(ctx, err)
	default:
		response.InternalErrorGin(ctx, err)
	}
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
