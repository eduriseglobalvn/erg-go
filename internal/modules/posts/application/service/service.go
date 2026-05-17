// Package service provides business logic for the posts module.
package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"erg.ninja/internal/modules/posts/api/dto"
	entities "erg.ninja/internal/modules/posts/domain/entity"
	"erg.ninja/internal/modules/posts/infrastructure/repository"
	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/storage"
)

const (
	postsCacheKey      = "posts:list"
	categoriesCacheKey = "posts:categories"
	previewCacheTTL    = 30 * time.Minute
	listCacheTTL       = 5 * time.Minute
)

// Service provides post business logic.
type Service struct {
	repo  *repository.Repository
	redis *cache.RedisClient
	log   *logger.Logger
	r2    *storage.R2Client
}

// NewService creates a new posts service.
func NewService(repo *repository.Repository, redis *cache.RedisClient, log *logger.Logger, r2 *storage.R2Client) *Service {
	return &Service{
		repo:  repo,
		redis: redis,
		log:   log,
		r2:    r2,
	}
}

// ─── Post CRUD ────────────────────────────────────────────────────────────────

// CreatePost creates a new post.
func (s *Service) CreatePost(ctx context.Context, req dto.CreatePostRequest, authorID string) (*entities.Post, error) {
	// Resolve category.
	cat, err := s.repo.GetCategoryByID(ctx, req.CategoryID)
	if err != nil {
		return nil, fmt.Errorf("posts.CreatePost: %w", err)
	}
	if cat == nil {
		return nil, fmt.Errorf("posts.CreatePost: category not found")
	}

	// Generate slug if not provided.
	slug := req.Slug
	if slug == "" {
		slug = s.generateSlug(req.Title)
	}

	// Determine status.
	status := entities.PostStatus(strings.ToUpper(req.Status))
	if status == "" {
		status = entities.PostStatusDraft
	}
	isPublished := status == entities.PostStatusPublished

	// Prepare ObjectIDs
	objCatID, _ := bson.ObjectIDFromHex(req.CategoryID)
	objAuthorID, _ := bson.ObjectIDFromHex(authorID)

	// Build post.
	post := &entities.Post{
		ID:               bson.NewObjectID(),
		Title:            req.Title,
		Slug:             slug,
		Excerpt:          req.Excerpt,
		Content:          req.Content,
		Thumbnail:        req.Thumbnail,
		Status:           status,
		IsPublished:      isPublished,
		AuthorID:         objAuthorID,
		CreatedByID:      authorID,
		CategoryID:       objCatID,
		Tags:             req.Tags,
		IsCreatedByAI:    req.IsCreatedByAI,
		AIPrompt:         req.AIPrompt,
		AIJobID:          req.AIJobID,
		MetaTitle:        req.MetaTitle,
		MetaDescription:  req.MetaDescription,
		FocusKeyword:     req.FocusKeyword,
		Keywords:         req.Keywords,
		CanonicalURL:     req.CanonicalURL,
		SchemaType:       entities.SchemaType(req.SchemaType),
		Meta:             req.Meta,
		SEOScore:         req.SEOScore,
		ReadabilityScore: req.ReadabilityScore,
		KeywordDensity:   req.KeywordDensity,
		RobotsIndex:      true,
		RobotsFollow:     true,
		ViewCount:        0,
		CommentCount:     0,
	}

	if isPublished {
		now := time.Now().UTC()
		post.PublishedAt = &now
		post.PublishedBy = authorID
	}

	// SEO auto-fill.
	if post.MetaTitle == "" && req.Title != "" {
		post.MetaTitle = req.Title
	}
	if post.MetaDescription == "" && req.Excerpt != "" {
		post.MetaDescription = req.Excerpt
	}

	if err := s.repo.CreatePost(ctx, post); err != nil {
		return nil, fmt.Errorf("posts.CreatePost: %w", err)
	}

	// Populate relations for response.
	post.Author = &entities.AuthorRef{ID: objAuthorID}
	post.Category = &entities.CategoryRef{ID: objCatID, Name: cat.Name, Slug: cat.Slug}

	s.clearListCache(ctx)
	s.log.InfoContext(ctx).Str("post_id", post.ID.Hex()).Str("slug", post.Slug).Msg("posts: post created")
	return post, nil
}

// GetPostByID retrieves a post by ID.
func (s *Service) GetPostByID(ctx context.Context, id string) (*entities.Post, error) {
	post, err := s.repo.GetPostByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("posts.GetPostByID: %w", err)
	}
	if post == nil || post.DeletedAt != nil {
		return nil, nil
	}

	// Increment view count for published posts.
	if post.IsPublished {
		_ = s.repo.IncrementViewCount(ctx, id)
	}

	// Populate relations.
	s.populateRelations(ctx, post)
	return post, nil
}

// GetPostBySlug retrieves a published post by slug.
func (s *Service) GetPostBySlug(ctx context.Context, slug string) (*entities.Post, error) {
	post, err := s.repo.GetPostBySlug(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("posts.GetPostBySlug: %w", err)
	}
	if post == nil {
		return nil, nil
	}

	if !post.IsPublished {
		return nil, nil // not available publicly
	}

	// Increment view count.
	_ = s.repo.IncrementViewCount(ctx, post.ID.Hex())

	s.populateRelations(ctx, post)
	return post, nil
}

// UpdatePost updates an existing post.
func (s *Service) UpdatePost(ctx context.Context, id string, req dto.UpdatePostRequest, userID string) (*entities.Post, error) {
	post, err := s.repo.GetPostByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("posts.UpdatePost.GetPostByID: %w", err)
	}
	if post == nil {
		return nil, nil
	}

	updates := bson.M{}

	if req.Title != "" {
		updates["title"] = req.Title
	}
	if req.Slug != "" {
		slug := strings.ToLower(req.Slug)
		updates["slug"] = slug
	}
	if req.Excerpt != "" {
		updates["excerpt"] = req.Excerpt
	}
	if req.Content != "" {
		updates["content"] = req.Content
	}
	if req.Thumbnail != "" {
		updates["thumbnail_url"] = req.Thumbnail
	}
	if req.Status != "" {
		status := entities.PostStatus(strings.ToUpper(req.Status))
		updates["status"] = status
		if status == entities.PostStatusPublished && post.Status != entities.PostStatusPublished {
			now := time.Now().UTC()
			updates["published_at"] = now
			updates["published_by_id"] = userID
			updates["is_published"] = true
		} else if status != entities.PostStatusPublished {
			updates["is_published"] = false
		}
	}
	if req.CategoryID != "" {
		updates["category_id"] = req.CategoryID
	}
	if req.Tags != nil {
		updates["tags"] = req.Tags
	}
	if req.MetaTitle != "" {
		updates["meta_title"] = req.MetaTitle
	}
	if req.MetaDescription != "" {
		updates["meta_description"] = req.MetaDescription
	}
	if req.FocusKeyword != "" {
		updates["focus_keyword"] = req.FocusKeyword
	}
	if req.Keywords != "" {
		updates["keywords"] = req.Keywords
	}
	if req.CanonicalURL != "" {
		updates["canonical_url"] = req.CanonicalURL
	}
	if req.SchemaType != "" {
		updates["schema_type"] = entities.SchemaType(req.SchemaType)
	}
	if req.SEOScore != nil {
		updates["seo_score"] = *req.SEOScore
	}
	if req.ReadabilityScore != nil {
		updates["readability_score"] = *req.ReadabilityScore
	}
	if req.KeywordDensity != nil {
		updates["keyword_density"] = *req.KeywordDensity
	}
	if req.Meta != nil {
		updates["meta"] = req.Meta
	}
	if req.IsCreatedByAI != nil {
		updates["is_created_by_ai"] = *req.IsCreatedByAI
	}

	if err := s.repo.UpdatePost(ctx, id, updates); err != nil {
		return nil, fmt.Errorf("posts.UpdatePost: %w", err)
	}

	// Clear caches.
	s.clearListCache(ctx)
	s.clearPostCache(ctx, post.Slug)

	// Refetch updated post.
	return s.repo.GetPostByID(ctx, id)
}

// SoftDeletePost marks a post as deleted.
func (s *Service) SoftDeletePost(ctx context.Context, id string) error {
	post, err := s.repo.GetPostByID(ctx, id)
	if err != nil {
		return fmt.Errorf("posts.SoftDeletePost: %w", err)
	}
	if post == nil {
		return fmt.Errorf("posts.SoftDeletePost: post not found")
	}

	if err := s.repo.SoftDeletePost(ctx, id); err != nil {
		return fmt.Errorf("posts.SoftDeletePost: %w", err)
	}

	s.clearListCache(ctx)
	s.clearPostCache(ctx, post.Slug)
	s.log.InfoContext(ctx).Str("post_id", id).Msg("posts: post soft deleted")
	return nil
}

// RestorePost restores a post from trash.
func (s *Service) RestorePost(ctx context.Context, id string) error {
	if err := s.repo.RestorePost(ctx, id); err != nil {
		return fmt.Errorf("posts.RestorePost: %w", err)
	}
	s.clearListCache(ctx)
	s.log.InfoContext(ctx).Str("post_id", id).Msg("posts: post restored")
	return nil
}

// HardDeletePost permanently deletes a post.
func (s *Service) HardDeletePost(ctx context.Context, id string) error {
	post, _ := s.repo.GetPostByID(ctx, id)
	if err := s.repo.HardDeletePost(ctx, id); err != nil {
		return fmt.Errorf("posts.HardDeletePost: %w", err)
	}
	s.clearListCache(ctx)
	if post != nil {
		s.clearPostCache(ctx, post.Slug)
	}
	s.log.InfoContext(ctx).Str("post_id", id).Msg("posts: post permanently deleted")
	return nil
}

// PromotePost moves a post from hidden category to a public category.
func (s *Service) PromotePost(ctx context.Context, postID, categoryID, userID string) (*entities.Post, error) {
	cat, err := s.repo.GetCategoryByID(ctx, categoryID)
	if err != nil {
		return nil, fmt.Errorf("posts.PromotePost.GetCategory: %w", err)
	}
	if cat == nil {
		return nil, fmt.Errorf("posts.PromotePost: category not found")
	}
	if cat.IsHidden {
		return nil, fmt.Errorf("posts.PromotePost: target category is hidden")
	}

	updates := bson.M{
		"category_id": categoryID,
	}
	if err := s.repo.UpdatePost(ctx, postID, updates); err != nil {
		return nil, fmt.Errorf("posts.PromotePost: %w", err)
	}

	s.clearListCache(ctx)
	return s.repo.GetPostByID(ctx, postID)
}

// ─── Post List ────────────────────────────────────────────────────────────────

// ListPosts returns a paginated list of posts.
func (s *Service) ListPosts(ctx context.Context, q dto.PostQueryParams) ([]*entities.Post, int64, error) {
	page := q.Page
	if page < 1 {
		page = 1
	}
	limit := q.Limit
	if limit < 1 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	order := q.Order
	if order == "" {
		order = "DESC"
	}
	sortBy := q.SortBy
	if sortBy == "" {
		sortBy = "created_at"
	}

	// Get visible category IDs for filtering.
	visibleCatIDs, err := s.repo.GetVisibleCategoryIDs(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("posts.ListPosts.GetVisibleCategoryIDs: %w", err)
	}

	query := repository.ListPostsQuery{
		Page:       page,
		Limit:      limit,
		Search:     q.Search,
		CategoryID: q.CategoryID,
		Category:   q.Category,
		Status:     q.Status,
		SortBy:     sortBy,
		Order:      order,
		Deleted:    false,
		HiddenCats: false,
	}

	posts, total, err := s.repo.ListPosts(ctx, query)
	if err != nil {
		return nil, 0, fmt.Errorf("posts.ListPosts: %w", err)
	}

	// Filter out posts from hidden categories.
	if len(visibleCatIDs) > 0 {
		catSet := make(map[string]bool)
		for _, id := range visibleCatIDs {
			catSet[id] = true
		}
		filtered := make([]*entities.Post, 0, len(posts))
		for _, p := range posts {
			if catSet[p.CategoryID.Hex()] {
				filtered = append(filtered, p)
			}
		}
		posts = filtered
	}

	// Populate relations.
	for _, p := range posts {
		s.populateRelations(ctx, p)
	}

	return posts, total, nil
}

// ListDeletedPosts returns a paginated list of soft-deleted posts.
func (s *Service) ListDeletedPosts(ctx context.Context, q dto.PostQueryParams) ([]*entities.Post, int64, error) {
	page := q.Page
	if page < 1 {
		page = 1
	}
	limit := q.Limit
	if limit < 1 {
		limit = 10
	}

	query := repository.ListPostsQuery{
		Page:    page,
		Limit:   limit,
		Search:  q.Search,
		Deleted: true,
	}
	return s.repo.ListPosts(ctx, query)
}

// ListHiddenPosts returns posts from hidden categories.
func (s *Service) ListHiddenPosts(ctx context.Context, q dto.PostQueryParams) ([]*entities.Post, int64, error) {
	page := q.Page
	if page < 1 {
		page = 1
	}
	limit := q.Limit
	if limit < 1 {
		limit = 10
	}

	query := repository.ListPostsQuery{
		Page:       page,
		Limit:      limit,
		Search:     q.Search,
		Deleted:    false,
		HiddenCats: true,
	}
	return s.repo.ListPosts(ctx, query)
}

// ─── Category CRUD ───────────────────────────────────────────────────────────

// CreateCategory creates a new category.
func (s *Service) CreateCategory(ctx context.Context, req dto.CreateCategoryRequest) (*entities.Category, error) {
	slug := req.Slug
	if slug == "" {
		slug = s.generateSlug(req.Name)
	}

	cat := &entities.Category{
		ID:          bson.NewObjectID(),
		Name:        req.Name,
		Slug:        slug,
		Description: req.Description,
		Icon:        req.Icon,
		IsHidden:    false,
	}

	if err := s.repo.CreateCategory(ctx, cat); err != nil {
		return nil, fmt.Errorf("posts.CreateCategory: %w", err)
	}

	s.clearCategoriesCache(ctx)
	s.log.InfoContext(ctx).Str("category_id", cat.ID.Hex()).Str("name", cat.Name).Msg("posts: category created")
	return cat, nil
}

// ListCategories returns all visible categories.
func (s *Service) ListCategories(ctx context.Context) ([]*entities.Category, error) {
	cats, err := s.repo.ListCategories(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("posts.ListCategories: %w", err)
	}
	sortCategoriesForAdmin(cats)
	return cats, nil
}

// SeedDefaultCategories seeds both normal and hidden categories on startup.
func (s *Service) SeedDefaultCategories(ctx context.Context) error {
	if err := s.migrateLegacyPostCategories(ctx); err != nil {
		return err
	}

	defaultCategories := []struct {
		Name        string
		Slug        string
		Description string
		Icon        string
		IsHidden    bool
	}{
		// Standard Categories
		{
			Name:        "Thông báo",
			Slug:        "thong-bao",
			Description: "Thông báo chính thức từ EDURISE GLOBAL",
			Icon:        "Inbox",
			IsHidden:    false,
		},
		{
			Name:        "Tin giáo dục",
			Slug:        "tin-giao-duc",
			Description: "Tin tức mới nhất về giáo dục và đào tạo",
			Icon:        "GraduationCap",
			IsHidden:    false,
		},
		{
			Name:        "Mẹo và thủ thuật",
			Slug:        "meo-va-thu-thuat",
			Description: "Các mẹo hay và thủ thuật hữu ích",
			Icon:        "Lightbulb",
			IsHidden:    false,
		},
		{
			Name:        "Hoạt động của ERG",
			Slug:        "hoat-dong-cua-erg",
			Description: "Tin tức và hoạt động cộng đồng của EDURISE GLOBAL",
			Icon:        "Building2",
			IsHidden:    false,
		},
		// Hidden Categories
		{
			Name:        "Crawler Tips (Hidden)",
			Slug:        "__hidden_tips",
			Description: "Used internally for AI generation guidelines",
			Icon:        "",
			IsHidden:    true,
		},
		{
			Name:        "Reference Materials (Hidden)",
			Slug:        "__hidden_reference",
			Description: "Used internally for AI referencing",
			Icon:        "",
			IsHidden:    true,
		},
		{
			Name:        "Scrape Pool (Hidden)",
			Slug:        "__hidden_scrape_pool",
			Description: "Temporary pool for unclassified crawled articles",
			Icon:        "",
			IsHidden:    true,
		},
	}

	for _, data := range defaultCategories {
		// Check if it exists
		cat, err := s.repo.GetCategoryBySlug(ctx, data.Slug)
		if err != nil {
			return fmt.Errorf("posts.SeedDefaultCategories.Get: %w", err)
		}
		if cat != nil {
			updates := bson.M{}
			if cat.Name != data.Name {
				updates["name"] = data.Name
			}
			if cat.Description != data.Description {
				updates["description"] = data.Description
			}
			if cat.Icon != data.Icon {
				updates["icon"] = data.Icon
			}
			if cat.IsHidden != data.IsHidden {
				updates["is_hidden"] = data.IsHidden
			}
			if len(updates) > 0 {
				if err := s.repo.UpdateCategory(ctx, cat.ID.Hex(), updates); err != nil {
					s.log.WarnContext(ctx).Err(err).Str("slug", data.Slug).Msg("posts: failed to update category metadata")
				}
			}
			continue
		}

		newCat := &entities.Category{
			ID:          bson.NewObjectID(),
			Name:        data.Name,
			Slug:        data.Slug,
			Description: data.Description,
			Icon:        data.Icon,
			IsHidden:    data.IsHidden,
		}
		if err := s.repo.CreateCategory(ctx, newCat); err != nil {
			return fmt.Errorf("posts.SeedDefaultCategories.Create: %w", err)
		}
		s.log.InfoContext(ctx).Str("slug", data.Slug).Msg("posts: seeded category")
	}

	return nil
}

func (s *Service) migrateLegacyPostCategories(ctx context.Context) error {
	legacy, err := s.repo.GetCategoryBySlug(ctx, "hoat-dong-cong-ty")
	if err != nil {
		return fmt.Errorf("posts.SeedDefaultCategories.LegacyCompanyActivity: %w", err)
	}
	if legacy == nil {
		return nil
	}
	current, err := s.repo.GetCategoryBySlug(ctx, "thong-bao")
	if err != nil {
		return fmt.Errorf("posts.SeedDefaultCategories.CurrentAnnouncement: %w", err)
	}
	if current != nil && current.ID != legacy.ID {
		moved, err := s.repo.MovePostsToCategory(ctx, legacy.ID.Hex(), current.ID.Hex())
		if err != nil {
			return fmt.Errorf("posts.SeedDefaultCategories.MoveLegacyCompanyActivity: %w", err)
		}
		if err := s.repo.UpdateCategory(ctx, legacy.ID.Hex(), bson.M{
			"name":        "Hoạt động công ty (Legacy)",
			"description": "Deprecated category migrated into Thông báo",
			"is_hidden":   true,
		}); err != nil {
			return fmt.Errorf("posts.SeedDefaultCategories.HideLegacyCompanyActivity: %w", err)
		}
		s.log.InfoContext(ctx).
			Int64("moved_posts", moved).
			Str("legacy_slug", "hoat-dong-cong-ty").
			Msg("posts: moved legacy company category posts to thong-bao and hid legacy category")
		return nil
	}
	if err := s.repo.UpdateCategory(ctx, legacy.ID.Hex(), bson.M{
		"name":        "Thông báo",
		"slug":        "thong-bao",
		"description": "Thông báo chính thức từ EDURISE GLOBAL",
		"icon":        "Inbox",
		"is_hidden":   false,
	}); err != nil {
		return fmt.Errorf("posts.SeedDefaultCategories.MigrateCompanyActivity: %w", err)
	}
	s.log.InfoContext(ctx).Msg("posts: migrated hoat-dong-cong-ty category to thong-bao")
	return nil
}

func sortCategoriesForAdmin(cats []*entities.Category) {
	priority := map[string]int{
		"thong-bao":         1,
		"tin-giao-duc":      2,
		"meo-va-thu-thuat":  3,
		"hoat-dong-cua-erg": 4,
	}
	sort.SliceStable(cats, func(i, j int) bool {
		pi, iKnown := priority[cats[i].Slug]
		pj, jKnown := priority[cats[j].Slug]
		if iKnown || jKnown {
			if pi == 0 {
				pi = 1000
			}
			if pj == 0 {
				pj = 1000
			}
			if pi != pj {
				return pi < pj
			}
		}
		return strings.ToLower(cats[i].Name) < strings.ToLower(cats[j].Name)
	})
}

// GetCategoryByID returns a single category.
func (s *Service) GetCategoryByID(ctx context.Context, id string) (*entities.Category, error) {
	return s.repo.GetCategoryByID(ctx, id)
}

// UpdateCategory updates a category.
func (s *Service) UpdateCategory(ctx context.Context, id string, req dto.UpdateCategoryRequest) (*entities.Category, error) {
	cat, err := s.repo.GetCategoryByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("posts.UpdateCategory.GetCategory: %w", err)
	}
	if cat == nil {
		return nil, nil
	}

	updates := bson.M{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Slug != "" {
		updates["slug"] = strings.ToLower(req.Slug)
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Icon != "" {
		updates["icon"] = req.Icon
	}
	if req.IsHidden != nil {
		updates["is_hidden"] = *req.IsHidden
	}

	if err := s.repo.UpdateCategory(ctx, id, updates); err != nil {
		return nil, fmt.Errorf("posts.UpdateCategory: %w", err)
	}

	s.clearCategoriesCache(ctx)
	return s.repo.GetCategoryByID(ctx, id)
}

// DeleteCategory deletes a category (must have no posts).
func (s *Service) DeleteCategory(ctx context.Context, id string) error {
	count, err := s.repo.CountPostsByCategory(ctx, id)
	if err != nil {
		return fmt.Errorf("posts.DeleteCategory.CountPosts: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("posts.DeleteCategory: category has %d posts", count)
	}

	if err := s.repo.DeleteCategory(ctx, id); err != nil {
		return fmt.Errorf("posts.DeleteCategory: %w", err)
	}

	s.clearCategoriesCache(ctx)
	s.log.InfoContext(ctx).Str("category_id", id).Msg("posts: category deleted")
	return nil
}

// ─── Preview ─────────────────────────────────────────────────────────────────

// SavePreview saves a post preview to Redis.
func (s *Service) SavePreview(ctx context.Context, req dto.SavePreviewRequest) (string, error) {
	previewID := req.ID
	if previewID == "" {
		previewID = uuid.New().String()
	}
	redisKey := "POST_PREVIEW:" + previewID

	if s.redis != nil {
		// Marshal as JSON for Redis storage.
		// We use simple string storage here; in production use Redis JSON.
		data := fmt.Sprintf("%v", req)
		if err := s.redis.Set(ctx, redisKey, data, previewCacheTTL); err != nil {
			return "", fmt.Errorf("posts.SavePreview: %w", err)
		}
	}

	return previewID, nil
}

// GetPreview retrieves a preview from Redis.
func (s *Service) GetPreview(ctx context.Context, id string) (map[string]any, error) {
	redisKey := "POST_PREVIEW:" + id
	if s.redis == nil {
		return nil, nil
	}

	val, err := s.redis.Get(ctx, redisKey)
	if err != nil || val == "" {
		return nil, nil
	}

	// Return as map. In production, use proper JSON serialization.
	return map[string]any{"preview_id": id, "data": val}, nil
}

// ─── Image Upload ────────────────────────────────────────────────────────────

func (s *Service) UploadImage(ctx context.Context, filename string, contentType string, body []byte) (string, error) {
	if s.r2 == nil {
		return "", fmt.Errorf("posts.UploadImage: storage not configured")
	}

	url, err := s.r2.UploadRaw(ctx, body, "posts/images", filename, contentType)
	if err != nil {
		return "", fmt.Errorf("posts.UploadImage: %w", err)
	}
	return url, nil
}

// DeleteImage deletes an image from R2 storage.
func (s *Service) DeleteImage(ctx context.Context, imageURL string) error {
	if s.r2 == nil {
		return fmt.Errorf("posts.DeleteImage: storage not configured")
	}

	if err := s.r2.DeleteFile(ctx, imageURL); err != nil {
		return fmt.Errorf("posts.DeleteImage: %w", err)
	}
	return nil
}

// DeleteImageByFilename removes an editor image using the legacy filename-only API.
func (s *Service) DeleteImageByFilename(ctx context.Context, filename string) error {
	if s.r2 == nil {
		return fmt.Errorf("posts.DeleteImageByFilename: storage not configured")
	}
	filename = strings.TrimSpace(filename)
	if filename == "" || strings.Contains(filename, "..") {
		return fmt.Errorf("posts.DeleteImageByFilename: invalid filename")
	}
	if err := s.r2.DeleteFile(ctx, "raw/posts/images/"+filename); err != nil {
		return fmt.Errorf("posts.DeleteImageByFilename: %w", err)
	}
	return nil
}

// ─── Helpers ───────────────────────────────────────────────────────────────────

// populateRelations fills in lightweight author and category references.
func (s *Service) populateRelations(ctx context.Context, post *entities.Post) {
	if post.AuthorID != bson.NilObjectID && post.Author == nil {
		author, _ := s.repo.GetAuthorByID(ctx, post.AuthorID.Hex())
		if author != nil {
			post.Author = author
		} else {
			post.Author = &entities.AuthorRef{ID: post.AuthorID, FullName: ""}
		}
	}
	if post.CategoryID != bson.NilObjectID && post.Category == nil {
		cat, _ := s.repo.GetCategoryByID(ctx, post.CategoryID.Hex())
		if cat != nil {
			post.Category = &entities.CategoryRef{ID: cat.ID, Name: cat.Name, Slug: cat.Slug}
		}
	}
}

func (s *Service) generateSlug(text string) string {
	// Remove diacritics.
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	slug, _, _ := transform.String(t, text)
	slug = strings.ToLower(slug)
	var result []rune
	for _, r := range slug {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
			result = append(result, r)
		}
	}
	return string(result) + "-" + uuid.New().String()[:8]
}

// Cache helpers.

func (s *Service) clearListCache(ctx context.Context) {
	if s.redis == nil {
		return
	}
	s.redis.Del(ctx, postsCacheKey)
}

func (s *Service) clearCategoriesCache(ctx context.Context) {
	if s.redis == nil {
		return
	}
	s.redis.Del(ctx, categoriesCacheKey)
}

func (s *Service) clearPostCache(ctx context.Context, slug string) {
	if s.redis == nil {
		return
	}
	s.redis.Del(ctx, "post:"+slug)
}
