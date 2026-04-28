package scraper

import (
	"testing"
	"time"

	"erg.ninja/pkg/config"
)

func TestNewRobotsTxt(t *testing.T) {
	content := []byte(`
User-agent: *
Disallow: /admin/
Disallow: /private/
Allow: /public/
Crawl-delay: 5
Sitemap: https://example.com/sitemap.xml
`)
	r, err := NewRobotsTxt(content, "https://example.com")
	if err != nil {
		t.Fatalf("NewRobotsTxt: %v", err)
	}
	if r == nil {
		t.Fatal("NewRobotsTxt returned nil")
	}
}

func TestRobotsTxtCanBeFetched(t *testing.T) {
	content := []byte(`
User-agent: erg-crawler
Disallow: /admin/
Disallow: /api/private/
Allow: /api/public/
Crawl-delay: 10
`)
	r, err := NewRobotsTxt(content, "https://example.com")
	if err != nil {
		t.Fatalf("NewRobotsTxt: %v", err)
	}

	if !r.CanBeFetched("erg-crawler", "https://example.com/index.html") {
		t.Error("index.html should be allowed")
	}
	if r.CanBeFetched("erg-crawler", "https://example.com/admin/login") {
		t.Error("/admin/login should be disallowed")
	}
	if !r.CanBeFetched("erg-crawler", "https://example.com/api/public/data") {
		t.Error("/api/public/data should be allowed")
	}
}

func TestRobotsTxtCrawlDelay(t *testing.T) {
	content := []byte(`
User-agent: erg-crawler
Crawl-delay: 7
`)
	r, err := NewRobotsTxt(content, "https://example.com")
	if err != nil {
		t.Fatalf("NewRobotsTxt: %v", err)
	}

	delay := r.CrawlDelay("erg-crawler")
	if delay == 0 {
		t.Error("CrawlDelay should not be zero")
	}
}

func TestRobotsTxtSitemaps(t *testing.T) {
	content := []byte(`
Sitemap: https://example.com/sitemap.xml
Sitemap: https://example.com/news-sitemap.xml
`)
	r, err := NewRobotsTxt(content, "https://example.com")
	if err != nil {
		t.Fatalf("NewRobotsTxt: %v", err)
	}

	sitemaps := r.Sitemaps()
	if len(sitemaps) != 2 {
		t.Errorf("Sitemaps count = %d, want 2", len(sitemaps))
	}
}

func TestParseSimpleRobotsTxt(t *testing.T) {
	content := []byte(`
User-agent: *
Disallow: /admin/
Allow: /api/
Crawl-delay: 3
`)
	p, err := ParseSimpleRobotsTxt(content)
	if err != nil {
		t.Fatalf("ParseSimpleRobotsTxt: %v", err)
	}

	if !p.CanBeFetched("other-agent", "https://example.com/index") {
		t.Error("/index should be allowed for other-agent")
	}
	if p.CanBeFetched("other-agent", "https://example.com/admin/") {
		t.Error("/admin/ should be disallowed (rule has trailing slash)")
	}
	delay := p.CrawlDelay("other-agent")
	if delay == 0 {
		t.Error("CrawlDelay should not be zero")
	}
}

func TestMatchesPath(t *testing.T) {
	cases := []struct {
		path     string
		pattern  string
		expected bool
	}{
		{"/admin/login", "/admin/", true},
		{"/public/index", "/admin/", false},
		{"/api/v1/users", "/api/", true},
		{"/api/v1", "/api/", true},
		{"/", "/", true},
		{"", "", true},
	}
	for _, c := range cases {
		got := matchesPath(c.path, c.pattern)
		if got != c.expected {
			t.Errorf("matchesPath(%q, %q) = %v, want %v", c.path, c.pattern, got, c.expected)
		}
	}
}

func TestPathRegexCacheBounded(t *testing.T) {
	regexCacheMu.Lock()
	regexCache = newPathRegexCache(4)
	regexCacheMu.Unlock()

	for _, pattern := range []string{"/a/*", "/b/*", "/c/*", "/d/*", "/e/*"} {
		_ = getPathRegex(pattern)
	}

	regexCacheMu.Lock()
	defer regexCacheMu.Unlock()
	if got := regexCache.Len(); got != 4 {
		t.Fatalf("regex cache len = %d, want 4", got)
	}
	if _, ok := regexCache.Get("/a/*"); ok {
		t.Fatal("expected oldest regex entry to be evicted")
	}
}

func TestFetcherDefaults(t *testing.T) {
	cfg := config.ScraperConfig{
		UserAgents:       []string{"TestBot/1.0"},
		ProxyURLs:        []string{},
		MinDelay:         3 * time.Second,
		MaxDelay:         10 * time.Second,
		Timeout:          30 * time.Second,
		MaxResponseSize:  10 << 20,
		RespectRobotsTxt: true,
		MaxRetries:       3,
	}

	f := NewFetcher(cfg)
	if f == nil {
		t.Fatal("NewFetcher returned nil")
	}
	if len(f.userAgents) != 1 {
		t.Error("userAgents not set correctly")
	}
}

func TestSelectUserAgent(t *testing.T) {
	cfg := config.ScraperConfig{
		UserAgents: []string{"BotA/1.0", "BotB/2.0", "BotC/3.0"},
	}
	f := NewFetcher(cfg)

	// Should not panic across multiple calls.
	for i := 0; i < 100; i++ {
		ua := f.selectUserAgent()
		if ua == "" {
			t.Error("selectUserAgent returned empty string")
		}
	}
}

func TestSelectProxy(t *testing.T) {
	cfg := config.ScraperConfig{
		ProxyURLs: []string{"http://proxy1:8080", "http://proxy2:8080"},
	}
	f := NewFetcher(cfg)

	// Should rotate through proxies.
	u1, _ := f.selectProxy()
	u2, _ := f.selectProxy()
	u3, _ := f.selectProxy()

	if u1 == nil || u2 == nil || u3 == nil {
		t.Error("selectProxy should not return nil when proxies are configured")
	}
	if u1.Host != "proxy1:8080" {
		t.Errorf("first proxy = %v, want proxy1:8080", u1.Host)
	}
}

func TestCheckBlockPatterns(t *testing.T) {
	cfg := config.ScraperConfig{
		BlockPatterns: []string{"captcha", "access denied", "blocked"},
	}
	f := NewFetcher(cfg)

	if !f.checkBlockPatterns([]byte("This page contains captcha challenges")) {
		t.Error("should detect captcha pattern")
	}
	if f.checkBlockPatterns([]byte("Normal page content here")) {
		t.Error("should not detect block pattern in normal content")
	}
}
