package storage

import (
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

type UploadKind string

const (
	UploadKindImage    UploadKind = "image"
	UploadKindDocument UploadKind = "document"
)

const MultipartMemoryLimit int64 = 1 << 20

type ValidatedUpload struct {
	OriginalFilename string
	ObjectFilename   string
	ContentType      string
	Size             int64
}

var safeFilenameChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

var imageExtByMIME = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

var docExtByMIME = map[string]string{
	"application/pdf":    ".pdf",
	"application/msword": ".doc",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": ".docx",
}

func MaxRequestBytes(kind UploadKind, maxPayload int64) int64 {
	if maxPayload <= 0 {
		switch kind {
		case UploadKindImage:
			maxPayload = MaxImageSize
		case UploadKindDocument:
			maxPayload = MaxDocSize
		}
	}
	return maxPayload + MultipartMemoryLimit
}

func ValidateUpload(buf []byte, originalFilename string, declaredContentType string, kind UploadKind, maxSize int64) (ValidatedUpload, error) {
	if len(buf) == 0 {
		return ValidatedUpload{}, fmt.Errorf("upload: empty file")
	}
	if maxSize <= 0 {
		switch kind {
		case UploadKindImage:
			maxSize = MaxImageSize
		case UploadKindDocument:
			maxSize = MaxDocSize
		default:
			return ValidatedUpload{}, fmt.Errorf("upload: unsupported kind %q", kind)
		}
	}
	if int64(len(buf)) > maxSize {
		return ValidatedUpload{}, fmt.Errorf("upload: file exceeds %d bytes", maxSize)
	}

	original := SanitizeFilename(originalFilename)
	ext := strings.ToLower(filepath.Ext(original))
	contentType, canonicalExt, err := validateContentTypeAndExtension(buf, declaredContentType, ext, kind)
	if err != nil {
		return ValidatedUpload{}, err
	}
	if original == "" {
		original = "upload" + canonicalExt
	}
	return ValidatedUpload{
		OriginalFilename: original,
		ObjectFilename:   uuid.NewString() + canonicalExt,
		ContentType:      contentType,
		Size:             int64(len(buf)),
	}, nil
}

func SanitizeFilename(filename string) string {
	filename = strings.TrimSpace(filepath.Base(filename))
	filename = strings.ReplaceAll(filename, "\\", "/")
	filename = filepath.Base(filename)
	filename = safeFilenameChars.ReplaceAllString(filename, "-")
	filename = strings.Trim(filename, ".-_ ")
	if filename == "." || filename == ".." {
		return ""
	}
	if len(filename) > 120 {
		ext := filepath.Ext(filename)
		base := strings.TrimSuffix(filename, ext)
		if len(base) > 100 {
			base = base[:100]
		}
		filename = base + ext
	}
	return filename
}

func SafeFolder(folder string) string {
	folder = strings.ReplaceAll(strings.TrimSpace(folder), "\\", "/")
	parts := strings.Split(folder, "/")
	safe := make([]string, 0, len(parts))
	for _, part := range parts {
		part = SanitizeFilename(part)
		if part == "" {
			continue
		}
		safe = append(safe, part)
	}
	if len(safe) == 0 {
		return "default"
	}
	return strings.Join(safe, "/")
}

func validateContentTypeAndExtension(buf []byte, declared string, ext string, kind UploadKind) (string, string, error) {
	sniffed := strings.ToLower(strings.TrimSpace(strings.Split(http.DetectContentType(buf), ";")[0]))
	declared = strings.ToLower(strings.TrimSpace(strings.Split(declared, ";")[0]))

	switch kind {
	case UploadKindImage:
		canonicalExt, ok := imageExtByMIME[sniffed]
		if !ok {
			return "", "", fmt.Errorf("upload: unsupported image MIME %q", sniffed)
		}
		if ext != "" && ext != canonicalExt && !(sniffed == "image/jpeg" && ext == ".jpeg") {
			return "", "", fmt.Errorf("upload: extension %q does not match image MIME %q", ext, sniffed)
		}
		return sniffed, canonicalExt, nil
	case UploadKindDocument:
		if ext == ".pdf" && sniffed == "application/pdf" {
			return "application/pdf", ".pdf", nil
		}
		if ext == ".docx" && (sniffed == "application/zip" || declared == docxMIME()) {
			return docxMIME(), ".docx", nil
		}
		if ext == ".doc" && declared == "application/msword" {
			return "application/msword", ".doc", nil
		}
		if canonicalExt, ok := docExtByMIME[declared]; ok && ext == canonicalExt {
			return declared, canonicalExt, nil
		}
		return "", "", fmt.Errorf("upload: unsupported or mismatched document type ext=%q sniffed=%q declared=%q", ext, sniffed, declared)
	default:
		return "", "", fmt.Errorf("upload: unsupported kind %q", kind)
	}
}

func docxMIME() string {
	return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
}
