package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"erg.ninja/pkg/config"
)

func TestNewClient(t *testing.T) {
	cfg := config.AiConfig{
		GeminiAPIKey:  "test-key",
		GeminiModel:   "gemini-2.0-flash",
		GeminiTimeout: 10 * time.Second,
		CacheTTL:      24 * time.Hour,
		BatchSize:     10,
	}

	c, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c == nil {
		t.Fatal("NewClient returned nil")
	}
	if c.model != "gemini-2.0-flash" {
		t.Errorf("model = %q, want 'gemini-2.0-flash'", c.model)
	}
	if c.Provider() != "gemini" {
		t.Errorf("provider = %q, want gemini", c.Provider())
	}
}

func TestNewClientDefaultModel(t *testing.T) {
	cfg := config.AiConfig{GeminiAPIKey: "test"}
	c, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c.model != "gemini-2.0-flash" {
		t.Errorf("default model = %q, want 'gemini-2.0-flash'", c.model)
	}
}

func TestNewClientGroqDefaults(t *testing.T) {
	cfg := config.AiConfig{
		Provider:   "groq",
		GroqAPIKey: "test-key",
	}
	c, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if c.Provider() != "groq" {
		t.Errorf("provider = %q, want groq", c.Provider())
	}
	if c.Model() != "openai/gpt-oss-120b" {
		t.Errorf("model = %q, want openai/gpt-oss-120b", c.Model())
	}
	if !c.IsConfigured() {
		t.Fatal("expected groq client to be configured")
	}
}

func TestGenerateTextGroq(t *testing.T) {
	var gotAuth string
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"Generated content"}}]}`))
	}))
	defer server.Close()

	c, err := NewClient(config.AiConfig{
		Provider:    "groq",
		GroqAPIKey:  "test-key",
		GroqBaseURL: server.URL,
		GroqModel:   "openai/gpt-oss-120b",
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	got, err := c.GenerateText(context.Background(), "write something")
	if err != nil {
		t.Fatalf("GenerateText: %v", err)
	}
	if got != "Generated content" {
		t.Fatalf("GenerateText = %q", got)
	}
	if gotPath != "/chat/completions" {
		t.Fatalf("path = %q, want /chat/completions", gotPath)
	}
	if gotAuth != "Bearer test-key" {
		t.Fatalf("Authorization header = %q", gotAuth)
	}
}

func TestCacheKeyFor(t *testing.T) {
	key := cacheKeyFor("css-suggest", "main article content selector", "gemini-2.0-flash")
	if key == "" {
		t.Error("cacheKeyFor should not return empty string")
	}
}

func TestTrimBackticks(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"```json\n[\"a\", \"b\"]\n```", "[\"a\", \"b\"]"},
		{"[\"selector1\"]", "[\"selector1\"]"},
		{"```\nplain text\n```", "plain text"},
	}
	for _, c := range cases {
		got := trimBackticks(c.input)
		if got != c.expected {
			t.Errorf("trimBackticks(%q) = %q, want %q", c.input, got, c.expected)
		}
	}
}

func TestParseSelectors(t *testing.T) {
	// Valid JSON array.
	selectors, err := parseSelectors(`["article p", ".content", "#main"]`)
	if err != nil {
		t.Fatalf("parseSelectors: %v", err)
	}
	if len(selectors) != 3 {
		t.Errorf("selector count = %d, want 3", len(selectors))
	}

	// Markdown code block.
	selectors, err = parseSelectors("```json\n[\".class\", \"#id\"]\n```")
	if err != nil {
		t.Fatalf("parseSelectors from markdown: %v", err)
	}
	if len(selectors) != 2 {
		t.Errorf("selector count from markdown = %d, want 2", len(selectors))
	}

	// Invalid — should return error.
	_, err = parseSelectors("not a valid selector array")
	if err == nil {
		t.Error("parseSelectors should error on invalid input")
	}
}

func TestClientInMemoryCache(t *testing.T) {
	cfg := config.AiConfig{GeminiAPIKey: "test-key"}
	c, _ := NewClient(cfg)

	// Set a cached value.
	c.setCached(nil, "test-key", "cached response")

	// Get it back.
	got := c.getCached(nil, "test-key")
	if got != "cached response" {
		t.Errorf("getCached = %q, want 'cached response'", got)
	}
}

func TestTrimString(t *testing.T) {
	s := "this is a long string that should be trimmed"
	trimmed := trimString(s, 20)
	if len(trimmed) > 20 {
		t.Errorf("trimString should produce string of max length 20, got %d", len(trimmed))
	}
}
