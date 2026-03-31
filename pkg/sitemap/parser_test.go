package sitemap

import (
	"testing"
	"time"
)

const sampleSitemap = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>https://example.com/page1</loc>
    <lastmod>2024-01-01</lastmod>
    <changefreq>daily</changefreq>
    <priority>0.8</priority>
  </url>
  <url>
    <loc>https://example.com/page2</loc>
    <lastmod>2024-01-15T10:00:00Z</lastmod>
    <changefreq>weekly</changefreq>
    <priority>0.6</priority>
  </url>
</urlset>`

const sampleSitemapIndex = `<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <sitemap>
    <loc>https://example.com/sitemap-1.xml</loc>
    <lastmod>2024-01-01T00:00:00Z</lastmod>
  </sitemap>
  <sitemap>
    <loc>https://example.com/sitemap-2.xml</loc>
    <lastmod>2024-01-02T00:00:00Z</lastmod>
  </sitemap>
</sitemapindex>`

func TestNewParser(t *testing.T) {
	p := NewParser()
	if p == nil {
		t.Fatal("NewParser returned nil")
	}
}

func TestParseURLSet(t *testing.T) {
	p := NewParser()
	sm := &Sitemap{}
	if err := p.parseURLSet([]byte(sampleSitemap), sm); err != nil {
		t.Fatalf("parseURLSet: %v", err)
	}
	if len(sm.URLs) != 2 {
		t.Errorf("URL count = %d, want 2", len(sm.URLs))
	}
	if sm.URLs[0].Loc != "https://example.com/page1" {
		t.Errorf("first URL loc = %q", sm.URLs[0].Loc)
	}
	if sm.URLs[0].Priority != 0.8 {
		t.Errorf("first URL priority = %f, want 0.8", sm.URLs[0].Priority)
	}
}

func TestParseSitemapIndex(t *testing.T) {
	p := NewParser()
	sm := &Sitemap{}
	if err := p.parseSitemapIndex([]byte(sampleSitemapIndex), sm); err != nil {
		t.Fatalf("parseSitemapIndex: %v", err)
	}
	if !sm.IsIndex {
		t.Error("should be identified as a sitemap index")
	}
	if len(sm.Children) != 2 {
		t.Errorf("Children count = %d, want 2", len(sm.Children))
	}
}

func TestParseSitemapXML(t *testing.T) {
	p := NewParser()

	// URL set.
	sm1 := &Sitemap{}
	if err := p.parseSitemapXML([]byte(sampleSitemap), sm1); err != nil {
		t.Fatalf("parseSitemapXML (urlset): %v", err)
	}
	if sm1.IsIndex {
		t.Error("urlset should not be identified as index")
	}

	// Sitemap index.
	sm2 := &Sitemap{}
	if err := p.parseSitemapXML([]byte(sampleSitemapIndex), sm2); err != nil {
		t.Fatalf("parseSitemapXML (index): %v", err)
	}
	if !sm2.IsIndex {
		t.Error("sitemapindex should be identified as index")
	}
}

func TestParseSitemapTime(t *testing.T) {
	cases := []string{
		"2024-01-01",
		"2024-01-01T10:00:00Z",
		"2024-01-01T10:00:00+00:00",
	}
	for _, c := range cases {
		t_, err := parseSitemapTime(c)
		if err != nil {
			t.Errorf("parseSitemapTime(%q): %v", c, err)
		}
		_ = t_
	}

	_, err := parseSitemapTime("invalid date")
	if err == nil {
		t.Error("parseSitemapTime should error on invalid date")
	}
}

func TestExtractSitemapsFromRobots(t *testing.T) {
	content := []byte(`
User-agent: *
Disallow: /admin/
Sitemap: https://example.com/sitemap.xml
Sitemap: https://example.com/news-sitemap.xml
`)
	sitemaps := extractSitemapsFromRobots(content)
	if len(sitemaps) != 2 {
		t.Errorf("found %d sitemaps, want 2", len(sitemaps))
	}
}

func TestDeduplicateStrings(t *testing.T) {
	ss := []string{"a", "b", "a", "c", "b", "d"}
	deduped := deduplicateStrings(ss)
	if len(deduped) != 4 {
		t.Errorf("deduped count = %d, want 4", len(deduped))
	}
}

func TestURLStruct(t *testing.T) {
	u := URL{
		Loc:        "https://example.com/page",
		LastMod:    time.Now(),
		ChangeFreq: "daily",
		Priority:   0.7,
	}
	if u.Loc != "https://example.com/page" {
		t.Errorf("Loc = %q, want 'https://example.com/page'", u.Loc)
	}
}
