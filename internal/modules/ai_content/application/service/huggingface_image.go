package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"erg.ninja/pkg/config"
	"erg.ninja/pkg/logger"
)

const maxGeneratedImageBytes = 30 << 20

// HuggingFaceImageClient generates images through Hugging Face text-to-image inference.
type HuggingFaceImageClient struct {
	apiKey         string
	provider       string
	model          string
	baseURL        string
	timeout        time.Duration
	width          int
	height         int
	steps          int
	guidance       float64
	negativePrompt string
	httpClient     *http.Client
	log            *logger.Logger
}

func NewHuggingFaceImageClient(cfg config.AiConfig, log *logger.Logger) *HuggingFaceImageClient {
	timeout := cfg.HuggingFaceImageTimeout
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.HuggingFaceImageBaseURL), "/")
	if baseURL == "" {
		baseURL = "https://router.huggingface.co"
	}
	provider := strings.ToLower(strings.TrimSpace(cfg.HuggingFaceImageProvider))
	if provider == "" {
		provider = "fal-ai"
	}
	model := strings.TrimSpace(cfg.HuggingFaceImageModel)
	if model == "" {
		model = "black-forest-labs/FLUX.1-Krea-dev"
	}
	width := cfg.HuggingFaceImageWidth
	if width <= 0 {
		width = 1024
	}
	height := cfg.HuggingFaceImageHeight
	if height <= 0 {
		height = 576
	}
	steps := cfg.HuggingFaceImageSteps
	if steps <= 0 {
		steps = 28
	}
	guidance := cfg.HuggingFaceImageGuidance
	if guidance <= 0 {
		guidance = 7
	}
	negativePrompt := strings.TrimSpace(cfg.HuggingFaceImageNegativePrompt)
	if negativePrompt == "" {
		negativePrompt = "text, watermark, logo, blurry, distorted, low quality, extra fingers"
	}
	if log == nil {
		log = logger.NoOp()
	}
	return &HuggingFaceImageClient{
		apiKey:         strings.TrimSpace(cfg.HuggingFaceImageAPIKey),
		provider:       provider,
		model:          model,
		baseURL:        baseURL,
		timeout:        timeout,
		width:          width,
		height:         height,
		steps:          steps,
		guidance:       guidance,
		negativePrompt: negativePrompt,
		httpClient:     &http.Client{Timeout: timeout + 10*time.Second},
		log:            log,
	}
}

func newHuggingFaceImageClient(cfg config.AiConfig, log *logger.Logger) *HuggingFaceImageClient {
	return NewHuggingFaceImageClient(cfg, log)
}

func (c *HuggingFaceImageClient) IsConfigured() bool {
	return c != nil && strings.TrimSpace(c.apiKey) != "" && strings.TrimSpace(c.model) != ""
}

func (c *HuggingFaceImageClient) Model() string {
	if c == nil {
		return ""
	}
	return c.model
}

func (c *HuggingFaceImageClient) GenerateToTempFile(ctx context.Context, prompt string) (string, string, error) {
	if !c.IsConfigured() {
		return "", "", fmt.Errorf("huggingface image: token or model not configured")
	}
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return "", "", fmt.Errorf("huggingface image: prompt is required")
	}

	reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	body, err := json.Marshal(c.requestPayload(prompt))
	if err != nil {
		return "", "", fmt.Errorf("huggingface image: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, c.modelURL(), bytes.NewReader(body))
	if err != nil {
		return "", "", fmt.Errorf("huggingface image: new request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "image/png,image/jpeg,image/webp")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("huggingface image: request: %w", err)
	}
	defer resp.Body.Close()

	contentType := strings.ToLower(strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0]))
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", "", fmt.Errorf("huggingface image: status %d: %s", resp.StatusCode, trimString(string(raw), 300))
	}
	if strings.HasPrefix(contentType, "image/") {
		return writeTempImage(resp.Body, contentType)
	}

	if strings.Contains(contentType, "json") {
		return c.writeProviderJSONImageToTempFile(ctx, resp.Body)
	}

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return "", "", fmt.Errorf("huggingface image: expected image or provider JSON response, got %q: %s", contentType, trimString(string(raw), 300))
}

func writeTempImage(reader io.Reader, contentType string) (string, string, error) {
	ext := extensionForContentType(contentType)
	tmp, err := os.CreateTemp("", "erg-ai-image-*"+ext)
	if err != nil {
		return "", "", fmt.Errorf("huggingface image: create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanupOnError := true
	defer func() {
		_ = tmp.Close()
		if cleanupOnError {
			_ = os.Remove(tmpPath)
		}
	}()

	written, err := io.Copy(tmp, io.LimitReader(reader, maxGeneratedImageBytes+1))
	if err != nil {
		return "", "", fmt.Errorf("huggingface image: download temp file: %w", err)
	}
	if written > maxGeneratedImageBytes {
		return "", "", fmt.Errorf("huggingface image: generated image exceeds %d bytes", maxGeneratedImageBytes)
	}
	if err := tmp.Close(); err != nil {
		return "", "", fmt.Errorf("huggingface image: close temp file: %w", err)
	}

	cleanupOnError = false
	return tmpPath, contentType, nil
}

func (c *HuggingFaceImageClient) modelURL() string {
	if c.provider == "fal-ai" {
		return c.baseURL + "/fal-ai/" + falProviderModelID(c.model)
	}
	if c.provider == "hf-inference" {
		return c.baseURL + "/hf-inference/models/" + encodePathSegments(c.model)
	}
	if strings.Contains(c.baseURL, "api-inference.huggingface.co") {
		return strings.TrimRight(c.baseURL, "/") + "/" + encodePathSegments(c.model)
	}
	return c.baseURL + "/" + encodePathSegments(c.model)
}

func (c *HuggingFaceImageClient) requestPayload(prompt string) map[string]any {
	if c.provider == "fal-ai" {
		return map[string]any{
			"prompt":              prompt,
			"negative_prompt":     c.negativePrompt,
			"num_inference_steps": c.steps,
			"guidance_scale":      c.guidance,
			"image_size": map[string]any{
				"width":  c.width,
				"height": c.height,
			},
		}
	}
	return map[string]any{
		"inputs": prompt,
		"parameters": map[string]any{
			"negative_prompt":     c.negativePrompt,
			"num_inference_steps": c.steps,
			"guidance_scale":      c.guidance,
			"width":               c.width,
			"height":              c.height,
		},
	}
}

func (c *HuggingFaceImageClient) writeProviderJSONImageToTempFile(ctx context.Context, reader io.Reader) (string, string, error) {
	var payload struct {
		Images []struct {
			URL         string `json:"url"`
			ContentType string `json:"content_type"`
		} `json:"images"`
		Output any `json:"output"`
	}
	if err := json.NewDecoder(io.LimitReader(reader, 1<<20)).Decode(&payload); err != nil {
		return "", "", fmt.Errorf("huggingface image: decode provider response: %w", err)
	}

	imageURL := ""
	contentType := ""
	if len(payload.Images) > 0 {
		imageURL = strings.TrimSpace(payload.Images[0].URL)
		contentType = strings.TrimSpace(payload.Images[0].ContentType)
	}
	if imageURL == "" {
		switch output := payload.Output.(type) {
		case string:
			imageURL = strings.TrimSpace(output)
		case []any:
			if len(output) > 0 {
				if first, ok := output[0].(string); ok {
					imageURL = strings.TrimSpace(first)
				}
			}
		}
	}
	if imageURL == "" {
		return "", "", fmt.Errorf("huggingface image: provider response does not contain an image URL")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("huggingface image: new provider image download request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("huggingface image: download provider image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", "", fmt.Errorf("huggingface image: provider image status %d: %s", resp.StatusCode, trimString(string(raw), 300))
	}
	respContentType := strings.ToLower(strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0]))
	if respContentType != "" {
		contentType = respContentType
	}
	if contentType == "" {
		contentType = "image/png"
	}
	return writeTempImage(resp.Body, contentType)
}

func falProviderModelID(model string) string {
	model = strings.Trim(model, "/")
	switch strings.ToLower(model) {
	case "black-forest-labs/flux.1-krea-dev":
		return "fal-ai/flux/krea"
	case "black-forest-labs/flux.1-dev":
		return "fal-ai/flux/dev"
	case "black-forest-labs/flux.1-schnell":
		return "fal-ai/flux/schnell"
	default:
		if strings.HasPrefix(model, "fal-ai/") {
			return encodePathSegments(model)
		}
		return encodePathSegments(model)
	}
}

func encodePathSegments(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	encoded := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		encoded = append(encoded, url.PathEscape(part))
	}
	return strings.Join(encoded, "/")
}

func removeTempFile(path string) {
	if strings.TrimSpace(path) == "" {
		return
	}
	_ = os.Remove(path)
}

func generatedImageFilename(prefix, mime string) string {
	ext := extensionForContentType(mime)
	if ext == "" {
		ext = ".png"
	}
	return fmt.Sprintf("%s-%d%s", prefix, time.Now().UTC().UnixNano(), ext)
}

func extensionForContentType(mime string) string {
	switch strings.ToLower(strings.TrimSpace(mime)) {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/png":
		return ".png"
	default:
		if strings.HasPrefix(mime, "image/") {
			ext := "." + strings.TrimPrefix(mime, "image/")
			return filepath.Clean(ext)
		}
		return ""
	}
}

func trimString(value string, max int) string {
	value = strings.TrimSpace(value)
	if len(value) <= max {
		return value
	}
	return value[:max]
}
