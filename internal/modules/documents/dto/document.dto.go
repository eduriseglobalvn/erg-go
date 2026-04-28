package dto

import (
	"time"

	"erg.ninja/internal/modules/documents/entities"
)

// ─── Request DTOs ──────────────────────────────────────────────────────────────

// UploadDocumentRequest is the multipart/form-data payload for document upload.
type UploadDocumentRequest struct {
	TenantID     string             `json:"tenant_id" validate:"required"`
	Filename     string             `json:"filename" validate:"required"`
	OriginalName string             `json:"original_name" validate:"required"`
	MimeType     string             `json:"mime_type" validate:"required"`
	UploadedBy   string             `json:"uploaded_by" validate:"required"`
	Watermark    WatermarkConfigDTO `json:"watermark"`
}

// WatermarkConfigDTO is the watermark configuration in request/response payloads.
type WatermarkConfigDTO struct {
	Text     string  `json:"text" validate:"required"`
	Position string  `json:"position" validate:"required,oneof=CENTER CORNER TILED"`
	Opacity  float64 `json:"opacity" validate:"gte=0,lte=1"`
	Color    string  `json:"color" validate:"required"`
	FontSize int     `json:"font_size" validate:"required,gte=8,lte=144"`
	PerPage  bool    `json:"per_page"`
	OffsetX  float64 `json:"offset_x,omitempty"`
	OffsetY  float64 `json:"offset_y,omitempty"`
	Rotation float64 `json:"rotation,omitempty"`
}

// UpdateDocumentRequest is the payload for updating document metadata.
type UpdateDocumentRequest struct {
	Watermark *WatermarkConfigDTO `json:"watermark"`
}

// ─── Response DTOs ────────────────────────────────────────────────────────────

// DocumentResponse is the document metadata returned to clients.
type DocumentResponse struct {
	ID              string             `json:"id"`
	TenantID        string             `json:"tenant_id"`
	Filename        string             `json:"filename"`
	OriginalName    string             `json:"original_name"`
	MimeType        string             `json:"mime_type"`
	Size            int64              `json:"size"`
	StorageType     string             `json:"storage_type"`
	R2URL           string             `json:"r2_url,omitempty"`
	DriveID         string             `json:"drive_id,omitempty"`
	WatermarkConfig WatermarkConfigDTO `json:"watermark_config"`
	Status          string             `json:"status"`
	UploadedBy      string             `json:"uploaded_by"`
	CreatedAt       time.Time          `json:"createdAt"`
	UpdatedAt       time.Time          `json:"updatedAt"`
}

// ToEntity converts a WatermarkConfigDTO to an entity WatermarkConfig.
func (w WatermarkConfigDTO) ToEntity() entities.WatermarkConfig {
	return entities.WatermarkConfig{
		Text:     w.Text,
		Position: w.Position,
		Opacity:  w.Opacity,
		Color:    w.Color,
		FontSize: w.FontSize,
		PerPage:  w.PerPage,
		OffsetX:  w.OffsetX,
		OffsetY:  w.OffsetY,
		Rotation: w.Rotation,
	}
}

// ListDocumentsResponse is the paginated list response.
type ListDocumentsResponse struct {
	Items      []DocumentResponse `json:"items"`
	NextCursor string             `json:"next_cursor,omitempty"`
	Total      int64              `json:"total"`
}

// ToResponse converts a Document entity to a DocumentResponse.
func ToResponse(d *entities.Document) DocumentResponse {
	return DocumentResponse{
		ID:           d.ID,
		TenantID:     d.TenantID,
		Filename:     d.Filename,
		OriginalName: d.OriginalName,
		MimeType:     d.MimeType,
		Size:         d.Size,
		StorageType:  d.StorageType,
		R2URL:        d.R2URL,
		DriveID:      d.DriveID,
		WatermarkConfig: WatermarkConfigDTO{
			Text:     d.WatermarkConfig.Text,
			Position: d.WatermarkConfig.Position,
			Opacity:  d.WatermarkConfig.Opacity,
			Color:    d.WatermarkConfig.Color,
			FontSize: d.WatermarkConfig.FontSize,
			PerPage:  d.WatermarkConfig.PerPage,
			OffsetX:  d.WatermarkConfig.OffsetX,
			OffsetY:  d.WatermarkConfig.OffsetY,
			Rotation: d.WatermarkConfig.Rotation,
		},
		Status:     d.Status,
		UploadedBy: d.UploadedBy,
		CreatedAt:  d.CreatedAt,
		UpdatedAt:  d.UpdatedAt,
	}
}
