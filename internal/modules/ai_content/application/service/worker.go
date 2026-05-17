package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hibiken/asynq"

	"erg.ninja/pkg/queue"
)

// HandleGeneratePost executes AI post generation in the background.
func (s *Service) HandleGeneratePost(ctx context.Context, task *asynq.Task) error {
	var payload JobPayload
	if err := queue.ParsePayload(task.Payload(), &payload); err != nil {
		return fmt.Errorf("ai_content worker: invalid payload: %w", err)
	}

	s.log.Info().
		Str("topic", payload.Topic).
		Str("userId", payload.UserID).
		Msg("Background worker started generating AI post")

	client, provider, keyID, err := s.activeAIClient(ctx)
	if err != nil {
		s.log.Error().Err(err).Msg("ai_content worker: no active API key found for text generation")
		return err
	}

	s.log.Info().Str("provider", provider).Str("key_id", keyID).Msg("Generating content using AI provider")

	title := strings.TrimSpace(payload.Topic)
	if title == "" {
		title = "Bai viet moi"
	}

	raw, err := client.GenerateText(ctx, buildGeneratePostPrompt(payload))
	if err != nil {
		return fmt.Errorf("ai_content worker: generate content: %w", err)
	}
	s.markKeyUsed(ctx, keyID)
	result := normalizeGeneratedPost(raw, title, payload.CategoryID)
	if err := s.ensureGeneratedPostThumbnail(ctx, result, payload); err != nil {
		return fmt.Errorf("ai_content worker: generate thumbnail: %w", err)
	}

	resBytes, _ := json.Marshal(result)
	_, _ = task.ResultWriter().Write(resBytes)

	return nil
}

func buildGeneratePostPrompt(payload JobPayload) string {
	return fmt.Sprintf(`Generate one polished Vietnamese article for an education company CMS.

Topic: %s
Category ID: %s

Requirements:
- Return valid JSON only, no markdown fences and no commentary.
- JSON shape: {"title":"...","content":"...","excerpt":"...","slug":"...","thumbnailPrompt":"..."}.
- "content" must be clean HTML suitable for a rich text editor.
- "thumbnailPrompt" must be an English prompt for a clean 16:9 educational article thumbnail, no text inside the image.
- Include an H2 introduction, 3-5 useful sections, and a concise conclusion.
- Keep the tone professional, clear, and helpful for students/parents.
- Do not invent unverifiable statistics or legal/medical claims.`, strings.TrimSpace(payload.Topic), strings.TrimSpace(payload.CategoryID))
}

func normalizeGeneratedPost(raw, fallbackTitle, categoryID string) map[string]any {
	type generatedPost struct {
		Title           string `json:"title"`
		Content         string `json:"content"`
		Excerpt         string `json:"excerpt"`
		Slug            string `json:"slug"`
		ThumbnailURL    string `json:"thumbnailUrl"`
		ThumbnailPrompt string `json:"thumbnailPrompt"`
	}

	candidate := strings.TrimSpace(extractJSONObject(raw))
	var parsed generatedPost
	if candidate != "" {
		_ = json.Unmarshal([]byte(candidate), &parsed)
	}

	title := strings.TrimSpace(parsed.Title)
	if title == "" {
		title = fallbackTitle
	}
	content := strings.TrimSpace(parsed.Content)
	if content == "" {
		content = strings.TrimSpace(raw)
	}
	if content == "" {
		content = fmt.Sprintf("<h2>%s</h2><p>Noi dung dang duoc cap nhat.</p>", title)
	}
	excerpt := strings.TrimSpace(parsed.Excerpt)
	if excerpt == "" {
		excerpt = trimRunes(stripHTML(content), 180)
	}

	return map[string]any{
		"title":           title,
		"content":         content,
		"excerpt":         excerpt,
		"categoryId":      categoryID,
		"slug":            strings.TrimSpace(parsed.Slug),
		"thumbnailUrl":    strings.TrimSpace(parsed.ThumbnailURL),
		"thumbnailPrompt": strings.TrimSpace(parsed.ThumbnailPrompt),
	}
}

func (s *Service) ensureGeneratedPostThumbnail(ctx context.Context, result map[string]any, payload JobPayload) error {
	if s == nil {
		return fmt.Errorf("ai thumbnail: service is not configured")
	}
	if s.r2 == nil {
		return fmt.Errorf("ai thumbnail: R2 storage is not configured")
	}
	if result == nil {
		return fmt.Errorf("ai thumbnail: generated post result is empty")
	}
	if rawURL, _ := result["thumbnailUrl"].(string); strings.TrimSpace(rawURL) != "" {
		return nil
	}

	title, _ := result["title"].(string)
	excerpt, _ := result["excerpt"].(string)
	thumbnailPrompt, _ := result["thumbnailPrompt"].(string)
	thumbnailURL, err := s.generateAndUploadThumbnail(ctx, title, excerpt, thumbnailPrompt, payload.CategoryID)
	if err != nil {
		return err
	}
	result["thumbnailUrl"] = thumbnailURL
	return nil
}

func extractJSONObject(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```") {
		raw = strings.TrimPrefix(raw, "```json")
		raw = strings.TrimPrefix(raw, "```JSON")
		raw = strings.TrimPrefix(raw, "```")
		raw = strings.TrimSuffix(raw, "```")
		raw = strings.TrimSpace(raw)
	}
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end < start {
		return ""
	}
	return raw[start : end+1]
}

func stripHTML(input string) string {
	var b strings.Builder
	inTag := false
	for _, r := range input {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				b.WriteRune(r)
			}
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func trimRunes(input string, max int) string {
	runes := []rune(strings.TrimSpace(input))
	if len(runes) <= max {
		return string(runes)
	}
	return string(runes[:max])
}
