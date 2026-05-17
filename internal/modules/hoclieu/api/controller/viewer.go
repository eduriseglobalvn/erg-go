package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	. "erg.ninja/internal/modules/hoclieu/api/dto"
)

var errUnsatisfiableRange = errors.New("byte range is not satisfiable")

type byteRange struct {
	start int64
	end   int64
}

func writeAssetStream(ctx *gin.Context, asset *AssetDTO) {
	if isHTTPURL(asset.StorageURL) {
		setDeliveryHeaders(ctx, asset, "inline")
		ctx.Header("X-Hoclieu-Storage-Contract", "r2-url")
		ctx.Redirect(http.StatusTemporaryRedirect, asset.StorageURL)
		return
	}
	payload := sampleBytesFor(asset.SelectedFileType)
	size := int64(len(payload))
	contentType := contentTypeFor(asset.SelectedFileType)

	setDeliveryHeaders(ctx, asset, "inline")
	ctx.Header("Accept-Ranges", "bytes")
	ctx.Header("X-Hoclieu-Storage-Contract", "placeholder")

	rangeHeader := strings.TrimSpace(ctx.GetHeader("Range"))
	if rangeHeader == "" {
		ctx.Header("Content-Length", strconv.FormatInt(size, 10))
		ctx.Data(http.StatusOK, contentType, payload)
		return
	}

	rng, err := parseSingleByteRange(rangeHeader, size)
	if err != nil {
		ctx.Header("Content-Range", fmt.Sprintf("bytes */%d", size))
		ctx.Status(http.StatusRequestedRangeNotSatisfiable)
		return
	}

	part := payload[int(rng.start) : int(rng.end)+1]
	ctx.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", rng.start, rng.end, size))
	ctx.Header("Content-Length", strconv.FormatInt(int64(len(part)), 10))
	ctx.Data(http.StatusPartialContent, contentType, part)
}

func writeAssetDownload(ctx *gin.Context, asset *AssetDTO) {
	if isHTTPURL(asset.StorageURL) {
		setDeliveryHeaders(ctx, asset, "attachment")
		ctx.Header("X-Hoclieu-Storage-Contract", "r2-url")
		ctx.Redirect(http.StatusTemporaryRedirect, asset.StorageURL)
		return
	}
	payload := sampleBytesFor(asset.SelectedFileType)
	setDeliveryHeaders(ctx, asset, "attachment")
	ctx.Header("X-Hoclieu-Storage-Contract", "placeholder")
	ctx.Header("Content-Length", strconv.Itoa(len(payload)))
	ctx.Data(http.StatusOK, contentTypeFor(asset.SelectedFileType), payload)
}

func writeAuditHeaders(ctx *gin.Context, event string, asset *AssetDTO, userID string) {
	ctx.Header("X-Hoclieu-Audit-Event", event)
	ctx.Header("X-Hoclieu-Audit-Asset-Id", asset.ID)
	if userID != "" {
		ctx.Header("X-Hoclieu-Audit-User-Id", userID)
	}
	ctx.Header("X-Hoclieu-Audit-At", time.Now().UTC().Format(time.RFC3339))
}

func setDeliveryHeaders(ctx *gin.Context, asset *AssetDTO, disposition string) {
	ctx.Header("Cache-Control", "no-store")
	ctx.Header("Pragma", "no-cache")
	ctx.Header("X-Content-Type-Options", "nosniff")
	ctx.Header("X-Hoclieu-Can-Download", strconv.FormatBool(asset.CanDownload))
	ctx.Header("Content-Disposition", fmt.Sprintf(`%s; filename="%s"`, disposition, safeFilename(asset)))
}

func parseSingleByteRange(header string, size int64) (byteRange, error) {
	if size <= 0 {
		return byteRange{}, errUnsatisfiableRange
	}
	if !strings.HasPrefix(header, "bytes=") {
		return byteRange{}, errUnsatisfiableRange
	}
	spec := strings.TrimSpace(strings.TrimPrefix(header, "bytes="))
	if spec == "" || strings.Contains(spec, ",") {
		return byteRange{}, errUnsatisfiableRange
	}
	parts := strings.SplitN(spec, "-", 2)
	if len(parts) != 2 {
		return byteRange{}, errUnsatisfiableRange
	}

	if parts[0] == "" {
		suffix, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || suffix <= 0 {
			return byteRange{}, errUnsatisfiableRange
		}
		if suffix > size {
			suffix = size
		}
		return byteRange{start: size - suffix, end: size - 1}, nil
	}

	start, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || start < 0 || start >= size {
		return byteRange{}, errUnsatisfiableRange
	}
	end := size - 1
	if parts[1] != "" {
		end, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return byteRange{}, errUnsatisfiableRange
		}
	}
	if end < start {
		return byteRange{}, errUnsatisfiableRange
	}
	if end >= size {
		end = size - 1
	}
	return byteRange{start: start, end: end}, nil
}

func contentTypeFor(fileType AssetFileType) string {
	switch fileType {
	case AssetFileTypePDF:
		return "application/pdf"
	case AssetFileTypePPTX:
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case AssetFileTypeVideo:
		return "video/mp4"
	case AssetFileTypeAudio:
		return "audio/mpeg"
	case AssetFileTypeHTML5:
		return "text/html; charset=utf-8"
	case AssetFileTypeDOCX:
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case AssetFileTypeXLSX:
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case AssetFileTypeZIP:
		return "application/zip"
	case AssetFileTypeImage:
		return "image/png"
	default:
		return "application/octet-stream"
	}
}

func sampleBytesFor(fileType AssetFileType) []byte {
	if fileType == AssetFileTypePDF {
		return []byte("%PDF-1.4\n1 0 obj << /Type /Catalog /Pages 2 0 R >> endobj\n2 0 obj << /Type /Pages /Kids [3 0 R] /Count 1 >> endobj\n3 0 obj << /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R >> endobj\n4 0 obj << /Length 44 >> stream\nBT /F1 20 Tf 72 720 Td (ERG Hoclieu PDF) Tj ET\nendstream endobj\nxref\n0 5\n0000000000 65535 f \n0000000009 00000 n \n0000000058 00000 n \n0000000115 00000 n \n0000000211 00000 n \ntrailer << /Root 1 0 R /Size 5 >>\nstartxref\n304\n%%EOF\n")
	}

	label := "ERG hoclieu protected asset placeholder\n"
	switch fileType {
	case AssetFileTypeVideo:
		label = "ERG hoclieu protected video placeholder\n"
	case AssetFileTypeAudio:
		label = "ERG hoclieu protected audio placeholder\n"
	case AssetFileTypePPTX:
		label = "ERG hoclieu protected slide placeholder\n"
	}
	return []byte(strings.Repeat(label, 16))
}

func safeFilename(asset *AssetDTO) string {
	name := strings.TrimSpace(asset.OriginalFileName)
	if name == "" {
		name = asset.ID + "." + extensionFor(asset.SelectedFileType)
	}
	replacer := strings.NewReplacer(`\`, "", `"`, "", "\r", "", "\n", "")
	return replacer.Replace(name)
}

func isHTTPURL(value string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), "http://") ||
		strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), "https://")
}

func extensionFor(fileType AssetFileType) string {
	switch fileType {
	case AssetFileTypePDF:
		return "pdf"
	case AssetFileTypePPTX:
		return "pptx"
	case AssetFileTypeXLSX:
		return "xlsx"
	case AssetFileTypeZIP:
		return "zip"
	case AssetFileTypeVideo:
		return "mp4"
	case AssetFileTypeAudio:
		return "mp3"
	case AssetFileTypeLink:
		return "url"
	default:
		return "bin"
	}
}
