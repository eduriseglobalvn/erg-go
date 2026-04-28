// Package sitemap provides XML sitemap discovery, parsing, and URL extraction.
package sitemap

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	httpcli "erg.ninja/pkg/http"
	"erg.ninja/pkg/logger"
)

// Parser discovers and parses XML sitemaps.
type Parser struct {
	httpClient *httpcli.Client
	log        *logger.Logger
	timeout    time.Duration
}

// ParserOption configures a Parser.
type ParserOption func(*Parser)

// WithSitemapLogger sets the logger for the parser.
func WithSitemapLogger(log *logger.Logger) ParserOption {
	return func(p *Parser) {
		p.log = log
	}
}

// WithSitemapHTTPClient sets a custom HTTP client.
func WithSitemapHTTPClient(c *httpcli.Client) ParserOption {
	return func(p *Parser) {
		p.httpClient = c
	}
}

// NewParser creates a new sitemap parser.
func NewParser(opts ...ParserOption) *Parser {
	p := &Parser{
		httpClient: httpcli.NewClient(),
		log:        logger.NoOp(),
		timeout:    15 * time.Second,
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

// URL represents a single URL entry from a sitemap.
type URL struct {
	Loc        string
	LastMod    time.Time
	ChangeFreq string
	Priority   float64
	ParentURL  string // which sitemap this URL came from
}

// Sitemap represents a parsed sitemap (or sitemap index).
type Sitemap struct {
	URLs     []URL
	IsIndex  bool
	Children []string // child sitemap URLs (for sitemap indexes)
	Parent   string   // parent sitemap URL (for sitemap index entries)
}

// Fetch fetches and parses a sitemap from the given URL.
func (p *Parser) Fetch(ctx context.Context, sitemapURL string) (*Sitemap, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	req, err := http.NewRequestWithContext(ctx, "GET", sitemapURL, nil)
	if err != nil {
		return nil, fmt.Errorf("sitemap: new request: %w", err)
	}
	req.Header.Set("User-Agent", "erg-crawler/1.0 (+https://erg.ninja/bot)")

	resp, err := p.httpClient.Do(ctx, "GET", sitemapURL, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("sitemap: fetch %s: %w", sitemapURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sitemap: unexpected status %d for %s", resp.StatusCode, sitemapURL)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 50<<20)) // 50 MB max
	if err != nil {
		return nil, fmt.Errorf("sitemap: read body: %w", err)
	}

	sm := &Sitemap{Parent: sitemapURL}
	if err := p.parseSitemapXML(body, sm); err != nil {
		return nil, fmt.Errorf("sitemap: parse XML: %w", err)
	}

	return sm, nil
}

// Discover discovers sitemaps for a given domain.
// It checks common locations and robots.txt.
func (p *Parser) Discover(ctx context.Context, domain string) ([]string, error) {
	base, err := url.Parse(domain)
	if err != nil {
		return nil, fmt.Errorf("sitemap: parse domain: %w", err)
	}
	if base.Scheme == "" {
		base.Scheme = "https"
	}

	var sitemaps []string
	candidates := []string{
		base.String() + "/sitemap.xml",
		base.String() + "/sitemap_index.xml",
		base.String() + "/sitemap-index.xml",
		base.String() + "/wp-sitemap.xml", // WordPress
		base.String() + "/sitemap-news.xml",
	}

	for _, candidate := range candidates {
		found, err := p.checkExists(ctx, candidate)
		if err != nil {
			continue
		}
		if found {
			sitemaps = append(sitemaps, candidate)
		}
	}

	// Also check robots.txt.
	robotsURL := base.String() + "/robots.txt"
	robotsBody, err := p.fetchRobotsTxt(ctx, robotsURL)
	if err == nil {
		found := extractSitemapsFromRobots(robotsBody)
		for _, s := range found {
			sitemaps = append(sitemaps, s)
		}
	}

	return deduplicateStrings(sitemaps), nil
}

// FetchAll fetches a sitemap and any nested sitemaps up to maxDepth.
func (p *Parser) FetchAll(ctx context.Context, sitemapURL string, maxDepth int) ([]URL, error) {
	if maxDepth <= 0 {
		maxDepth = 3
	}

	var allURLs []URL
	seen := make(map[string]bool)
	var mu sync.Mutex

	type fetchJob struct {
		url    string
		depth  int
		parent string
	}

	jobs := make(chan fetchJob, 100)
	results := make(chan []URL, 50)
	errors := make(chan error, 50)

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				urls, err := p.Fetch(ctx, job.url)
				if err != nil {
					select {
					case errors <- fmt.Errorf("sitemap: fetch %s: %w", job.url, err):
					default:
					}
					continue
				}

				mu.Lock()
				for _, u := range urls.URLs {
					if !seen[u.Loc] {
						seen[u.Loc] = true
						allURLs = append(allURLs, u)
					}
				}
				mu.Unlock()

				// Recursively fetch child sitemaps if not at max depth.
				if job.depth < maxDepth && urls.IsIndex {
					for _, childURL := range urls.Children {
						if !seen[childURL] {
							seen[childURL] = true
							jobs <- fetchJob{url: childURL, depth: job.depth + 1}
						}
					}
				}

				select {
				case results <- urls.URLs:
				default:
				}
			}
		}()
	}

	// Start with the initial sitemap.
	jobs <- fetchJob{url: sitemapURL, depth: 0}

	// Close jobs channel when done.
	go func() {
		wg.Wait()
		close(results)
		close(errors)
	}()

	// Drain results.
	for range results {
		// Results are already processed inline.
	}

	// Drain errors.
	for err := range errors {
		p.log.WarnContext(ctx).Err(err).Msg("sitemap: fetch error")
	}

	return allURLs, nil
}

// DiscoverAndFetch combines discovery and full fetching.
func (p *Parser) DiscoverAndFetch(ctx context.Context, domain string, maxDepth int) ([]URL, error) {
	sitemaps, err := p.Discover(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("sitemap: discover: %w", err)
	}

	var allURLs []URL
	for _, smURL := range sitemaps {
		urls, err := p.FetchAll(ctx, smURL, maxDepth)
		if err != nil {
			p.log.WarnContext(ctx).Err(err).Str("sitemap", smURL).Msg("failed to fetch sitemap")
			continue
		}
		allURLs = append(allURLs, urls...)
	}

	return allURLs, nil
}

// parseSitemapXML parses the sitemap XML content into a Sitemap struct.
func (p *Parser) parseSitemapXML(data []byte, sm *Sitemap) error {
	// Detect if this is a sitemap index.
	trimmed := strings.TrimSpace(string(data))
	if strings.Contains(trimmed, "<sitemapindex") || strings.Contains(trimmed, "<sitemap ") {
		sm.IsIndex = true
		return p.parseSitemapIndex(data, sm)
	}
	if strings.Contains(trimmed, "<urlset") || strings.Contains(trimmed, "<url ") {
		sm.IsIndex = false
		return p.parseURLSet(data, sm)
	}
	return fmt.Errorf("sitemap: unknown sitemap format")
}

// parseURLSet parses a standard XML urlset.
func (p *Parser) parseURLSet(data []byte, sm *Sitemap) error {
	var xs urlSetXML
	if err := xml.Unmarshal(data, &xs); err != nil {
		return fmt.Errorf("sitemap: unmarshal urlset: %w", err)
	}

	for _, entry := range xs.URLs {
		u := URL{
			Loc:        entry.Loc,
			ChangeFreq: entry.ChangeFreq,
			Priority:   entry.Priority,
			ParentURL:  sm.Parent,
		}
		if entry.LastMod != "" {
			if t, err := parseSitemapTime(entry.LastMod); err == nil {
				u.LastMod = t
			}
		}
		sm.URLs = append(sm.URLs, u)
	}

	return nil
}

// parseSitemapIndex parses a sitemap index file.
// Detects XML namespace usage and handles both forms:
// Non-namespaced: <sitemapindex><sitemap><loc>URL</loc></sitemap></sitemapindex>
// Namespaced:     <sitemapindex xmlns="..."><sitemap><loc>URL</loc></sitemap></sitemapindex>
func (p *Parser) parseSitemapIndex(data []byte, sm *Sitemap) error {
	// Check if this is a namespaced XML document (has xmlns on root).
	hasNS := strings.Contains(string(data), `xmlns=`)
	isNamespaced := hasNS

	if !isNamespaced {
		// Simple case: parse directly with unmarshal.
		var si sitemapIndexXML
		if err := xml.Unmarshal(data, &si); err != nil {
			return fmt.Errorf("sitemap: unmarshal sitemapindex: %w", err)
		}
		for _, s := range si.Sitemaps {
			sm.Children = append(sm.Children, s.Loc)
		}
		return nil
	}

	// Namespaced: extract <loc> values directly by finding them as text between tags.
	// This avoids the namespace decoding issues with xml.Decoder.
	return p.parseSitemapIndexNS(data, sm)
}

// parseSitemapIndexNS extracts <loc> values from namespaced sitemap index XML.
func (p *Parser) parseSitemapIndexNS(data []byte, sm *Sitemap) error {
	content := string(data)
	// Find all <loc>...</loc> pairs at the sitemap level.
	// Split by <sitemap> and extract loc from each.
	parts := strings.Split(content, "<sitemap>")
	for _, part := range parts[1:] { // skip first (before any <sitemap>)
		// Find the <loc>...</loc> within this <sitemap> entry.
		locStart := strings.Index(part, "<loc>")
		if locStart < 0 {
			continue
		}
		rest := part[locStart+5:] // skip "<loc>"
		locEnd := strings.Index(rest, "</loc>")
		if locEnd < 0 {
			continue
		}
		loc := strings.TrimSpace(rest[:locEnd])
		if loc != "" {
			sm.Children = append(sm.Children, loc)
		}
	}
	return nil
}

// checkExists checks if a URL returns a 200 OK response.
func (p *Parser) checkExists(ctx context.Context, urlStr string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, "HEAD", urlStr, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("User-Agent", "erg-crawler/1.0")

	resp, err := p.httpClient.Do(ctx, "GET", urlStr, nil, nil)
	if err != nil {
		return false, nil // Don't treat as error, just means not found.
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	return resp.StatusCode == http.StatusOK, nil
}

// fetchRobotsTxt fetches the robots.txt file content.
func (p *Parser) fetchRobotsTxt(ctx context.Context, robotsURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", robotsURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "erg-crawler/1.0")

	resp, err := p.httpClient.Do(ctx, "GET", robotsURL, nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("robots.txt returned status %d", resp.StatusCode)
	}

	return io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MB max
}

// extractSitemapsFromRobots parses Sitemap: directives from robots.txt content.
func extractSitemapsFromRobots(content []byte) []string {
	var sitemaps []string
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(strings.ToLower(line))
		if strings.HasPrefix(line, "sitemap:") {
			url := strings.TrimSpace(strings.TrimPrefix(line, "sitemap:"))
			if url != "" {
				sitemaps = append(sitemaps, url)
			}
		}
	}
	return sitemaps
}

// deduplicateStrings removes duplicate strings from a slice.
func deduplicateStrings(ss []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(ss))
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// parseSitemapTime parses a datetime in common sitemap formats.
func parseSitemapTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	formats := []string{
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05Z",
		"2006-01-02",
		time.RFC3339,
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("sitemap: cannot parse time %q", s)
}

// ---- XML Structures ----

type urlSetXML struct {
	XMLName xml.Name      `xml:"urlset"`
	URLs    []urlEntryXML `xml:"url"`
}

type urlEntryXML struct {
	Loc        string  `xml:"loc"`
	LastMod    string  `xml:"lastmod"`
	ChangeFreq string  `xml:"changefreq"`
	Priority   float64 `xml:"priority"`
}

type sitemapIndexXML struct {
	XMLName  xml.Name        `xml:"sitemapindex"`
	Sitemaps []sitemapRefXML `xml:"sitemap"`
}

type sitemapRefXML struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod"`
}
