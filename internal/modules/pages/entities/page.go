package entities

import (
	"time"
)

// Status values.
const (
	StatusDraft     = "DRAFT"
	StatusPublished = "PUBLISHED"
)

// Page represents a CMS page stored in MongoDB.
type Page struct {
	ID              string     `bson:"_id,omitempty"`
	TenantID        string     `bson:"tenant_id"`
	Slug            string     `bson:"slug"`
	Domain          string     `bson:"domain"`
	Title           string     `bson:"title"`
	Content         string     `bson:"content"`
	MetaTitle       string     `bson:"meta_title"`
	MetaDescription string     `bson:"meta_description"`
	FaqJSON         string     `bson:"faq_json"`
	Status          string     `bson:"status"`
	PublishedAt     *time.Time `bson:"published_at,omitempty"`
	CreatedAt       time.Time  `bson:"created_at"`
	UpdatedAt       time.Time  `bson:"updated_at"`
}

// PageCollection is the MongoDB collection name for pages.
const PageCollection = "cms_pages"
