package ai_content

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hibiken/asynq"

	"erg.ninja/pkg/queue"
)

// HandleGeneratePost executes the logic for AI post generation in the background.
func (s *Service) HandleGeneratePost(ctx context.Context, task *asynq.Task) error {
	var payload JobPayload
	if err := queue.ParsePayload(task.Payload(), &payload); err != nil {
		return fmt.Errorf("ai_content worker: invalid payload: %w", err)
	}

	s.log.Info().
		Str("topic", payload.Topic).
		Str("userId", payload.UserID).
		Msg("Background worker started generating AI post")

	// 1. Fetch active Gemini API key from DB
	key, err := s.repo.GetActiveKey(ctx, ProviderGemini)
	if err != nil || key == nil {
		s.log.Error().Err(err).Msg("ai_content worker: No active API key found for text generation")
		return fmt.Errorf("no active API key found for text generation")
	}

	// 2. Mock call to LLM
	// Here you would integrate with go-gemini SDK or do a raw HTTP request.
	s.log.Info().Msg("Generating content using Gemini API...")

	// Simulate AI generation time
	// time.Sleep(3 * time.Second)

	title := strings.TrimSpace(payload.Topic)
	if title == "" {
		title = "Bai viet moi"
	}

	generatedContent := fmt.Sprintf(
		"<h1>%s</h1><p>Nội dung bài viết tự động sinh cho chủ đề: %s.</p><p>Ban co the bien tap, bo sung hinh anh va toi uu SEO truoc khi dang.</p>",
		title,
		payload.Topic,
	)

	// 3. Update task result so `GetJobStatus` can read it
	resBytes, _ := json.Marshal(map[string]any{
		"title":      title,
		"content":    generatedContent,
		"excerpt":    fmt.Sprintf("Noi dung AI nhap cho chu de: %s", payload.Topic),
		"categoryId": payload.CategoryID,
		"slug":       "",
	})

	// You can set task result natively in asynq returning a non-nil result.
	task.ResultWriter().Write(resBytes)

	return nil
}
