// Package ai provides an AI client for CSS selector suggestions and content analysis.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/logger"
)

// Client wraps the Gemini API for CSS selector suggestions and content analysis.
type Client struct {
	apiKey   string
	model    string
	timeout  time.Duration
	log      *logger.Logger
	redis    *cache.RedisClient
	cacheTTL time.Duration
	client   *http.Client
	mu       sync.RWMutex
	inMemory map[string]*cachedResult
}

// ClientOption configures a Client.
type ClientOption func(*Client)

// WithGeminiLogger sets the logger for the AI client.
func WithGeminiLogger(log *logger.Logger) ClientOption {
	return func(c *Client) {
		c.log = log
	}
}

// WithRedisCache attaches a Redis client for distributed caching.
func WithRedisCache(redis *cache.RedisClient) ClientOption {
	return func(c *Client) {
		c.redis = redis
	}
}

// cachedResult holds a cached Gemini response.
type cachedResult struct {
	Response string
	CachedAt time.Time
}

// NewClient creates a new Gemini AI client from configuration.
func NewClient(cfg config.AiConfig, opts ...ClientOption) (*Client, error) {
	c := &Client{
		apiKey:   cfg.GeminiAPIKey,
		model:    cfg.GeminiModel,
		timeout:  cfg.GeminiTimeout,
		cacheTTL: cfg.CacheTTL,
		log:      logger.NoOp(),
		inMemory: make(map[string]*cachedResult),
		client: &http.Client{
			Timeout: cfg.GeminiTimeout + 5*time.Second,
		},
	}
	for _, o := range opts {
		o(c)
	}

	if c.model == "" {
		c.model = "gemini-2.0-flash"
	}
	if c.cacheTTL == 0 {
		c.cacheTTL = 24 * time.Hour
	}

	return c, nil
}

// SuggestCSSSelectors asks the Gemini model to suggest CSS selectors for extracting
// content from an unstructured HTML page based on a content description.
func (c *Client) SuggestCSSSelectors(ctx context.Context, htmlContent, contentDescription string) ([]string, error) {
	// Check cache first.
	cacheKey := cacheKeyFor("css-suggest", contentDescription, c.model)
	if cached := c.getCached(ctx, cacheKey); cached != "" {
		c.log.DebugContext(ctx).Str("cache", "hit").Str("key", cacheKey).Msg("gemini: returning cached suggestion")
		return parseSelectors(cached)
	}

	// Build the prompt.
	prompt := fmt.Sprintf(`Given the following HTML page description and content, suggest CSS selectors to extract the described content.
Only return a JSON array of CSS selector strings. No explanation.

Content description: %s

HTML preview (first 2000 chars):
%s

Return format: ["selector1", "selector2", ...]`, contentDescription, string([]byte(htmlContent)[:min(len(htmlContent), 2000)]))

	response, err := c.generateContent(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("ai: SuggestCSSSelectors: %w", err)
	}

	// Cache the response.
	c.setCached(ctx, cacheKey, response)

	selectors, err := parseSelectors(response)
	if err != nil {
		return nil, fmt.Errorf("ai: parse selectors: %w", err)
	}
	return selectors, nil
}

// AnalyzeContent sends content to Gemini for analysis (summarization, classification, etc.).
func (c *Client) AnalyzeContent(ctx context.Context, content, analysisType string) (string, error) {
	var prompt string
	switch analysisType {
	case "summarize":
		prompt = fmt.Sprintf("Summarize the following content in 2-3 sentences:\n\n%s", content)
	case "classify":
		prompt = fmt.Sprintf("Classify the following content into one of these categories: news, blog, forum, e-commerce, other. Return only the category name:\n\n%s", content)
	case "extract_topics":
		prompt = fmt.Sprintf("Extract 3-5 main topics from the following content as a JSON array of strings:\n\n%s", content)
	default:
		prompt = content
	}

	return c.generateContent(ctx, prompt)
}

// generateContent sends a request to the Gemini API and returns the response text.
func (c *Client) generateContent(ctx context.Context, prompt string) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("ai: Gemini API key not configured (set gemini_api_key)")
	}

	apiURL := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		c.model, c.apiKey)

	requestBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature":     0.7,
			"maxOutputTokens": 2048,
			"topP":            0.9,
		},
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("ai: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("ai: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ai: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ai: Gemini API returned status %d", resp.StatusCode)
	}

	var apiResp geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return "", fmt.Errorf("ai: decode response: %w", err)
	}

	if len(apiResp.Candidates) == 0 {
		return "", fmt.Errorf("ai: Gemini returned no candidates")
	}

	text := apiResp.Candidates[0].Content.Parts[0].Text
	return text, nil
}

// geminiResponse represents the Gemini API JSON response structure.
type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

// getCached retrieves a cached response from Redis or in-memory.
func (c *Client) getCached(ctx context.Context, key string) string {
	// Check in-memory first.
	c.mu.RLock()
	if r, ok := c.inMemory[key]; ok {
		if time.Since(r.CachedAt) < c.cacheTTL {
			c.mu.RUnlock()
			return r.Response
		}
	}
	c.mu.RUnlock()

	// Check Redis if configured.
	if c.redis != nil {
		val, err := c.redis.Get(ctx, "ai:cache:"+key)
		if err == nil && val != "" {
			// Also populate in-memory cache.
			c.mu.Lock()
			c.inMemory[key] = &cachedResult{Response: val, CachedAt: time.Now()}
			c.mu.Unlock()
			return val
		}
	}

	return ""
}

// setCached stores a response in Redis and in-memory.
func (c *Client) setCached(ctx context.Context, key, response string) {
	// Store in-memory.
	c.mu.Lock()
	c.inMemory[key] = &cachedResult{Response: response, CachedAt: time.Now()}
	c.mu.Unlock()

	// Store in Redis if configured.
	if c.redis != nil {
		if err := c.redis.Set(ctx, "ai:cache:"+key, response, c.cacheTTL); err != nil {
			c.log.WarnContext(ctx).Err(err).Str("key", key).Msg("gemini: cache write failed")
		}
	}
}

// cacheKeyFor generates a cache key for a prompt and model combination.
func cacheKeyFor(purpose, content, model string) string {
	// Use a hash of the content as the key to keep it short.
	// For simplicity, use first 100 chars + purpose + model.
	short := content
	if len(short) > 100 {
		short = short[:100]
	}
	return fmt.Sprintf("%s:%s:%s", purpose, model, short)
}

// parseSelectors extracts CSS selectors from a JSON array string.
func parseSelectors(response string) ([]string, error) {
	// Try to parse as JSON array first.
	var selectors []string
	if err := json.Unmarshal([]byte(response), &selectors); err == nil {
		return selectors, nil
	}

	// Try to extract from markdown code blocks.
	response = trimBackticks(response)
	if err := json.Unmarshal([]byte(response), &selectors); err == nil {
		return selectors, nil
	}

	return nil, fmt.Errorf("ai: cannot parse selectors from response: %s", trimString(response, 200))
}

// trimBackticks removes triple backticks and optional "json"/"js" language label.
func trimBackticks(s string) string {
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```JSON")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

func trimString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
