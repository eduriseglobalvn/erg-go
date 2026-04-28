// Package repository provides PostgreSQL data access for the posts module.
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"gorm.io/gorm"

	"erg.ninja/internal/modules/posts/entities"
	"erg.ninja/internal/persistence/postgrescore"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
)

// Repository provides data access for posts and categories.
type Repository struct {
	db  *gorm.DB
	log *logger.Logger
}

// NewRepository creates a new posts repository.
func NewRepository(gormClient *database.GORMPostgresClient, log *logger.Logger) *Repository {
	var db *gorm.DB
	if gormClient != nil {
		db = gormClient.DB()
	}
	return &Repository{db: db, log: log}
}

func (r *Repository) ensureDB() error {
	if r.db == nil {
		return fmt.Errorf("posts.repo: postgres client unavailable")
	}
	return nil
}

// ─── Post CRUD ─────────────────────────────────────────────────────────────────

// CreatePost inserts a new post.
func (r *Repository) CreatePost(ctx context.Context, post *entities.Post) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	now := time.Now().UTC()
	post.CreatedAt = now
	post.UpdatedAt = now
	if post.Status == "" {
		post.Status = entities.PostStatusDraft
	}
	post.ViewCount = 0
	post.CommentCount = 0

	record, err := postToRecord(post)
	if err != nil {
		return fmt.Errorf("posts.repo.CreatePost.record: %w", err)
	}
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		return fmt.Errorf("posts.repo.CreatePost: %w", err)
	}
	return nil
}

// GetPostByID retrieves a post by its ID.
func (r *Repository) GetPostByID(ctx context.Context, id string) (*entities.Post, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	var record postgrescore.Post
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("posts.repo.GetPostByID: %w", err)
	}
	return recordToPost(&record)
}

// GetPostBySlug retrieves a published post by slug (excludes soft-deleted).
func (r *Repository) GetPostBySlug(ctx context.Context, slug string) (*entities.Post, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	var record postgrescore.Post
	err := r.db.WithContext(ctx).
		Where("slug = ? AND deleted_at IS NULL", slug).
		First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("posts.repo.GetPostBySlug: %w", err)
	}
	return recordToPost(&record)
}

// GetPostBySlugIncludeDeleted retrieves a post by slug including deleted ones.
func (r *Repository) GetPostBySlugIncludeDeleted(ctx context.Context, slug string) (*entities.Post, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	var record postgrescore.Post
	err := r.db.WithContext(ctx).Unscoped().Where("slug = ?", slug).First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("posts.repo.GetPostBySlugIncludeDeleted: %w", err)
	}
	return recordToPost(&record)
}

// UpdatePost updates an existing post.
func (r *Repository) UpdatePost(ctx context.Context, id string, updates bson.M) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	updates["updated_at"] = time.Now().UTC()
	result := r.db.WithContext(ctx).
		Model(&postgrescore.Post{}).
		Where("id = ?", id).
		Updates(convertPostUpdates(updates))
	if result.Error != nil {
		return fmt.Errorf("posts.repo.UpdatePost: %w", result.Error)
	}
	return nil
}

// SoftDeletePost marks a post as deleted (soft delete).
func (r *Repository) SoftDeletePost(ctx context.Context, id string) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	now := time.Now().UTC()
	if err := r.db.WithContext(ctx).Model(&postgrescore.Post{}).Where("id = ?", id).Updates(map[string]any{
		"deleted_at": now,
		"updated_at": now,
	}).Error; err != nil {
		return fmt.Errorf("posts.repo.SoftDeletePost: %w", err)
	}
	return nil
}

// RestorePost clears the deleted_at field.
func (r *Repository) RestorePost(ctx context.Context, id string) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).Unscoped().Model(&postgrescore.Post{}).Where("id = ?", id).Updates(map[string]any{
		"deleted_at": nil,
		"updated_at": time.Now().UTC(),
	}).Error; err != nil {
		return fmt.Errorf("posts.repo.RestorePost: %w", err)
	}
	return nil
}

// HardDeletePost permanently removes a post.
func (r *Repository) HardDeletePost(ctx context.Context, id string) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).Unscoped().Where("id = ?", id).Delete(&postgrescore.Post{}).Error; err != nil {
		return fmt.Errorf("posts.repo.HardDeletePost: %w", err)
	}
	return nil
}

// IncrementViewCount increments the view counter for a post.
func (r *Repository) IncrementViewCount(ctx context.Context, id string) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).Model(&postgrescore.Post{}).Where("id = ?", id).Updates(map[string]any{
		"view_count": gorm.Expr("view_count + ?", 1),
		"updated_at": time.Now().UTC(),
	}).Error; err != nil {
		return fmt.Errorf("posts.repo.IncrementViewCount: %w", err)
	}
	return nil
}

// ─── Post List Queries ────────────────────────────────────────────────────────

// ListPostsQuery holds all filter options for listing posts.
type ListPostsQuery struct {
	Page       int
	Limit      int
	Search     string
	CategoryID string
	Category   string
	Status     string
	SortBy     string
	Order      string
	Deleted    bool // if true, return deleted posts; if false, exclude deleted
	HiddenCats bool // if true, include posts from hidden categories
}

// ListPosts returns a paginated list of posts matching the query.
func (r *Repository) ListPosts(ctx context.Context, q ListPostsQuery) ([]*entities.Post, int64, error) {
	if err := r.ensureDB(); err != nil {
		return nil, 0, err
	}

	query := r.db.WithContext(ctx).Model(&postgrescore.Post{})
	if q.Deleted {
		query = query.Unscoped().Where("deleted_at IS NOT NULL")
	} else {
		query = query.Where("deleted_at IS NULL")
	}
	if q.Status != "" {
		query = query.Where("status = ?", q.Status)
	}
	if q.CategoryID != "" {
		query = query.Where("category_id = ?", q.CategoryID)
	}
	if q.Category != "" || !q.HiddenCats {
		query = query.Joins("JOIN post_categories ON post_categories.id = posts.category_id")
		if q.Category != "" {
			query = query.Where("post_categories.slug = ?", q.Category)
		}
		if !q.HiddenCats {
			query = query.Where("post_categories.is_hidden = ?", false)
		}
	}
	if q.Search != "" {
		search := "%" + strings.ToLower(q.Search) + "%"
		query = query.Where(
			"LOWER(title) LIKE ? OR LOWER(excerpt) LIKE ? OR LOWER(content) LIKE ? OR LOWER(keywords) LIKE ?",
			search, search, search, search,
		)
	}

	sortField := allowedPostSort(q.SortBy)
	sortOrder := "DESC"
	if strings.ToUpper(q.Order) == "ASC" {
		sortOrder = "ASC"
	}
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
	skip := (page - 1) * limit

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("posts.repo.ListPosts.Count: %w", err)
	}

	var records []postgrescore.Post
	if err := query.Order(sortField + " " + sortOrder).Offset(skip).Limit(limit).Find(&records).Error; err != nil {
		return nil, 0, fmt.Errorf("posts.repo.ListPosts.Find: %w", err)
	}

	posts := make([]*entities.Post, 0, len(records))
	for i := range records {
		post, err := recordToPost(&records[i])
		if err != nil {
			return nil, 0, fmt.Errorf("posts.repo.ListPosts.Decode: %w", err)
		}
		posts = append(posts, post)
	}
	return posts, total, nil
}

// ─── Category CRUD ─────────────────────────────────────────────────────────────

// CreateCategory inserts a new category.
func (r *Repository) CreateCategory(ctx context.Context, cat *entities.Category) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	now := time.Now().UTC()
	cat.CreatedAt = now
	cat.UpdatedAt = now

	record := &postgrescore.PostCategory{
		ID:          cat.ID.Hex(),
		Name:        cat.Name,
		Slug:        cat.Slug,
		Description: cat.Description,
		Icon:        cat.Icon,
		IsHidden:    cat.IsHidden,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		if isDuplicateKey(err) {
			return fmt.Errorf("posts.repo.CreateCategory: category slug already exists")
		}
		return fmt.Errorf("posts.repo.CreateCategory: %w", err)
	}
	return nil
}

// GetCategoryByID retrieves a category by ID.
func (r *Repository) GetCategoryByID(ctx context.Context, id string) (*entities.Category, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	var record postgrescore.PostCategory
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("posts.repo.GetCategoryByID: %w", err)
	}
	return recordToCategory(&record)
}

// GetCategoryBySlug retrieves a category by slug.
func (r *Repository) GetCategoryBySlug(ctx context.Context, slug string) (*entities.Category, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	var record postgrescore.PostCategory
	err := r.db.WithContext(ctx).Where("slug = ?", slug).First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("posts.repo.GetCategoryBySlug: %w", err)
	}
	return recordToCategory(&record)
}

// UpdateCategory updates an existing category.
func (r *Repository) UpdateCategory(ctx context.Context, id string, updates bson.M) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	updates["updated_at"] = time.Now().UTC()
	if err := r.db.WithContext(ctx).Model(&postgrescore.PostCategory{}).Where("id = ?", id).Updates(convertCategoryUpdates(updates)).Error; err != nil {
		return fmt.Errorf("posts.repo.UpdateCategory: %w", err)
	}
	return nil
}

// DeleteCategory removes a category.
func (r *Repository) DeleteCategory(ctx context.Context, id string) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).Where("id = ?", id).Delete(&postgrescore.PostCategory{}).Error; err != nil {
		return fmt.Errorf("posts.repo.DeleteCategory: %w", err)
	}
	return nil
}

// ListCategories returns all categories, optionally filtering hidden ones.
func (r *Repository) ListCategories(ctx context.Context, includeHidden bool) ([]*entities.Category, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	query := r.db.WithContext(ctx).Model(&postgrescore.PostCategory{})
	if !includeHidden {
		query = query.Where("is_hidden = ?", false)
	}
	var records []postgrescore.PostCategory
	if err := query.Order("name ASC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("posts.repo.ListCategories: %w", err)
	}
	cats := make([]*entities.Category, 0, len(records))
	for i := range records {
		cat, err := recordToCategory(&records[i])
		if err != nil {
			return nil, fmt.Errorf("posts.repo.ListCategories.Decode: %w", err)
		}
		cats = append(cats, cat)
	}
	return cats, nil
}

// CountPostsByCategory returns how many non-deleted posts belong to a category.
func (r *Repository) CountPostsByCategory(ctx context.Context, categoryID string) (int64, error) {
	if err := r.ensureDB(); err != nil {
		return 0, err
	}
	var count int64
	if err := r.db.WithContext(ctx).Model(&postgrescore.Post{}).
		Where("category_id = ? AND deleted_at IS NULL", categoryID).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("posts.repo.CountPostsByCategory: %w", err)
	}
	return count, nil
}

// GetVisibleCategoryIDs returns IDs of categories that are not hidden.
func (r *Repository) GetVisibleCategoryIDs(ctx context.Context) ([]string, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	type row struct {
		ID string
	}
	var rows []row
	if err := r.db.WithContext(ctx).
		Table("post_categories").
		Select("id").
		Where("is_hidden = ?", false).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("posts.repo.GetVisibleCategoryIDs: %w", err)
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.ID != "" {
			ids = append(ids, row.ID)
		}
	}
	return ids, nil
}

// GetAuthorByID resolves a lightweight author reference from the users table.
func (r *Repository) GetAuthorByID(ctx context.Context, id string) (*entities.AuthorRef, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	var user postgrescore.AuthUser
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("posts.repo.GetAuthorByID: %w", err)
	}
	objID, err := bson.ObjectIDFromHex(user.ID)
	if err != nil {
		return nil, err
	}
	return &entities.AuthorRef{
		ID:        objID,
		FullName:  user.FullName,
		AvatarURL: user.AvatarURL,
	}, nil
}

func allowedPostSort(sortBy string) string {
	switch sortBy {
	case "title", "updated_at", "published_at", "view_count", "created_at", "status":
		return sortBy
	default:
		return "created_at"
	}
}

func postToRecord(post *entities.Post) (*postgrescore.Post, error) {
	metaJSON, err := marshalJSON(post.Meta)
	if err != nil {
		return nil, err
	}
	schemaMarkupJSON, err := marshalJSON(post.SchemaMarkup)
	if err != nil {
		return nil, err
	}
	schemaDataJSON, err := marshalJSON(post.SchemaData)
	if err != nil {
		return nil, err
	}
	faqItemsJSON, err := marshalJSON(post.FAQItems)
	if err != nil {
		return nil, err
	}
	howToStepsJSON, err := marshalJSON(post.HowToSteps)
	if err != nil {
		return nil, err
	}
	introVideoJSON, err := marshalJSON(post.IntroVideo)
	if err != nil {
		return nil, err
	}
	tagsJSON, err := marshalJSON(post.Tags)
	if err != nil {
		return nil, err
	}

	return &postgrescore.Post{
		ID:               post.ID.Hex(),
		Title:            post.Title,
		Slug:             post.Slug,
		Excerpt:          post.Excerpt,
		Content:          post.Content,
		MetaJSON:         metaJSON,
		ThumbnailURL:     post.Thumbnail,
		Status:           string(post.Status),
		IsPublished:      post.IsPublished,
		PublishedAt:      post.PublishedAt,
		CreatedByID:      post.CreatedByID,
		PublishedByID:    post.PublishedBy,
		AuthorID:         post.AuthorID.Hex(),
		ViewCount:        post.ViewCount,
		CommentCount:     post.CommentCount,
		CategoryID:       post.CategoryID.Hex(),
		IsCreatedByAI:    post.IsCreatedByAI,
		AIPrompt:         post.AIPrompt,
		AIJobID:          post.AIJobID,
		MetaTitle:        post.MetaTitle,
		MetaDescription:  post.MetaDescription,
		FocusKeyword:     post.FocusKeyword,
		Keywords:         post.Keywords,
		CanonicalURL:     post.CanonicalURL,
		SchemaType:       string(post.SchemaType),
		SEOScore:         post.SEOScore,
		ReadabilityScore: post.ReadabilityScore,
		KeywordDensity:   post.KeywordDensity,
		SchemaMarkupJSON: schemaMarkupJSON,
		SchemaDataJSON:   schemaDataJSON,
		RobotsIndex:      post.RobotsIndex,
		RobotsFollow:     post.RobotsFollow,
		RobotsAdvanced:   post.RobotsAdvanced,
		OGTitle:          post.OGTitle,
		OGDescription:    post.OGDescription,
		OGImage:          post.OGImage,
		TwitterCard:      post.TwitterCard,
		BreadcrumbTitle:  post.BreadcrumbTitle,
		FAQItemsJSON:     faqItemsJSON,
		HowToStepsJSON:   howToStepsJSON,
		IntroVideoJSON:   introVideoJSON,
		TagsJSON:         tagsJSON,
		CreatedAt:        post.CreatedAt.UTC(),
		UpdatedAt:        post.UpdatedAt.UTC(),
	}, nil
}

func recordToPost(record *postgrescore.Post) (*entities.Post, error) {
	postID, err := bson.ObjectIDFromHex(record.ID)
	if err != nil {
		return nil, err
	}
	authorID, err := parseObjectID(record.AuthorID)
	if err != nil {
		return nil, err
	}
	categoryID, err := parseObjectID(record.CategoryID)
	if err != nil {
		return nil, err
	}

	meta, err := unmarshalMap(record.MetaJSON)
	if err != nil {
		return nil, err
	}
	schemaMarkup, err := unmarshalMap(record.SchemaMarkupJSON)
	if err != nil {
		return nil, err
	}
	schemaData, err := unmarshalMap(record.SchemaDataJSON)
	if err != nil {
		return nil, err
	}
	var faqItems []entities.FAQItem
	if err := unmarshalInto(record.FAQItemsJSON, &faqItems); err != nil {
		return nil, err
	}
	var howToSteps []entities.HowToStep
	if err := unmarshalInto(record.HowToStepsJSON, &howToSteps); err != nil {
		return nil, err
	}
	var introVideo *entities.IntroVideo
	if record.IntroVideoJSON != "" {
		var value entities.IntroVideo
		if err := unmarshalInto(record.IntroVideoJSON, &value); err != nil {
			return nil, err
		}
		introVideo = &value
	}
	var tags []string
	if err := unmarshalInto(record.TagsJSON, &tags); err != nil {
		return nil, err
	}

	post := &entities.Post{
		ID:               postID,
		Title:            record.Title,
		Slug:             record.Slug,
		Excerpt:          record.Excerpt,
		Content:          record.Content,
		Thumbnail:        record.ThumbnailURL,
		Status:           entities.PostStatus(record.Status),
		IsPublished:      record.IsPublished,
		AuthorID:         authorID,
		CreatedByID:      record.CreatedByID,
		PublishedBy:      record.PublishedByID,
		CategoryID:       categoryID,
		ViewCount:        record.ViewCount,
		CommentCount:     record.CommentCount,
		IsCreatedByAI:    record.IsCreatedByAI,
		AIPrompt:         record.AIPrompt,
		AIJobID:          record.AIJobID,
		MetaTitle:        record.MetaTitle,
		MetaDescription:  record.MetaDescription,
		FocusKeyword:     record.FocusKeyword,
		Keywords:         record.Keywords,
		CanonicalURL:     record.CanonicalURL,
		SchemaType:       entities.SchemaType(record.SchemaType),
		SEOScore:         record.SEOScore,
		ReadabilityScore: record.ReadabilityScore,
		KeywordDensity:   record.KeywordDensity,
		SchemaMarkup:     schemaMarkup,
		SchemaData:       schemaData,
		RobotsIndex:      record.RobotsIndex,
		RobotsFollow:     record.RobotsFollow,
		RobotsAdvanced:   record.RobotsAdvanced,
		OGTitle:          record.OGTitle,
		OGDescription:    record.OGDescription,
		OGImage:          record.OGImage,
		TwitterCard:      record.TwitterCard,
		BreadcrumbTitle:  record.BreadcrumbTitle,
		FAQItems:         faqItems,
		HowToSteps:       howToSteps,
		IntroVideo:       introVideo,
		Meta:             meta,
		PublishedAt:      record.PublishedAt,
		CreatedAt:        record.CreatedAt,
		UpdatedAt:        record.UpdatedAt,
		Tags:             tags,
	}
	if record.DeletedAt.Valid {
		deletedAt := record.DeletedAt.Time
		post.DeletedAt = &deletedAt
	}
	return post, nil
}

func recordToCategory(record *postgrescore.PostCategory) (*entities.Category, error) {
	id, err := bson.ObjectIDFromHex(record.ID)
	if err != nil {
		return nil, err
	}
	return &entities.Category{
		ID:          id,
		Name:        record.Name,
		Slug:        record.Slug,
		Description: record.Description,
		Icon:        record.Icon,
		IsHidden:    record.IsHidden,
		CreatedAt:   record.CreatedAt,
		UpdatedAt:   record.UpdatedAt,
	}, nil
}

func convertPostUpdates(updates bson.M) map[string]any {
	result := make(map[string]any, len(updates))
	for key, value := range updates {
		switch key {
		case "thumbnail_url":
			result["thumbnail_url"] = value
		case "published_by_id":
			result["published_by_id"] = value
		case "category_id":
			result["category_id"] = stringifyObjectID(value)
		case "schema_type":
			result["schema_type"] = fmt.Sprint(value)
		case "schema_markup":
			if raw, err := marshalJSON(value); err == nil {
				result["schema_markup_json"] = raw
			}
		case "schema_data":
			if raw, err := marshalJSON(value); err == nil {
				result["schema_data_json"] = raw
			}
		case "faq_items":
			if raw, err := marshalJSON(value); err == nil {
				result["faq_items_json"] = raw
			}
		case "how_to_steps":
			if raw, err := marshalJSON(value); err == nil {
				result["how_to_steps_json"] = raw
			}
		case "intro_video":
			if raw, err := marshalJSON(value); err == nil {
				result["intro_video_json"] = raw
			}
		case "tags":
			if raw, err := marshalJSON(value); err == nil {
				result["tags_json"] = raw
			}
		case "meta":
			if raw, err := marshalJSON(value); err == nil {
				result["meta_json"] = raw
			}
		default:
			result[key] = value
		}
	}
	return result
}

func convertCategoryUpdates(updates bson.M) map[string]any {
	result := make(map[string]any, len(updates))
	for key, value := range updates {
		result[key] = value
	}
	return result
}

func stringifyObjectID(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case bson.ObjectID:
		return v.Hex()
	default:
		return fmt.Sprint(v)
	}
}

func parseObjectID(value string) (bson.ObjectID, error) {
	if value == "" {
		return bson.NilObjectID, nil
	}
	return bson.ObjectIDFromHex(value)
}

func marshalJSON(value any) (string, error) {
	if value == nil {
		return "", nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func unmarshalMap(raw string) (map[string]any, error) {
	if raw == "" {
		return nil, nil
	}
	var value map[string]any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return nil, err
	}
	return value, nil
}

func unmarshalInto(raw string, out any) error {
	if raw == "" {
		return nil
	}
	return json.Unmarshal([]byte(raw), out)
}

func isDuplicateKey(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate key") || strings.Contains(msg, "unique constraint")
}
