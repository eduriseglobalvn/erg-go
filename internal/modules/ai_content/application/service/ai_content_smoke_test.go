//go:build smoke

package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"erg.ninja/pkg/ai"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/storage"
)

func TestSmokeAIContentGeneratesPostAndImage(t *testing.T) {
	t.Setenv("ERG_PROFILE", "")

	var cfg config.Config
	if err := config.NewLoader(config.WithConfigPaths("../../../config")).Load(&cfg); err != nil {
		t.Fatalf("load config: %v", err)
	}
	if strings.TrimSpace(cfg.Ai.GroqAPIKey) == "" && strings.TrimSpace(cfg.Ai.GeminiAPIKey) == "" {
		t.Fatal("AI text provider key is not configured")
	}
	if strings.TrimSpace(cfg.Ai.HuggingFaceImageAPIKey) == "" {
		t.Fatal("Hugging Face image token is not configured")
	}
	if cfg.R2.BucketName == "" || cfg.R2.Endpoint == "" || cfg.R2.AccessKeyID == "" || cfg.R2.SecretKey == "" {
		t.Fatal("R2 is not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	log := logger.NoOp()
	textClient, err := ai.NewClient(cfg.Ai, ai.WithGeminiLogger(log))
	if err != nil {
		t.Fatalf("create AI text client: %v", err)
	}
	r2, err := storage.NewR2Client(ctx, storage.R2Config{
		BucketName:   cfg.R2.BucketName,
		Endpoint:     cfg.R2.Endpoint,
		AccessKeyID:  cfg.R2.AccessKeyID,
		SecretKey:    cfg.R2.SecretKey,
		PublicDomain: cfg.R2.PublicDomain,
		Region:       cfg.R2.Region,
	}, storage.WithR2Logger(log))
	if err != nil {
		t.Fatalf("create R2 client: %v", err)
	}

	imageClient := newHuggingFaceImageClient(cfg.Ai, log)
	if !imageClient.IsConfigured() {
		t.Fatal("Hugging Face image client is not configured")
	}

	svc := NewService(nil, nil, log, textClient, r2, imageClient, nil)
	payload := JobPayload{
		Topic:      "Loi ich cua viec hoc tin hoc quoc te cho hoc sinh trung hoc",
		CategoryID: "smoke-category",
		UserID:     "smoke-test",
	}

	raw, err := textClient.GenerateText(ctx, buildGeneratePostPrompt(payload))
	if err != nil {
		t.Fatalf("generate AI content: %v", err)
	}
	result := normalizeGeneratedPost(raw, payload.Topic, payload.CategoryID)
	if err := svc.ensureGeneratedPostThumbnail(ctx, result, payload); err != nil {
		t.Fatalf("generate and upload thumbnail: %v", err)
	}

	title, _ := result["title"].(string)
	content, _ := result["content"].(string)
	thumbnailURL, _ := result["thumbnailUrl"].(string)
	thumbnailPrompt, _ := result["thumbnailPrompt"].(string)

	if strings.TrimSpace(title) == "" {
		t.Fatal("generated title is empty")
	}
	if !strings.Contains(strings.ToLower(content), "<") {
		t.Fatalf("generated content does not look like HTML: %.120s", content)
	}
	if !strings.HasPrefix(thumbnailURL, "http") {
		t.Fatalf("thumbnailUrl was not uploaded to R2: %q", thumbnailURL)
	}
	if strings.TrimSpace(thumbnailPrompt) == "" {
		t.Fatal("thumbnailPrompt is empty")
	}

	t.Logf("AI_CONTENT_SMOKE title=%q content_chars=%d thumbnail_url=%s thumbnail_prompt_chars=%d", title, len(content), thumbnailURL, len(thumbnailPrompt))
}
