package dto

import "time"

// ─── Request DTOs ──────────────────────────────────────────────────────────────

// CreatePageRequest is the payload for creating a page.
type CreatePageRequest struct {
	TenantID        string `json:"tenant_id" validate:"required"`
	Slug            string `json:"slug" validate:"required,max=255"`
	Domain          string `json:"domain" validate:"required"`
	Title           string `json:"title" validate:"required,max=500"`
	Content         string `json:"content"`
	MetaTitle       string `json:"metaTitle" validate:"omitempty,max=120"`
	MetaDescription string `json:"metaDescription" validate:"omitempty,max=320"`
	FaqJSON         string `json:"faq_json"`
	Status          string `json:"status" validate:"required,oneof=DRAFT PUBLISHED"`
}

// UpdatePageRequest is the payload for updating a page.
type UpdatePageRequest struct {
	Title           *string `json:"title" validate:"omitempty,max=500"`
	Content         *string `json:"content"`
	MetaTitle       *string `json:"metaTitle" validate:"omitempty,max=120"`
	MetaDescription *string `json:"metaDescription" validate:"omitempty,max=320"`
	FaqJSON         *string `json:"faq_json"`
	Status          *string `json:"status" validate:"omitempty,oneof=DRAFT PUBLISHED"`
}

// ─── Response DTOs ────────────────────────────────────────────────────────────

// PageResponse is the public-facing page payload with FAQ already parsed.
type PageResponse struct {
	ID              string     `json:"id"`
	Slug            string     `json:"slug"`
	Domain          string     `json:"domain"`
	Title           string     `json:"title"`
	Content         string     `json:"content"`
	MetaTitle       string     `json:"metaTitle"`
	MetaDescription string     `json:"metaDescription"`
	Faqs            []FaqItem  `json:"faqs"`
	PublishedAt     *time.Time `json:"published_at,omitempty"`
}

// FaqItem represents a single FAQ entry.
type FaqItem struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}
