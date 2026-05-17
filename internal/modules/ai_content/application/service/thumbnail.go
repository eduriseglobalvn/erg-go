package service

import (
	"context"
	"fmt"
	"os"
	"strings"
)

func (s *Service) generateAndUploadThumbnail(ctx context.Context, title, excerpt, prompt, categoryID string) (string, error) {
	if s.image == nil || !s.image.IsConfigured() {
		return "", fmt.Errorf("ai thumbnail: Hugging Face image generation is not configured")
	}
	return s.generateAndUploadHuggingFaceThumbnail(ctx, title, excerpt, prompt)
}

func (s *Service) generateAndUploadHuggingFaceThumbnail(ctx context.Context, title, excerpt, prompt string) (string, error) {
	localPath, mime, err := s.image.GenerateToTempFile(ctx, buildImagePrompt(title, excerpt, prompt))
	if err != nil {
		return "", err
	}
	defer removeTempFile(localPath)

	buf, err := os.ReadFile(localPath) // #nosec G304 -- path is created by os.CreateTemp in GenerateToTempFile.
	if err != nil {
		return "", fmt.Errorf("ai thumbnail: read Hugging Face temp image: %w", err)
	}
	filename := generatedImageFilename("ai-hf-thumbnail", mime)
	url, err := s.r2.UploadRaw(ctx, buf, "posts/images", filename, mime)
	if err != nil {
		return "", fmt.Errorf("ai thumbnail: upload Hugging Face image to R2: %w", err)
	}
	return url, nil
}

func buildImagePrompt(title, excerpt, prompt string) string {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		prompt = strings.TrimSpace(title)
		if prompt == "" {
			prompt = "modern education, students learning with technology"
		}
		if excerpt = strings.TrimSpace(excerpt); excerpt != "" {
			prompt += ". Context: " + trimRunes(excerpt, 180)
		}
	}
	return strings.TrimSpace(prompt + ". Professional editorial education article thumbnail, 16:9 composition, clean modern lighting, Vietnamese education brand feeling, no readable text, no watermark.")
}
