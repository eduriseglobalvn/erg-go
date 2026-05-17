package entity

import (
	"time"

	"erg.ninja/internal/modules/hoclieu/api/dto"
)

const (
	collectionTaxonomy      = "hoclieu_taxonomy"
	collectionPrograms      = "hoclieu_programs"
	collectionDesigner      = "hoclieu_designer_presets"
	collectionResources     = "hoclieu_resources"
	collectionAssets        = "hoclieu_assets"
	collectionResourceItems = "hoclieu_resource_items"
	collectionVersions      = "hoclieu_versions"
)

// TaxonomyNode is the persistent model contract for the Hoclieu content tree.
// It intentionally supports arbitrary depth so future subjects can introduce
// their own hierarchy without changing schema.
type TaxonomyNode struct {
	ID           string    `bson:"_id,omitempty" json:"id"`
	TenantID     string    `bson:"tenant_id" json:"tenantId"`
	Kind         string    `bson:"kind" json:"kind"`
	Label        string    `bson:"label" json:"label"`
	Slug         string    `bson:"slug" json:"slug"`
	ParentID     string    `bson:"parent_id,omitempty" json:"parentId,omitempty"`
	SubjectID    string    `bson:"subject_id,omitempty" json:"subjectId,omitempty"`
	GradeID      string    `bson:"grade_id,omitempty" json:"gradeId,omitempty"`
	CategoryID   string    `bson:"category_id,omitempty" json:"categoryId,omitempty"`
	BookSeriesID string    `bson:"book_series_id,omitempty" json:"bookSeriesId,omitempty"`
	LevelIDs     []string  `bson:"level_ids,omitempty" json:"levelIds,omitempty"`
	Depth        int       `bson:"depth" json:"depth"`
	SortOrder    int       `bson:"sort_order" json:"sortOrder"`
	Status       string    `bson:"status" json:"status"`
	Description  string    `bson:"description,omitempty" json:"description,omitempty"`
	CreatedAt    time.Time `bson:"created_at" json:"createdAt"`
	UpdatedAt    time.Time `bson:"updated_at" json:"updatedAt"`
}

type ResourceRecord struct {
	ID             string                `bson:"_id,omitempty" json:"id"`
	TenantID       string                `bson:"tenant_id" json:"tenantId"`
	ProgramSlug    string                `bson:"program_slug" json:"programSlug"`
	SubjectID      string                `bson:"subject_id" json:"subjectId"`
	GradeID        string                `bson:"grade_id,omitempty" json:"gradeId,omitempty"`
	CategoryID     string                `bson:"category_id" json:"categoryId"`
	SectionID      string                `bson:"section_id,omitempty" json:"sectionId,omitempty"`
	BookSeriesID   string                `bson:"book_series_id,omitempty" json:"bookSeriesId,omitempty"`
	TopicID        string                `bson:"topic_id,omitempty" json:"topicId,omitempty"`
	LevelID        string                `bson:"level_id,omitempty" json:"levelId,omitempty"`
	DocumentTypeID string                `bson:"document_type_id,omitempty" json:"documentTypeId,omitempty"`
	Title          string                `bson:"title" json:"title"`
	Slug           string                `bson:"slug" json:"slug"`
	Subtitle       string                `bson:"subtitle,omitempty" json:"subtitle,omitempty"`
	Description    string                `bson:"description,omitempty" json:"description,omitempty"`
	ThumbnailURL   string                `bson:"thumbnail_url,omitempty" json:"thumbnailUrl,omitempty"`
	Tags           []string              `bson:"tags,omitempty" json:"tags,omitempty"`
	Metadata       map[string]string     `bson:"metadata,omitempty" json:"metadata,omitempty"`
	Visibility     string                `bson:"visibility" json:"visibility"`
	Status         string                `bson:"status" json:"status"`
	PublishedAt    *time.Time            `bson:"published_at,omitempty" json:"publishedAt,omitempty"`
	LectureDesign  *dto.LectureDesignDTO `bson:"lecture_design,omitempty" json:"lectureDesign,omitempty"`
	CreatedBy      string                `bson:"created_by,omitempty" json:"createdBy,omitempty"`
	UpdatedBy      string                `bson:"updated_by,omitempty" json:"updatedBy,omitempty"`
	CreatedAt      time.Time             `bson:"created_at" json:"createdAt"`
	UpdatedAt      time.Time             `bson:"updated_at" json:"updatedAt"`
}

type AssetRecord struct {
	ID               string            `bson:"_id,omitempty" json:"id"`
	ResourceID       string            `bson:"resource_id" json:"resourceId"`
	Title            string            `bson:"title" json:"title"`
	SelectedFileType dto.AssetFileType `bson:"selected_file_type" json:"selectedFileType"`
	LaunchMode       dto.LaunchMode    `bson:"launch_mode" json:"launchMode"`
	OriginalFileName string            `bson:"original_file_name,omitempty" json:"originalFileName,omitempty"`
	DetectedMimeType string            `bson:"detected_mime_type,omitempty" json:"detectedMimeType,omitempty"`
	FileExtension    string            `bson:"file_extension,omitempty" json:"fileExtension,omitempty"`
	FileSizeBytes    int64             `bson:"file_size_bytes,omitempty" json:"fileSizeBytes,omitempty"`
	StorageProvider  string            `bson:"storage_provider" json:"storageProvider"`
	StorageURL       string            `bson:"storage_url,omitempty" json:"storageUrl,omitempty"`
	CanDownload      bool              `bson:"can_download" json:"canDownload"`
	Status           string            `bson:"status" json:"status"`
	CreatedAt        time.Time         `bson:"created_at" json:"createdAt"`
	UpdatedAt        time.Time         `bson:"updated_at" json:"updatedAt"`
}

type ResourceItemRecord struct {
	ID          string    `bson:"_id,omitempty" json:"id"`
	TenantID    string    `bson:"tenant_id" json:"tenantId"`
	ResourceID  string    `bson:"resource_id" json:"resourceId"`
	AssetID     string    `bson:"asset_id,omitempty" json:"assetId,omitempty"`
	UnitTitle   string    `bson:"unit_title" json:"unitTitle"`
	LessonTitle string    `bson:"lesson_title,omitempty" json:"lessonTitle,omitempty"`
	SortOrder   int       `bson:"sort_order" json:"sortOrder"`
	PageCount   int       `bson:"page_count,omitempty" json:"pageCount,omitempty"`
	DurationSec int       `bson:"duration_sec,omitempty" json:"durationSec,omitempty"`
	CreatedAt   time.Time `bson:"created_at" json:"createdAt"`
	UpdatedAt   time.Time `bson:"updated_at" json:"updatedAt"`
}
