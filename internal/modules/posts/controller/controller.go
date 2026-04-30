// Package controller handles HTTP requests for the posts module.
package controller

import (
	"fmt"
	"io"
	"strconv"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/dto/response"
	"erg.ninja/internal/middleware"
	"erg.ninja/internal/modules/posts/dto"
	"erg.ninja/internal/modules/posts/entities"
	"erg.ninja/internal/modules/posts/service"
	"erg.ninja/pkg/auth"
	"erg.ninja/pkg/logger"
)

// Controller handles HTTP requests for posts.
type Controller struct {
	svc          *service.Service
	log          *logger.Logger
	JWTValidator *auth.JWTValidator
}

// NewController creates a new posts controller.
func NewController(svc *service.Service, jwtValidator *auth.JWTValidator, log *logger.Logger) *Controller {
	return &Controller{svc: svc, JWTValidator: jwtValidator, log: log}
}

// RegisterRoutes mounts the posts REST API routes.
func (c *Controller) RegisterRoutes(r *gin.Engine) {
	api := r.Group("/api/posts")

	// ── Public routes ────────────────────────────────────────────────────
	api.GET("/", c.ListPosts)
	api.GET("/:id", c.GetPostByID)
	api.GET("/slug/:slug", c.GetPostBySlug)
	api.GET("/preview/:id", c.GetPreview)

	// ── Authenticated routes ─────────────────────────────────────────────
	protected := api.Group("")
	protected.Use(middleware.JWTMiddleware(c.JWTValidator), middleware.RequireRoles("admin", "moderator", "editor"))
	protected.POST("/", c.CreatePost)
	protected.PUT("/:id", c.UpdatePost)
	protected.DELETE("/:id", c.SoftDeletePost)
	protected.PUT("/:id/restore", c.RestorePost)
	protected.DELETE("/:id/permanent", c.HardDeletePost)
	protected.POST("/:id/promote", c.PromotePost)
	protected.POST("/preview", c.SavePreview)
	protected.GET("/hidden", c.ListHiddenPosts)
	protected.GET("/trash", c.ListTrash)
	protected.POST("/images/upload", c.UploadImage)
	protected.DELETE("/images", c.DeleteImage)
	protected.DELETE("/images/id/:filename", c.DeleteImageByFilename)

	// ── Category routes ─────────────────────────────────────────────────
	api.GET("/categories", c.ListCategories)
	api.GET("/categories/:id", c.GetCategory)

	admin := api.Group("/categories")
	admin.Use(middleware.JWTMiddleware(c.JWTValidator), middleware.RequireRoles("admin", "moderator", "editor"))
	admin.POST("", c.CreateCategory)
	admin.PUT("/:id", c.UpdateCategory)
	admin.DELETE("/:id", c.DeleteCategory)
}

// ─── Auth Middleware ─────────────────────────────────────────────────────────

// ─── Post Handlers ──────────────────────────────────────────────────────────

// ListPosts handles GET /api/posts.
// @Summary List all posts
// @Description Fetch paged and filtered list of posts.
// @Tags Posts
// @Accept json
// @Produce json
// @Param page query int false "Page number"
// @Param limit query int false "Items per page"
// @Param search query string false "Search query"
// @Param category query string false "Filter by category slug"
// @Param status query string false "Filter by status"
// @Success 200 {object} response.Response{data=[]dto.PostResponse}
// @Router /api/posts [get]
func (c *Controller) ListPosts(ctx *gin.Context) {
	q := dto.PostQueryParams{
		Page:       page(ctx, "page", 1),
		Limit:      page(ctx, "limit", 10),
		Search:     ctx.Query("search"),
		Category:   ctx.Query("category"),
		CategoryID: ctx.Query("category_id"),
		Status:     ctx.Query("status"),
		SortBy:     ctx.Query("sortBy"),
		Order:      ctx.Query("order"),
	}

	posts, total, err := c.svc.ListPosts(ctx.Request.Context(), q)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("posts: ListPosts failed")
		response.InternalErrorGin(ctx, err)
		return
	}

	items := make([]dto.PostResponse, len(posts))
	for i, p := range posts {
		items[i] = toPostResponse(p)
	}

	response.PaginatedGin(ctx, items, total, q.Page, q.Limit)
}

// GetPostByID handles GET /api/posts/:id.
// @Summary Get post by ID
// @Description Fetch a single post by its internal ID.
// @Tags Posts
// @Accept json
// @Produce json
// @Param id path string true "Post ID"
// @Success 200 {object} dto.PostResponse
// @Failure 404 {object} response.Response
// @Router /api/posts/{id} [get]
func (c *Controller) GetPostByID(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		response.BadRequestGin(ctx, fmt.Errorf("id is required"))
		return
	}

	post, err := c.svc.GetPostByID(ctx.Request.Context(), id)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Str("id", id).Msg("posts: GetPostByID failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	if post == nil {
		response.NotFoundGin(ctx, "post not found")
		return
	}

	response.SuccessGin(ctx, toPostResponse(post))
}

// GetPostBySlug handles GET /api/posts/slug/:slug.
// @Summary Get post by slug
// @Description Fetch a single post by its URL slug.
// @Tags Posts
// @Accept json
// @Produce json
// @Param slug path string true "Post Slug"
// @Success 200 {object} dto.PostResponse
// @Failure 404 {object} response.Response
// @Router /api/posts/slug/{slug} [get]
func (c *Controller) GetPostBySlug(ctx *gin.Context) {
	slug := ctx.Param("slug")
	if slug == "" {
		response.BadRequestGin(ctx, fmt.Errorf("slug is required"))
		return
	}

	post, err := c.svc.GetPostBySlug(ctx.Request.Context(), slug)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Str("slug", slug).Msg("posts: GetPostBySlug failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	if post == nil {
		response.NotFoundGin(ctx, "post not found")
		return
	}

	response.SuccessGin(ctx, toPostResponse(post))
}

// CreatePost handles POST /api/posts.
// @Summary Create a new post
// @Description Create a new blog post or news item.
// @Tags Posts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param payload body dto.CreatePostRequest true "Post Data"
// @Success 201 {object} dto.PostResponse
// @Router /api/posts [post]
func (c *Controller) CreatePost(ctx *gin.Context) {
	var req dto.CreatePostRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, fmt.Errorf("invalid JSON body"))
		return
	}
	if req.Title == "" {
		response.BadRequestGin(ctx, fmt.Errorf("title is required"))
		return
	}
	if req.CategoryID == "" {
		response.BadRequestGin(ctx, fmt.Errorf("category_id is required"))
		return
	}

	userID := middleware.GetUserID(ctx.Request.Context())

	post, err := c.svc.CreatePost(ctx.Request.Context(), req, userID)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("posts: CreatePost failed")
		response.InternalErrorGin(ctx, err)
		return
	}

	response.CreatedGin(ctx, toPostResponse(post))
}

// UpdatePost handles PUT /api/posts/:id.
// @Summary Update a post
// @Description Updates an existing post by ID.
// @Tags Posts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Post ID"
// @Param payload body dto.UpdatePostRequest true "Update data"
// @Success 200 {object} dto.PostResponse
// @Router /api/posts/{id} [put]
func (c *Controller) UpdatePost(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		response.BadRequestGin(ctx, nil)
		return
	}

	var req dto.UpdatePostRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, fmt.Errorf("invalid JSON body"))
		return
	}

	userID := middleware.GetUserID(ctx.Request.Context())

	post, err := c.svc.UpdatePost(ctx.Request.Context(), id, req, userID)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Str("id", id).Msg("posts: UpdatePost failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	if post == nil {
		response.NotFoundGin(ctx, "post not found")
		return
	}

	response.SuccessGin(ctx, toPostResponse(post))
}

// SoftDeletePost handles DELETE /api/posts/:id.
// @Summary Soft-delete a post
// @Description Moves a post to trash (soft delete).
// @Tags Posts
// @Produce json
// @Security BearerAuth
// @Param id path string true "Post ID"
// @Success 200 {object} map[string]any
// @Router /api/posts/{id} [delete]
func (c *Controller) SoftDeletePost(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		response.BadRequestGin(ctx, nil)
		return
	}

	if err := c.svc.SoftDeletePost(ctx.Request.Context(), id); err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Str("id", id).Msg("posts: SoftDeletePost failed")
		response.InternalErrorGin(ctx, err)
		return
	}

	response.SuccessGin(ctx, map[string]any{"message": "post moved to trash"})
}

// RestorePost handles PUT /api/posts/:id/restore.
// @Summary Restore a trashed post
// @Description Restores a soft-deleted post from trash.
// @Tags Posts
// @Produce json
// @Security BearerAuth
// @Param id path string true "Post ID"
// @Success 200 {object} map[string]any
// @Router /api/posts/{id}/restore [put]
func (c *Controller) RestorePost(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		response.BadRequestGin(ctx, nil)
		return
	}

	if err := c.svc.RestorePost(ctx.Request.Context(), id); err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Str("id", id).Msg("posts: RestorePost failed")
		response.InternalErrorGin(ctx, err)
		return
	}

	response.SuccessGin(ctx, map[string]any{"message": "post restored"})
}

// HardDeletePost handles DELETE /api/posts/:id/permanent.
// @Summary Permanently delete a post
// @Description Permanently removes a post from the database.
// @Tags Posts
// @Produce json
// @Security BearerAuth
// @Param id path string true "Post ID"
// @Success 200 {object} map[string]any
// @Router /api/posts/{id}/permanent [delete]
func (c *Controller) HardDeletePost(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		response.BadRequestGin(ctx, nil)
		return
	}

	if err := c.svc.HardDeletePost(ctx.Request.Context(), id); err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Str("id", id).Msg("posts: HardDeletePost failed")
		response.InternalErrorGin(ctx, err)
		return
	}

	response.SuccessGin(ctx, map[string]any{"message": "post permanently deleted"})
}

// PromotePost handles POST /api/posts/:id/promote.
// @Summary Promote a post
// @Description Promotes a crawled/draft post to a specific category.
// @Tags Posts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Post ID"
// @Param payload body dto.PromoteRequest true "Promotion data"
// @Success 200 {object} dto.PostResponse
// @Router /api/posts/{id}/promote [post]
func (c *Controller) PromotePost(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		response.BadRequestGin(ctx, nil)
		return
	}

	var req dto.PromoteRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, fmt.Errorf("invalid JSON body"))
		return
	}
	if req.CategoryID == "" {
		response.BadRequestGin(ctx, fmt.Errorf("category_id is required"))
		return
	}

	userID := middleware.GetUserID(ctx.Request.Context())

	post, err := c.svc.PromotePost(ctx.Request.Context(), id, req.CategoryID, userID)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Str("id", id).Msg("posts: PromotePost failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	if post == nil {
		response.NotFoundGin(ctx, "post or category not found")
		return
	}

	response.SuccessGin(ctx, toPostResponse(post))
}

// ListTrash handles GET /api/posts/trash.
// @Summary List trashed posts
// @Description Returns paginated list of soft-deleted posts.
// @Tags Posts
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number"
// @Param limit query int false "Items per page"
// @Success 200 {object} map[string]any
// @Router /api/posts/trash [get]
func (c *Controller) ListTrash(ctx *gin.Context) {
	q := dto.PostQueryParams{
		Page:   page(ctx, "page", 1),
		Limit:  page(ctx, "limit", 10),
		Search: ctx.Query("search"),
	}

	posts, total, err := c.svc.ListDeletedPosts(ctx.Request.Context(), q)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("posts: ListTrash failed")
		response.InternalErrorGin(ctx, err)
		return
	}

	items := make([]dto.PostResponse, len(posts))
	for i, p := range posts {
		items[i] = toPostResponse(p)
	}

	response.PaginatedGin(ctx, items, total, q.Page, q.Limit)
}

// ListHiddenPosts handles GET /api/posts/hidden.
// @Summary List hidden posts
// @Description Returns paginated list of hidden/unpublished posts.
// @Tags Posts
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number"
// @Param limit query int false "Items per page"
// @Success 200 {object} map[string]any
// @Router /api/posts/hidden [get]
func (c *Controller) ListHiddenPosts(ctx *gin.Context) {
	q := dto.PostQueryParams{
		Page:   page(ctx, "page", 1),
		Limit:  page(ctx, "limit", 10),
		Search: ctx.Query("search"),
	}

	posts, total, err := c.svc.ListHiddenPosts(ctx.Request.Context(), q)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("posts: ListHiddenPosts failed")
		response.InternalErrorGin(ctx, err)
		return
	}

	items := make([]dto.PostResponse, len(posts))
	for i, p := range posts {
		items[i] = toPostResponse(p)
	}

	response.PaginatedGin(ctx, items, total, q.Page, q.Limit)
}

// SavePreview handles POST /api/posts/preview.
// @Summary Save a post preview
// @Description Saves a temporary preview of post content.
// @Tags Posts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 201 {object} map[string]any
// @Router /api/posts/preview [post]
func (c *Controller) SavePreview(ctx *gin.Context) {
	var req dto.SavePreviewRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, fmt.Errorf("invalid JSON body"))
		return
	}

	previewID, err := c.svc.SavePreview(ctx.Request.Context(), req)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("posts: SavePreview failed")
		response.InternalErrorGin(ctx, err)
		return
	}

	response.CreatedGin(ctx, map[string]any{"id": previewID})
}

// GetPreview handles GET /api/posts/preview/:id.
// @Summary Get a post preview
// @Description Retrieves a temporary preview by ID.
// @Tags Posts
// @Produce json
// @Param id path string true "Preview ID"
// @Success 200 {object} map[string]any
// @Router /api/posts/preview/{id} [get]
func (c *Controller) GetPreview(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		response.BadRequestGin(ctx, nil)
		return
	}

	data, err := c.svc.GetPreview(ctx.Request.Context(), id)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Str("id", id).Msg("posts: GetPreview failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	if data == nil {
		response.NotFoundGin(ctx, "preview not found or expired")
		return
	}

	response.SuccessGin(ctx, data)
}

// UploadImage handles POST /api/posts/images/upload.
// @Summary Upload an image for a post
// @Description Uploads an image to Cloudflare R2 storage.
// @Tags Posts
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param file formData file true "Image file"
// @Success 200 {object} map[string]any
// @Router /api/posts/images/upload [post]
func (c *Controller) UploadImage(ctx *gin.Context) {
	file, header, err := ctx.Request.FormFile("file")
	if err != nil {
		response.BadRequestGin(ctx, fmt.Errorf("file is required"))
		return
	}
	defer file.Close()

	body, err := io.ReadAll(file)
	if err != nil {
		response.InternalErrorGin(ctx, err)
		return
	}

	contentType := header.Header.Get("Content-Type")
	url, err := c.svc.UploadImage(ctx.Request.Context(), header.Filename, contentType, body)
	if err != nil {
		response.InternalErrorGin(ctx, err)
		return
	}

	response.OKGin(ctx, map[string]any{"url": url})
}

// DeleteImage handles DELETE /api/posts/images.
// @Summary Delete an image
// @Description Deletes an image from Cloudflare R2 storage.
// @Tags Posts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param url body string true "Image URL to delete"
// @Success 200 {object} map[string]any
// @Router /api/posts/images [delete]
func (c *Controller) DeleteImage(ctx *gin.Context) {
	var req struct {
		URL string `json:"url" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, fmt.Errorf("url is required"))
		return
	}

	if err := c.svc.DeleteImage(ctx.Request.Context(), req.URL); err != nil {
		response.InternalErrorGin(ctx, err)
		return
	}

	response.OKGin(ctx, map[string]any{"message": "Deleted successfully"})
}

// DeleteImageByFilename handles DELETE /api/posts/images/id/:filename.
func (c *Controller) DeleteImageByFilename(ctx *gin.Context) {
	filename := ctx.Param("filename")
	if filename == "" {
		response.BadRequestGin(ctx, fmt.Errorf("filename is required"))
		return
	}

	if err := c.svc.DeleteImageByFilename(ctx.Request.Context(), filename); err != nil {
		response.InternalErrorGin(ctx, err)
		return
	}

	response.OKGin(ctx, map[string]any{"message": "Deleted successfully"})
}

// ─── Category Handlers ──────────────────────────────────────────────────────

// ListCategories handles GET /api/posts/categories.
// @Summary List post categories
// @Description Fetch all categories available for posts.
// @Tags Posts Categories
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/posts/categories [get]
func (c *Controller) ListCategories(ctx *gin.Context) {
	cats, err := c.svc.ListCategories(ctx.Request.Context())
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("posts: ListCategories failed")
		response.InternalErrorGin(ctx, err)
		return
	}

	items := make([]dto.CategoryResponse, len(cats))
	for i, cat := range cats {
		items[i] = toCategoryResponse(cat)
	}

	response.SuccessGin(ctx, items)
}

// GetCategory handles GET /api/posts/categories/:id.
// @Summary Get a category
// @Description Returns a single post category by ID.
// @Tags Posts Categories
// @Produce json
// @Param id path string true "Category ID"
// @Success 200 {object} map[string]any
// @Router /api/posts/categories/{id} [get]
func (c *Controller) GetCategory(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		response.BadRequestGin(ctx, nil)
		return
	}

	cat, err := c.svc.GetCategoryByID(ctx.Request.Context(), id)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Str("id", id).Msg("posts: GetCategory failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	if cat == nil {
		response.NotFoundGin(ctx, "category not found")
		return
	}

	response.SuccessGin(ctx, toCategoryResponse(cat))
}

// CreateCategory handles POST /api/posts/categories.
// @Summary Create a category
// @Description Creates a new post category.
// @Tags Posts Categories
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 201 {object} map[string]any
// @Router /api/posts/categories [post]
func (c *Controller) CreateCategory(ctx *gin.Context) {
	var req dto.CreateCategoryRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, fmt.Errorf("invalid JSON body"))
		return
	}
	if req.Name == "" {
		response.BadRequestGin(ctx, fmt.Errorf("name is required"))
		return
	}

	cat, err := c.svc.CreateCategory(ctx.Request.Context(), req)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Msg("posts: CreateCategory failed")
		response.InternalErrorGin(ctx, err)
		return
	}

	response.CreatedGin(ctx, toCategoryResponse(cat))
}

// UpdateCategory handles PUT /api/posts/categories/:id.
// @Summary Update a category
// @Description Updates an existing post category.
// @Tags Posts Categories
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Category ID"
// @Success 200 {object} map[string]any
// @Router /api/posts/categories/{id} [put]
func (c *Controller) UpdateCategory(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		response.BadRequestGin(ctx, nil)
		return
	}

	var req dto.UpdateCategoryRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		response.BadRequestGin(ctx, fmt.Errorf("invalid JSON body"))
		return
	}

	cat, err := c.svc.UpdateCategory(ctx.Request.Context(), id, req)
	if err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Str("id", id).Msg("posts: UpdateCategory failed")
		response.InternalErrorGin(ctx, err)
		return
	}
	if cat == nil {
		response.NotFoundGin(ctx, "category not found")
		return
	}

	response.SuccessGin(ctx, toCategoryResponse(cat))
}

// DeleteCategory handles DELETE /api/posts/categories/:id.
// @Summary Delete a category
// @Description Deletes a post category.
// @Tags Posts Categories
// @Produce json
// @Security BearerAuth
// @Param id path string true "Category ID"
// @Success 200 {object} map[string]any
// @Router /api/posts/categories/{id} [delete]
func (c *Controller) DeleteCategory(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		response.BadRequestGin(ctx, nil)
		return
	}

	if err := c.svc.DeleteCategory(ctx.Request.Context(), id); err != nil {
		c.log.ErrorContext(ctx.Request.Context()).Err(err).Str("id", id).Msg("posts: DeleteCategory failed")
		response.InternalErrorGin(ctx, err)
		return
	}

	response.SuccessGin(ctx, map[string]any{"message": "category deleted"})
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func page(ctx *gin.Context, key string, fallback int) int {
	v := ctx.Query(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return fallback
	}
	return n
}

func toPostResponse(p *entities.Post) dto.PostResponse {
	resp := dto.PostResponse{
		ID:              p.ID.Hex(),
		Title:           p.Title,
		Slug:            p.Slug,
		Content:         p.Content,
		Excerpt:         p.Excerpt,
		ThumbnailURL:    p.Thumbnail,
		Status:          string(p.Status),
		IsPublished:     p.IsPublished,
		ViewCount:       p.ViewCount,
		CommentCount:    p.CommentCount,
		Tags:            p.Tags,
		IsCreatedByAI:   p.IsCreatedByAI,
		MetaTitle:       p.MetaTitle,
		MetaDescription: p.MetaDescription,
		FocusKeyword:    p.FocusKeyword,
		SchemaType:      string(p.SchemaType),
		SEOScore:        p.SEOScore,
		OGTitle:         p.OGTitle,
		OGDescription:   p.OGDescription,
		OGImage:         p.OGImage,
		TwitterCard:     p.TwitterCard,
		SchemaMarkup:    p.SchemaMarkup,
		Meta:            p.Meta,
		CreatedAt:       p.CreatedAt,
		UpdatedAt:       p.UpdatedAt,
		PublishedAt:     p.PublishedAt,
	}
	if p.Author != nil {
		resp.Author = &dto.AuthorRef{ID: p.Author.ID.Hex(), FullName: p.Author.FullName, AvatarURL: p.Author.AvatarURL}
	}
	if p.Category != nil {
		resp.Category = &dto.CategoryRef{ID: p.Category.ID.Hex(), Name: p.Category.Name, Slug: p.Category.Slug}
	}
	return resp
}

func toCategoryResponse(cat *entities.Category) dto.CategoryResponse {
	return dto.CategoryResponse{
		ID:          cat.ID.Hex(),
		Name:        cat.Name,
		Slug:        cat.Slug,
		Description: cat.Description,
		Icon:        cat.Icon,
		IsHidden:    cat.IsHidden,
		CreatedAt:   cat.CreatedAt,
		UpdatedAt:   cat.UpdatedAt,
	}
}
