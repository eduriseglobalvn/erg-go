package entities

import (
	"time"

	docEntities "erg.ninja/internal/modules/documents/domain/entity"
)

// DisclosureDocument represents a public disclosure record with its metadata and file reference.
type DisclosureDocument struct {
	ID               string                      `bson:"_id,omitempty" json:"id"`
	TenantID         string                      `bson:"tenant_id" json:"tenant_id"`
	SectionSlug      string                      `bson:"section_slug" json:"section_slug"`
	Slug             string                      `bson:"slug" json:"slug"`
	Title            string                      `bson:"title" json:"title"`
	MenuLabel        string                      `bson:"menu_label" json:"menu_label"`
	ShortDescription string                      `bson:"short_description" json:"short_description"`
	Description      string                      `bson:"description" json:"description"`
	PublishedAt      string                      `bson:"published_at" json:"published_at"`
	EffectiveDate    string                      `bson:"effective_date" json:"effective_date"`
	ReferenceCode    string                      `bson:"reference_code" json:"reference_code"`
	IssuingAuthority string                      `bson:"issuing_authority" json:"issuing_authority"`
	ReviewCycle      string                      `bson:"review_cycle" json:"review_cycle"`
	AccessScope      string                      `bson:"access_scope" json:"access_scope"`
	HeroKicker       string                      `bson:"hero_kicker" json:"hero_kicker"`
	Highlights       []string                    `bson:"highlights" json:"highlights"`
	DetailBlocks     []DetailBlock               `bson:"detail_blocks" json:"detail_blocks"`
	Cover            CoverConfig                 `bson:"cover" json:"cover"`
	DocumentID       string                      `bson:"document_id,omitempty" json:"document_id"` // Reference to cms_documents
	ThumbnailURL     string                      `bson:"thumbnail_url,omitempty" json:"thumbnail_url"`
	SchoolYear       string                      `bson:"school_year" json:"school_year"`
	WatermarkConfig  docEntities.WatermarkConfig `bson:"watermark_config,omitempty" json:"watermark_config,omitempty"`
	CreatedAt        time.Time                   `bson:"created_at" json:"created_at"`
	UpdatedAt        time.Time                   `bson:"updated_at" json:"updated_at"`
}

type DetailBlock struct {
	Heading string `bson:"heading" json:"heading"`
	Body    string `bson:"body" json:"body"`
}

type CoverConfig struct {
	Eyebrow  string `bson:"eyebrow" json:"eyebrow"`
	IssuedBy string `bson:"issued_by" json:"issued_by"`
	Title    string `bson:"title" json:"title"`
	Subtitle string `bson:"subtitle" json:"subtitle"`
	Footer   string `bson:"footer" json:"footer"`
}

const DisclosureCollection = "cms_public_disclosures"
