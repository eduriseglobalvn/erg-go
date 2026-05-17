package storage

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func TestValidateUploadImageRejectsMismatchedExtension(t *testing.T) {
	buf := testPNG(t)
	_, err := ValidateUpload(buf, "../avatar.jpg", "image/jpeg", UploadKindImage, MaxImageSize)
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("expected mismatch error, got %v", err)
	}
}

func TestValidateUploadImageGeneratesSafeObjectName(t *testing.T) {
	buf := testPNG(t)
	got, err := ValidateUpload(buf, "../avatar.png", "image/png", UploadKindImage, MaxImageSize)
	if err != nil {
		t.Fatalf("ValidateUpload() error = %v", err)
	}
	if got.ContentType != "image/png" {
		t.Fatalf("ContentType = %q", got.ContentType)
	}
	if !strings.HasSuffix(got.ObjectFilename, ".png") || strings.Contains(got.ObjectFilename, "avatar") {
		t.Fatalf("unsafe object filename = %q", got.ObjectFilename)
	}
	if got.OriginalFilename != "avatar.png" {
		t.Fatalf("OriginalFilename = %q", got.OriginalFilename)
	}
}

func TestValidateUploadPDF(t *testing.T) {
	buf := []byte("%PDF-1.7\n%test")
	got, err := ValidateUpload(buf, "cv.pdf", "application/pdf", UploadKindDocument, MaxDocSize)
	if err != nil {
		t.Fatalf("ValidateUpload() error = %v", err)
	}
	if got.ContentType != "application/pdf" || !strings.HasSuffix(got.ObjectFilename, ".pdf") {
		t.Fatalf("unexpected validation result: %#v", got)
	}
}

func TestSanitizeFilename(t *testing.T) {
	if got := SanitizeFilename(`..\..\evil<script>.pdf`); got != "evil-script-.pdf" {
		t.Fatalf("SanitizeFilename() = %q", got)
	}
}

func testPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}
	return buf.Bytes()
}
