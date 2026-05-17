package service

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	. "erg.ninja/internal/modules/hoclieu/api/dto"
)

type fileTypeValidationResult struct {
	OriginalFileName string
	DetectedMimeType string
	FileExtension    string
	Warnings         []FileTypeWarningDTO
}

func validateSelectedFileTypeMetadata(fileType AssetFileType, originalFileName, detectedMimeType string) (fileTypeValidationResult, error) {
	result := fileTypeValidationResult{
		OriginalFileName: strings.TrimSpace(originalFileName),
		DetectedMimeType: normalizeMime(detectedMimeType),
		FileExtension:    normalizeExtension(originalFileName),
	}
	if !fileType.Valid() {
		return result, ErrInvalidFileType
	}
	if result.FileExtension != "" && !extensionAllowedFor(fileType, result.FileExtension) {
		return result, fmt.Errorf("%w: selectedFileType=%s does not allow extension %q", ErrInvalidAssetMetadata, fileType, result.FileExtension)
	}
	if result.DetectedMimeType == "" {
		return result, nil
	}
	if mimeAllowedFor(fileType, result.DetectedMimeType) {
		return result, nil
	}
	if isSoftMimeMismatch(fileType, result.FileExtension, result.DetectedMimeType) {
		result.Warnings = append(result.Warnings, FileTypeWarningDTO{
			Code:    "mime_metadata_warning",
			Field:   "detectedMimeType",
			Message: fmt.Sprintf("detected MIME %q is accepted for %s only as a soft metadata warning", result.DetectedMimeType, fileType),
		})
		return result, nil
	}
	return result, fmt.Errorf("%w: selectedFileType=%s does not allow MIME %q", ErrInvalidAssetMetadata, fileType, result.DetectedMimeType)
}

func applyResourceFileTypeMetadata(resource *ResourceDetailDTO, selectedFileType AssetFileType, validation fileTypeValidationResult, audit *FileTypeAuditDTO) {
	resource.SelectedFileType = selectedFileType
	resource.FileTypeBadge = string(selectedFileType)
	resource.LaunchMode = defaultLaunchMode(selectedFileType)
	resource.OriginalFileName = validation.OriginalFileName
	resource.DetectedMimeType = validation.DetectedMimeType
	resource.FileExtension = validation.FileExtension
	resource.MetadataWarnings = append([]FileTypeWarningDTO(nil), validation.Warnings...)
	resource.FileTypeAudit = audit
}

func applyAssetFileTypeMetadata(asset *AssetDTO, selectedFileType AssetFileType, validation fileTypeValidationResult, audit *FileTypeAuditDTO) {
	asset.SelectedFileType = selectedFileType
	asset.FileTypeBadge = string(selectedFileType)
	asset.LaunchMode = defaultLaunchMode(selectedFileType)
	asset.OriginalFileName = validation.OriginalFileName
	asset.DetectedMimeType = validation.DetectedMimeType
	asset.FileExtension = validation.FileExtension
	asset.MetadataWarnings = append([]FileTypeWarningDTO(nil), validation.Warnings...)
	asset.FileTypeAudit = audit
}

func newFileTypeAudit(now time.Time, actorID, source string, previous, current AssetFileType, originalFileName, detectedMimeType string, warnings []FileTypeWarningDTO) *FileTypeAuditDTO {
	warningCodes := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		warningCodes = append(warningCodes, warning.Code)
	}
	return &FileTypeAuditDTO{
		ChangedAt:                &now,
		ChangedBy:                actorOrSystem(actorID),
		PreviousSelectedFileType: previous,
		SelectedFileType:         current,
		Source:                   source,
		OriginalFileName:         strings.TrimSpace(originalFileName),
		DetectedMimeType:         normalizeMime(detectedMimeType),
		Warnings:                 warningCodes,
	}
}

func actorOrSystem(actorID string) string {
	actorID = strings.TrimSpace(actorID)
	if actorID == "" {
		return "system"
	}
	return actorID
}

func normalizeExtension(originalFileName string) string {
	ext := strings.ToLower(strings.TrimSpace(filepath.Ext(originalFileName)))
	if ext == "." {
		return ""
	}
	return ext
}

func normalizeMime(mimeType string) string {
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	if idx := strings.Index(mimeType, ";"); idx >= 0 {
		mimeType = strings.TrimSpace(mimeType[:idx])
	}
	return mimeType
}

func extensionAllowedFor(fileType AssetFileType, ext string) bool {
	for _, allowed := range extensionsFor(fileType) {
		if ext == allowed {
			return true
		}
	}
	return false
}

func extensionsFor(fileType AssetFileType) []string {
	switch fileType {
	case AssetFileTypePDF:
		return []string{".pdf"}
	case AssetFileTypePPTX:
		return []string{".pptx"}
	case AssetFileTypeVideo:
		return []string{".mp4", ".m4v", ".mov", ".webm", ".avi", ".mkv"}
	case AssetFileTypeAudio:
		return []string{".mp3", ".wav", ".m4a", ".aac", ".ogg", ".oga"}
	case AssetFileTypeHTML5:
		return []string{".html", ".htm", ".zip"}
	case AssetFileTypeLink:
		return []string{".url", ".webloc", ".html", ".htm"}
	case AssetFileTypeQuiz:
		return []string{".json", ".quiz", ".qti", ".zip"}
	case AssetFileTypeZIP:
		return []string{".zip"}
	case AssetFileTypeDOCX:
		return []string{".docx"}
	case AssetFileTypeXLSX:
		return []string{".xlsx"}
	case AssetFileTypeImage:
		return []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg"}
	default:
		return nil
	}
}

func mimeAllowedFor(fileType AssetFileType, mimeType string) bool {
	for _, allowed := range exactMimesFor(fileType) {
		if mimeType == allowed {
			return true
		}
	}
	switch fileType {
	case AssetFileTypeVideo:
		return strings.HasPrefix(mimeType, "video/")
	case AssetFileTypeAudio:
		return strings.HasPrefix(mimeType, "audio/")
	case AssetFileTypeImage:
		return strings.HasPrefix(mimeType, "image/")
	default:
		return false
	}
}

func exactMimesFor(fileType AssetFileType) []string {
	switch fileType {
	case AssetFileTypePDF:
		return []string{"application/pdf"}
	case AssetFileTypePPTX:
		return []string{"application/vnd.openxmlformats-officedocument.presentationml.presentation"}
	case AssetFileTypeHTML5:
		return []string{"text/html", "application/xhtml+xml", "application/zip"}
	case AssetFileTypeLink:
		return []string{"text/uri-list", "text/html"}
	case AssetFileTypeQuiz:
		return []string{"application/json", "application/zip", "application/xml", "text/xml"}
	case AssetFileTypeZIP:
		return []string{"application/zip", "application/x-zip-compressed"}
	case AssetFileTypeDOCX:
		return []string{"application/vnd.openxmlformats-officedocument.wordprocessingml.document"}
	case AssetFileTypeXLSX:
		return []string{"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"}
	default:
		return nil
	}
}

func isSoftMimeMismatch(fileType AssetFileType, ext, mimeType string) bool {
	if ext == "" || !extensionAllowedFor(fileType, ext) {
		return false
	}
	if mimeType == "application/octet-stream" || mimeType == "binary/octet-stream" {
		return true
	}
	if mimeType == "application/zip" {
		return fileType == AssetFileTypePPTX || fileType == AssetFileTypeDOCX || fileType == AssetFileTypeXLSX
	}
	if mimeType == "text/plain" {
		return fileType == AssetFileTypeLink && (ext == ".url" || ext == ".webloc")
	}
	return false
}
