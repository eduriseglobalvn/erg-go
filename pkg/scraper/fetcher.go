// Package scraper provides HTTP fetching with anti-blocking measures, robots.txt
// compliance, proxy rotation, and adaptive delays.
package scraper

import (
	"context"
	crypto_rand "crypto/rand"
	"fmt"
	"io"
	"math/big"
	stdhttp "net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"erg.ninja/pkg/config"
	safehttp "erg.ninja/pkg/http"
	"erg.ninja/pkg/logger"
)

// RobotsParser checks whether a URL can be fetched according to robots.txt.
type RobotsParser interface {
	CanBeFetched(useragent, url string) bool
	CrawlDelay(useragent string) time.Duration
}

// Fetcher fetches web pages with anti-blocking and policy compliance.
type Fetcher struct {
	client       *stdhttp.Client
	cfg          config.ScraperConfig
	log          *logger.Logger
	robotsParser RobotsParser
	userAgents   []string
	proxyURLs    []string
	proxyIndex   int
	proxyMu      sync.Mutex
	lastReqTime  time.Time
	lastReqMu    sync.Mutex
}

// FetcherOption configures a Fetcher.
type FetcherOption func(*Fetcher)

// WithRobotsParser sets a custom robots.txt parser.
func WithRobotsParser(r RobotsParser) FetcherOption {
	return func(f *Fetcher) {
		f.robotsParser = r
	}
}

// WithLogger sets the logger for the fetcher.
func WithFetcherLogger(log *logger.Logger) FetcherOption {
	return func(f *Fetcher) {
		f.log = log
	}
}

// NewFetcher creates a new web page fetcher with anti-blocking measures.
func NewFetcher(cfg config.ScraperConfig, opts ...FetcherOption) *Fetcher {
	f := &Fetcher{
		cfg:        cfg,
		log:        logger.NoOp(),
		userAgents: cfg.UserAgents,
		proxyURLs:  cfg.ProxyURLs,
	}
	for _, o := range opts {
		o(f)
	}

	transport := &stdhttp.Transport{
		DialContext:           safehttp.SafeDialContext(safehttp.SafeDialerConfig{}),
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	f.client = &stdhttp.Client{
		Transport: transport,
		Timeout:   cfg.Timeout,
	}

	return f
}

// FetchResult holds the result of a page fetch.
type FetchResult struct {
	URL         string
	StatusCode  int
	Body        []byte
	ContentType string
	Duration    time.Duration
	Error       error
}

// Fetch fetches a URL, respecting robots.txt and applying anti-blocking delays.
func (f *Fetcher) Fetch(ctx context.Context, targetURL string) *FetchResult {
	start := time.Now()

	if _, err := url.Parse(targetURL); err != nil {
		return &FetchResult{URL: targetURL, Error: fmt.Errorf("scraper: parse URL: %w", err)}
	}

	// Check robots.txt.
	if f.cfg.RespectRobotsTxt && f.robotsParser != nil {
		if !f.robotsParser.CanBeFetched("erg-crawler", targetURL) {
			return &FetchResult{
				URL:   targetURL,
				Error: fmt.Errorf("scraper: URL disallowed by robots.txt: %s", targetURL),
			}
		}
		// Respect crawl-delay.
		delay := f.robotsParser.CrawlDelay("erg-crawler")
		if delay < f.cfg.MinDelay {
			delay = f.cfg.MinDelay
		}
		f.applyDelay(ctx, delay)
	} else {
		f.applyDelay(ctx, f.cfg.MinDelay)
	}

	// Select User-Agent.
	ua := f.selectUserAgent()

	// Select Proxy.
	proxyURL, err := f.selectProxy()
	if err != nil {
		// No proxy needed if none configured.
		proxyURL = nil
	}

	// Build request.
	req, err := stdhttp.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return &FetchResult{URL: targetURL, Error: fmt.Errorf("scraper: new request: %w", err)}
	}

	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip, deflate")

	if proxyURL != nil {
		req.Header.Set("X-Forwarded-For", proxyURL.Host)
	}

	// Execute with retries.
	var lastErr error
	for attempt := 0; attempt <= f.cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*attempt) * time.Second
			select {
			case <-ctx.Done():
				return &FetchResult{URL: targetURL, Error: ctx.Err()}
			case <-time.After(backoff):
			}
		}

		resp, err := f.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("scraper: do %s: %w", targetURL, err)
			continue
		}

		// Check for block patterns.
		if f.isBlocked(resp) {
			resp.Body.Close()
			return &FetchResult{
				URL:        targetURL,
				StatusCode: resp.StatusCode,
				Error:      fmt.Errorf("scraper: block detected for %s (status %d)", targetURL, resp.StatusCode),
			}
		}

		// Enforce max response size.
		limitedReader := io.LimitReader(resp.Body, f.cfg.MaxResponseSize)
		body, err := io.ReadAll(limitedReader)
		resp.Body.Close()

		if err != nil {
			lastErr = fmt.Errorf("scraper: read body %s: %w", targetURL, err)
			continue
		}

		return &FetchResult{
			URL:         targetURL,
			StatusCode:  resp.StatusCode,
			Body:        body,
			ContentType: resp.Header.Get("Content-Type"),
			Duration:    time.Since(start),
		}
	}

	return &FetchResult{URL: targetURL, Error: lastErr}
}

// FetchWithEtag performs a conditional fetch using ETag/Last-Modified headers.
func (f *Fetcher) FetchWithEtag(ctx context.Context, targetURL, etag, lastModified string) *FetchResult {
	req, err := stdhttp.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return &FetchResult{URL: targetURL, Error: fmt.Errorf("scraper: new request: %w", err)}
	}
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	if lastModified != "" {
		req.Header.Set("If-Modified-Since", lastModified)
	}

	// Use a single attempt for conditional requests.
	resp, err := f.client.Do(req)
	if err != nil {
		return &FetchResult{URL: targetURL, Error: fmt.Errorf("scraper: conditional fetch: %w", err)}
	}
	defer resp.Body.Close()

	// 304 Not Modified — content hasn't changed.
	if resp.StatusCode == stdhttp.StatusNotModified {
		return &FetchResult{URL: targetURL, StatusCode: resp.StatusCode}
	}

	limitedReader := io.LimitReader(resp.Body, f.cfg.MaxResponseSize)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return &FetchResult{URL: targetURL, StatusCode: resp.StatusCode, Error: fmt.Errorf("scraper: read body: %w", err)}
	}

	return &FetchResult{
		URL:         targetURL,
		StatusCode:  resp.StatusCode,
		Body:        body,
		ContentType: resp.Header.Get("Content-Type"),
	}
}

// isBlocked detects block patterns from response status codes and body content.
func (f *Fetcher) isBlocked(resp *stdhttp.Response) bool {
	// Check status codes.
	for _, code := range f.cfg.BlockStatusCodes {
		if resp.StatusCode == code {
			return true
		}
	}
	return false
}

// checkBlockPatterns scans the body for block-related keywords.
func (f *Fetcher) checkBlockPatterns(body []byte) bool {
	bodyLower := strings.ToLower(string(body))
	for _, pattern := range f.cfg.BlockPatterns {
		if strings.Contains(bodyLower, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

// applyDelay waits the minimum time between requests to avoid rate limiting.
func (f *Fetcher) applyDelay(ctx context.Context, minDelay time.Duration) {
	f.lastReqMu.Lock()
	defer f.lastReqMu.Unlock()

	deadline := f.lastReqTime.Add(minDelay)
	now := time.Now()
	if deadline.After(now) {
		select {
		case <-ctx.Done():
			return
		case <-time.After(deadline.Sub(now)):
		}
	}
	f.lastReqTime = time.Now()
}

// selectUserAgent randomly selects a User-Agent from the configured list.
func (f *Fetcher) selectUserAgent() string {
	if len(f.userAgents) == 0 {
		return "Mozilla/5.0 (compatible; erg-crawler/1.0)"
	}
	return f.userAgents[randomIndex(len(f.userAgents))]
}

func randomIndex(length int) int {
	if length <= 1 {
		return 0
	}
	n, err := crypto_rand.Int(crypto_rand.Reader, big.NewInt(int64(length)))
	if err != nil {
		return int(time.Now().UnixNano() % int64(length))
	}
	return int(n.Int64())
}

// selectProxy rotates through configured proxy URLs in round-robin fashion.
func (f *Fetcher) selectProxy() (*url.URL, error) {
	if len(f.proxyURLs) == 0 {
		return nil, nil
	}
	f.proxyMu.Lock()
	defer f.proxyMu.Unlock()
	proxyStr := f.proxyURLs[f.proxyIndex%len(f.proxyURLs)]
	f.proxyIndex++
	return url.Parse(proxyStr)
}

// FetchRobotsTxt fetches and returns the robots.txt content for a base URL.
// Respects context deadline/cancellation and enforces a 1 MB max response.
func FetchRobotsTxt(ctx context.Context, baseURL string) ([]byte, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("scraper: parse base URL: %w", err)
	}
	robotsURL := fmt.Sprintf("%s://%s/robots.txt", u.Scheme, u.Host)

	req, err := stdhttp.NewRequestWithContext(ctx, "GET", robotsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("scraper: new robots.txt request: %w", err)
	}
	req.Header.Set("User-Agent", "erg-crawler/1.0")
	req.Header.Set("Accept", "text/plain,text/html,*/*")

	// Honor context deadline; cap at 10s so a nil context never blocks forever.
	deadline := 10 * time.Second
	if d, ok := ctx.Deadline(); ok {
		if remaining := time.Until(d); remaining > 0 && remaining < deadline {
			deadline = remaining
		}
	}

	client := &stdhttp.Client{
		Timeout: deadline,
		Transport: &stdhttp.Transport{
			DialContext:         safehttp.SafeDialContext(safehttp.SafeDialerConfig{}),
			MaxIdleConns:        5,
			IdleConnTimeout:     30 * time.Second,
			TLSHandshakeTimeout: 5 * time.Second,
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("scraper: fetch robots.txt: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != stdhttp.StatusOK {
		return nil, fmt.Errorf("scraper: robots.txt returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // max 1 MB
	if err != nil {
		return nil, fmt.Errorf("scraper: read robots.txt: %w", err)
	}
	return body, nil
}
