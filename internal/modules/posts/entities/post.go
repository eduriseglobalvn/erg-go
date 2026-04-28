// Package entities provides domain models for the posts module.
package entities

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// PostStatus represents the publication status of a post.
type PostStatus string

const (
	PostStatusDraft     PostStatus = "DRAFT"
	PostStatusPublished PostStatus = "PUBLISHED"
	PostStatusArchived  PostStatus = "ARCHIVED"
	PostStatusTrash     PostStatus = "TRASH"
)

// SchemaType represents the structured data schema type.
type SchemaType string

const (
	SchemaTypeArticle     SchemaType = "Article"
	SchemaTypeNewsArticle SchemaType = "NewsArticle"
	SchemaTypeBlogPosting SchemaType = "BlogPosting"
	SchemaTypeCourse      SchemaType = "Course"
	SchemaTypeJobPosting  SchemaType = "JobPosting"
	SchemaTypeEvent       SchemaType = "Event"
	SchemaTypeProduct     SchemaType = "Product"
)

// Post represents a blog post document stored in MongoDB.
type Post struct {
	ID          bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Title       string     `bson:"title" json:"title"`
	Slug        string     `bson:"slug" json:"slug"`
	Excerpt     string     `bson:"excerpt,omitempty" json:"excerpt,omitempty"`
	Content     string     `bson:"content,omitempty" json:"content,omitempty"`
	Thumbnail   string     `bson:"thumbnail_url,omitempty" json:"thumbnail_url,omitempty"`
	Status      PostStatus `bson:"status" json:"status"`
	IsPublished bool       `bson:"is_published" json:"is_published"`

	// Author
	AuthorID    bson.ObjectID `bson:"author_id" json:"author_id"`
	CreatedByID string        `bson:"created_by_id" json:"created_by_id"`
	PublishedBy string        `bson:"published_by_id,omitempty" json:"published_by_id,omitempty"`

	// Category
	CategoryID bson.ObjectID `bson:"category_id" json:"category_id"`

	// Stats
	ViewCount    int64 `bson:"view_count" json:"viewCount"`
	CommentCount int64 `bson:"comment_count" json:"commentCount"`

	// AI metadata
	IsCreatedByAI bool   `bson:"is_created_by_ai" json:"isCreatedByAI"`
	AIPrompt      string `bson:"ai_prompt,omitempty" json:"ai_prompt,omitempty"`
	AIJobID       string `bson:"ai_job_id,omitempty" json:"ai_job_id,omitempty"`

	// SEO
	MetaTitle        string     `bson:"meta_title,omitempty" json:"meta_title,omitempty"`
	MetaDescription  string     `bson:"meta_description,omitempty" json:"meta_description,omitempty"`
	FocusKeyword     string     `bson:"focus_keyword,omitempty" json:"focus_keyword,omitempty"`
	Keywords         string     `bson:"keywords,omitempty" json:"keywords,omitempty"`
	CanonicalURL     string     `bson:"canonical_url,omitempty" json:"canonical_url,omitempty"`
	SchemaType       SchemaType `bson:"schema_type,omitempty" json:"schema_type,omitempty"`
	SEOScore         int        `bson:"seo_score" json:"seoScore"`
	ReadabilityScore int        `bson:"readability_score" json:"readabilityScore"`
	KeywordDensity   float64    `bson:"keyword_density" json:"keywordDensity"`

	// Advanced SEO
	SchemaMarkup   map[string]any `bson:"schema_markup,omitempty" json:"schema_markup,omitempty"`
	SchemaData     map[string]any `bson:"schema_data,omitempty" json:"schema_data,omitempty"`
	RobotsIndex    bool           `bson:"robots_index" json:"robots_index"`
	RobotsFollow   bool           `bson:"robots_follow" json:"robots_follow"`
	RobotsAdvanced string         `bson:"robots_advanced,omitempty" json:"robots_advanced,omitempty"`

	// Open Graph
	OGTitle       string `bson:"og_title,omitempty" json:"og_title,omitempty"`
	OGDescription string `bson:"og_description,omitempty" json:"og_description,omitempty"`
	OGImage       string `bson:"og_image,omitempty" json:"og_image,omitempty"`

	// Twitter Card
	TwitterCard string `bson:"twitter_card,omitempty" json:"twitter_card,omitempty"`

	// SEO Advanced
	BreadcrumbTitle string      `bson:"breadcrumb_title,omitempty" json:"breadcrumb_title,omitempty"`
	FAQItems        []FAQItem   `bson:"faq_items,omitempty" json:"faq_items,omitempty"`
	HowToSteps      []HowToStep `bson:"how_to_steps,omitempty" json:"how_to_steps,omitempty"`
	IntroVideo      *IntroVideo `bson:"intro_video,omitempty" json:"intro_video,omitempty"`

	// Meta (JSON blob for extra config like TOC)
	Meta map[string]any `bson:"meta,omitempty" json:"meta,omitempty"`

	// Timestamps
	PublishedAt *time.Time `bson:"published_at,omitempty" json:"published_at,omitempty"`
	CreatedAt   time.Time  `bson:"created_at" json:"createdAt"`
	UpdatedAt   time.Time  `bson:"updated_at" json:"updatedAt"`
	DeletedAt   *time.Time `bson:"deleted_at,omitempty" json:"deleted_at,omitempty"`

	// Tags
	Tags []string `bson:"tags,omitempty" json:"tags,omitempty"`

	// Relations (populated in queries)
	Author   *AuthorRef   `bson:"author,omitempty" json:"author,omitempty"`
	Category *CategoryRef `bson:"category,omitempty" json:"category,omitempty"`
}

// FAQItem represents an FAQ entry.
type FAQItem struct {
	Question string `bson:"question" json:"question"`
	Answer   string `bson:"answer" json:"answer"`
}

// HowToStep represents a step in a how-to guide.
type HowToStep struct {
	Name  string `bson:"name" json:"name"`
	Text  string `bson:"text" json:"text"`
	Image string `bson:"image,omitempty" json:"image,omitempty"`
	URL   string `bson:"url,omitempty" json:"url,omitempty"`
}

// IntroVideo represents a video embedded in a post.
type IntroVideo struct {
	Name         string `bson:"name" json:"name"`
	Description  string `bson:"description" json:"description"`
	ThumbnailURL string `bson:"thumbnail_url" json:"thumbnailUrl"`
	UploadDate   string `bson:"upload_date" json:"upload_date"`
	ContentURL   string `bson:"content_url" json:"content_url"`
}

// AuthorRef is a lightweight author reference embedded in posts.
type AuthorRef struct {
	ID        bson.ObjectID `bson:"id" json:"id"`
	FullName  string        `bson:"full_name" json:"fullName"`
	AvatarURL string        `bson:"avatar_url,omitempty" json:"avatar_url,omitempty"`
}

// CategoryRef is a lightweight category reference embedded in posts.
type CategoryRef struct {
	ID   bson.ObjectID `bson:"id" json:"id"`
	Name string        `bson:"name" json:"name"`
	Slug string        `bson:"slug" json:"slug"`
}

// PostCollection is the MongoDB collection name.
const PostCollection = "posts"
