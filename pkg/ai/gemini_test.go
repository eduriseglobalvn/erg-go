package ai

import (
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
