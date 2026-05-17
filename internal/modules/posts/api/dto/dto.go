// Package dto provides request/response types for the posts module.
package dto

import "time"

// ─── Post Request DTOs ─────────────────────────────────────────────────────────

// CreatePostRequest is the payload for POST /api/posts.
type CreatePostRequest struct {
	Title            string         `json:"title" validate:"required,min=1,max=500"`
	Slug             string         `json:"slug,omitempty"`
	Excerpt          string         `json:"excerpt,omitempty"`
	Content          string         `json:"content,omitempty"`
	Thumbnail        string         `json:"thumbnailUrl,omitempty"`
	Status           string         `json:"status,omitempty"`
	CategoryID       string         `json:"categoryId" validate:"required"`
	Tags             []string       `json:"tags,omitempty"`
	IsCreatedByAI    bool           `json:"isCreatedByAI"`
	AIPrompt         string         `json:"aiPrompt,omitempty"`
	AIJobID          string         `json:"aiJobId,omitempty"`
	MetaTitle        string         `json:"metaTitle,omitempty"`
	MetaDescription  string         `json:"metaDescription,omitempty"`
	FocusKeyword     string         `json:"focusKeyword,omitempty"`
	Keywords         string         `json:"keywords,omitempty"`
	CanonicalURL     string         `json:"canonicalUrl,omitempty"`
	SchemaType       string         `json:"schemaType,omitempty"`
	SEOScore         int            `json:"seoScore,omitempty"`
	ReadabilityScore int            `json:"readabilityScore,omitempty"`
	KeywordDensity   float64        `json:"keywordDensity,omitempty"`
	Meta             map[string]any `json:"meta,omitempty"`
}

// UpdatePostRequest is the payload for PUT /api/posts/:id.
type UpdatePostRequest struct {
	Title            string         `json:"title,omitempty"`
	Slug             string         `json:"slug,omitempty"`
	Excerpt          string         `json:"excerpt,omitempty"`
	Content          string         `json:"content,omitempty"`
	Thumbnail        string         `json:"thumbnailUrl,omitempty"`
	Status           string         `json:"status,omitempty"`
	CategoryID       string         `json:"categoryId,omitempty"`
	Tags             []string       `json:"tags,omitempty"`
	IsCreatedByAI    *bool          `json:"isCreatedByAI,omitempty"`
	AIPrompt         string         `json:"aiPrompt,omitempty"`
	AIJobID          string         `json:"aiJobId,omitempty"`
	MetaTitle        string         `json:"metaTitle,omitempty"`
	MetaDescription  string         `json:"metaDescription,omitempty"`
	FocusKeyword     string         `json:"focusKeyword,omitempty"`
	Keywords         string         `json:"keywords,omitempty"`
	CanonicalURL     string         `json:"canonicalUrl,omitempty"`
	SchemaType       string         `json:"schemaType,omitempty"`
	SEOScore         *int           `json:"seoScore,omitempty"`
	ReadabilityScore *int           `json:"readabilityScore,omitempty"`
	KeywordDensity   *float64       `json:"keywordDensity,omitempty"`
	Meta             map[string]any `json:"meta,omitempty"`
}

// PostQueryParams is the query string for GET /api/posts.
type PostQueryParams struct {
	Page       int    `query:"page"`
	Limit      int    `query:"limit"`
	Search     string `query:"search"`
	Category   string `query:"category"`
	CategoryID string `query:"category_id"`
	Status     string `query:"status"`
	SortBy     string `query:"sortBy"`
	Order      string `query:"order"`
}

// PromoteRequest is the payload for POST /api/posts/:id/promote.
type PromoteRequest struct {
	CategoryID string `json:"category_id" validate:"required"`
}

// SavePreviewRequest is the payload for POST /api/posts/preview.
type SavePreviewRequest struct {
	ID      string         `json:"id,omitempty"`
	Title   string         `json:"title,omitempty"`
	Content string         `json:"content,omitempty"`
	Slug    string         `json:"slug,omitempty"`
	Meta    map[string]any `json:"meta,omitempty"`
}

// UploadImageResponse is the response for POST /api/posts/images/upload.
type UploadImageResponse struct {
	URL string `json:"url"`
}

// ─── Post Response DTOs ────────────────────────────────────────────────────────

// PostResponse is the public-facing post document.
type PostResponse struct {
	ID              string         `json:"id"`
	Title           string         `json:"title"`
	Slug            string         `json:"slug"`
	Excerpt         string         `json:"excerpt,omitempty"`
	Content         string         `json:"content,omitempty"`
	ThumbnailURL    string         `json:"thumbnailUrl,omitempty"`
	Status          string         `json:"status"`
	IsPublished     bool           `json:"isPublished"`
	PublishedAt     *time.Time     `json:"publishedAt,omitempty"`
	ViewCount       int64          `json:"viewCount"`
	CommentCount    int64          `json:"commentCount"`
	Tags            []string       `json:"tags,omitempty"`
	IsCreatedByAI   bool           `json:"isCreatedByAI"`
	Category        *CategoryRef   `json:"category,omitempty"`
	Author          *AuthorRef     `json:"author,omitempty"`
	MetaTitle       string         `json:"metaTitle,omitempty"`
	MetaDescription string         `json:"metaDescription,omitempty"`
	FocusKeyword    string         `json:"focusKeyword,omitempty"`
	CanonicalURL    string         `json:"canonicalUrl,omitempty"`
	SchemaType      string         `json:"schemaType,omitempty"`
	SEOScore        int            `json:"seoScore"`
	OGTitle         string         `json:"ogTitle,omitempty"`
	OGDescription   string         `json:"ogDescription,omitempty"`
	OGImage         string         `json:"ogImage,omitempty"`
	TwitterCard     string         `json:"twitterCard,omitempty"`
	SchemaMarkup    map[string]any `json:"schemaMarkup,omitempty"`
	Meta            map[string]any `json:"meta,omitempty"`
	FAQItems        []FAQItemDTO   `json:"faqItems,omitempty"`
	HowToSteps      []HowToStepDTO `json:"howToSteps,omitempty"`
	IntroVideo      *IntroVideoDTO `json:"introVideo,omitempty"`
	CreatedAt       time.Time      `json:"createdAt"`
	UpdatedAt       time.Time      `json:"updatedAt"`
}

// FAQItemDTO mirrors entities.FAQItem for JSON serialization.
type FAQItemDTO struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

// HowToStepDTO mirrors entities.HowToStep for JSON serialization.
type HowToStepDTO struct {
	Name  string `json:"name"`
	Text  string `json:"text"`
	Image string `json:"image,omitempty"`
	URL   string `json:"url,omitempty"`
}

// IntroVideoDTO mirrors entities.IntroVideo for JSON serialization.
type IntroVideoDTO struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	ThumbnailURL string `json:"thumbnailUrl"`
	UploadDate   string `json:"uploadDate"`
	ContentURL   string `json:"contentUrl"`
}

// AuthorRef is a lightweight author reference in post responses.
type AuthorRef struct {
	ID          string          `json:"id"`
	FullName    string          `json:"fullName"`
	AvatarURL   string          `json:"avatarUrl,omitempty"`
	SocialLinks *SocialLinksDTO `json:"socialLinks,omitempty"`
}

// SocialLinksDTO holds social media links for an author.
type SocialLinksDTO struct {
	LinkedIn string `json:"linkedin,omitempty"`
	Twitter  string `json:"twitter,omitempty"`
	Facebook string `json:"facebook,omitempty"`
}

// CategoryRef is a lightweight category reference in post responses.
type CategoryRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// ─── Category DTOs ─────────────────────────────────────────────────────────────

// CreateCategoryRequest is the payload for POST /api/posts/categories.
type CreateCategoryRequest struct {
	Name        string `json:"name" validate:"required,min=1,max=200"`
	Slug        string `json:"slug,omitempty"`
	Description string `json:"description,omitempty"`
	Icon        string `json:"icon,omitempty"`
}

// UpdateCategoryRequest is the payload for PUT /api/posts/categories/:id.
type UpdateCategoryRequest struct {
	Name        string `json:"name,omitempty"`
	Slug        string `json:"slug,omitempty"`
	Description string `json:"description,omitempty"`
	Icon        string `json:"icon,omitempty"`
	IsHidden    *bool  `json:"isHidden,omitempty"`
}

// CategoryResponse is the public-facing category document.
type CategoryResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description,omitempty"`
	Icon        string    `json:"icon,omitempty"`
	IsHidden    bool      `json:"isHidden"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// ─── List/Count Response DTOs ──────────────────────────────────────────────────

// PostListResponse holds a paginated list of posts.
type PostListResponse struct {
	Data []PostResponse `json:"data"`
	Meta *PostMeta      `json:"meta"`
}

// PostMeta contains pagination metadata.
type PostMeta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int64 `json:"totalPages"`
}

// CategoryListResponse holds a list of categories.
type CategoryListResponse struct {
	Data []CategoryResponse `json:"data"`
}
