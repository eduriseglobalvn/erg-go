package entities

import "time"

// DocumentStatus values.
const (
	DocStatusProcessing = "PROCESSING"
	DocStatusReady      = "READY"
	DocStatusFailed     = "FAILED"
)

// StorageType values.
const (
	StorageR2     = "R2"
	StorageGDrive = "GDRIVE"
)

// Document represents an uploaded document stored in MongoDB + R2.
type Document struct {
	ID              string          `bson:"_id,omitempty"`
	TenantID        string          `bson:"tenant_id"`
	Filename        string          `bson:"filename"`
	OriginalName    string          `bson:"original_name"`
	MimeType        string          `bson:"mime_type"`
	Size            int64           `bson:"size"`
	StorageType     string          `bson:"storage_type"`
	R2URL           string          `bson:"r2_url,omitempty"`
	DriveID         string          `bson:"drive_id,omitempty"`
	WatermarkConfig WatermarkConfig `bson:"watermark_config"`
	Status          string          `bson:"status"`
	UploadedBy      string          `bson:"uploaded_by"`
	CreatedAt       time.Time       `bson:"created_at"`
	UpdatedAt       time.Time       `bson:"updated_at"`
}

// WatermarkConfig controls how watermarks are applied to PDF pages.
type WatermarkConfig struct {
	Text     string  `bson:"text" json:"text"`
	Position string  `bson:"position" json:"position"` // CENTER | CORNER | TILED
	Opacity  float64 `bson:"opacity" json:"opacity"`
	Color    string  `bson:"color" json:"color"`
	FontSize int     `bson:"font_size" json:"font_size"`
	PerPage  bool    `bson:"per_page" json:"per_page"`
	OffsetX  float64 `bson:"offset_x,omitempty" json:"offset_x,omitempty"`
	OffsetY  float64 `bson:"offset_y,omitempty" json:"offset_y,omitempty"`
	Rotation float64 `bson:"rotation,omitempty" json:"rotation,omitempty"`
}

// DocumentCollection is the MongoDB collection name for documents.
const DocumentCollection = "cms_documents"
