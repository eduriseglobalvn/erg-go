// Package scraper provides robots.txt parsing and path compliance checking.
package scraper

import (
	"container/list"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/temoto/robotstxt"
)

// RobotsTxt wraps the temoto/robotstxt library with caching and crawl delay support.
type RobotsTxt struct {
	parser     *robotstxt.RobotsData
	crawlDelay time.Duration
	baseURL    string
}

// NewRobotsTxt parses a robots.txt content and returns a RobotsTxt instance.
func NewRobotsTxt(robotsContent []byte, baseURL string) (*RobotsTxt, error) {
	parser, err := robotstxt.FromBytes(robotsContent)
	if err != nil {
		return nil, fmt.Errorf("scraper: parse robots.txt: %w", err)
	}
	return &RobotsTxt{parser: parser, baseURL: baseURL}, nil
}

// CanBeFetched reports whether the given URL is allowed for the given user agent.
func (r *RobotsTxt) CanBeFetched(useragent, urlStr string) bool {
	if r.parser == nil {
		return true
	}

	// Extract path from URL for consistent matching.
	u, err := url.Parse(urlStr)
	if err != nil {
		return true
	}
	pathToTest := u.Path
	if pathToTest == "" {
		pathToTest = "/"
	}

	// Use RobotsData.TestAgent which handles user-agent matching correctly.
	return r.parser.TestAgent(pathToTest, useragent)
}

// CrawlDelay returns the crawl delay for the user agent, or 0 if not set.
func (r *RobotsTxt) CrawlDelay(useragent string) time.Duration {
	if r.parser == nil {
		return 0
	}
	group := r.parser.FindGroup(useragent)
	if group == nil {
		return 0
	}
	delay := group.CrawlDelay
	if delay <= 0 {
		return 0
	}
	return time.Duration(delay) * time.Second
}

// Sitemaps returns the list of sitemap URLs declared in robots.txt.
func (r *RobotsTxt) Sitemaps() []string {
	if r.parser == nil {
		return nil
	}
	return r.parser.Sitemaps
}

// RobotsCache holds cached robots.txt data per host with TTL eviction.
type RobotsCache struct {
	parsers map[string]*RobotsTxt
	expires map[string]time.Time
	ttl     time.Duration
	mu      sync.RWMutex
}

// NewRobotsCache creates a new robots cache with the given TTL.
// Expired entries are evicted lazily on next access.
func NewRobotsCache(ttl time.Duration) *RobotsCache {
	return &RobotsCache{
		parsers: make(map[string]*RobotsTxt),
		expires: make(map[string]time.Time),
		ttl:     ttl,
	}
}

// GetOrFetch returns the cached RobotsTxt for the host, or parses and caches new content.
// Expired entries are silently replaced.
func (c *RobotsCache) GetOrFetch(host string, content []byte) (*RobotsTxt, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if r, ok := c.parsers[host]; ok {
		if time.Now().Before(c.expires[host]) {
			return r, nil
		}
		// Expired — evict.
		delete(c.parsers, host)
		delete(c.expires, host)
	}

	r, err := NewRobotsTxt(content, "https://"+host)
	if err != nil {
		return nil, err
	}
	c.parsers[host] = r
	c.expires[host] = time.Now().Add(c.ttl)
	return r, nil
}

// Evict removes a host from the cache (e.g., on explicit invalidation).
func (c *RobotsCache) Evict(host string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.parsers, host)
	delete(c.expires, host)
}

// RobotsRule represents a single allow/disallow rule from robots.txt.
type RobotsRule struct {
	Path  string
	Allow bool
}

// SimpleRobotsParser is a lightweight robots.txt parser for when the full library is not needed.
type SimpleRobotsParser struct {
	rules      []RobotsRule
	crawlDelay time.Duration
}

// ParseSimpleRobotsTxt parses a robots.txt content string into simple rules.
func ParseSimpleRobotsTxt(content []byte) (*SimpleRobotsParser, error) {
	p := &SimpleRobotsParser{}
	lines := strings.Split(string(content), "\n")

	var currentUserAgent string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) < 2 {
			continue
		}
		key := strings.TrimSpace(strings.ToLower(parts[0]))
		value := strings.TrimSpace(parts[1])

		switch key {
		case "user-agent", "useragent":
			currentUserAgent = value
		case "disallow":
			if currentUserAgent == "*" || strings.Contains(currentUserAgent, "erg") {
				p.rules = append(p.rules, RobotsRule{Path: value, Allow: false})
			}
		case "allow":
			if currentUserAgent == "*" || strings.Contains(currentUserAgent, "erg") {
				p.rules = append(p.rules, RobotsRule{Path: value, Allow: true})
			}
		case "crawl-delay":
			if v, err := strconv.ParseFloat(value, 64); err == nil && v > 0 {
				p.crawlDelay = time.Duration(v) * time.Second
			}
		}
	}

	return p, nil
}

// SimpleCanBeFetched checks if a URL can be fetched according to the parsed rules.
// More specific (longer) patterns take precedence. For disallow rules, only apply
// when no more specific allow rule covers the URL.
func (p *SimpleRobotsParser) CanBeFetched(useragent, urlStr string) bool {
	if len(p.rules) == 0 {
		return true
	}

	u, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	allowed := true
	maxSpecificity := -1

	for _, rule := range p.rules {
		specificity := matchesPathSpecificity(u.Path, rule.Path)
		if specificity >= 0 && specificity >= maxSpecificity {
			maxSpecificity = specificity
			allowed = rule.Allow
		}
	}

	return allowed
}

// matchesPathSpecificity returns how specifically a pattern matches a URL path.
// Returns -1 if no match. Higher = more specific.
// For directory patterns like "/admin/", checks if path starts with that prefix.
func matchesPathSpecificity(urlPath, pattern string) int {
	if pattern == "" {
		return 0
	}
	if strings.HasSuffix(pattern, "$") {
		// Exact path match
		base := strings.TrimSuffix(pattern, "$")
		if urlPath == base {
			return len(base) + 1 // higher than prefix match
		}
		return -1
	}
	// Directory prefix match
	if strings.HasPrefix(urlPath, pattern) {
		return len(pattern)
	}
	// Glob match (* wildcard)
	globPattern := strings.ReplaceAll(regexp.QuoteMeta(pattern), `\*`, `.*`)
	re := getPathRegex(globPattern)
	if re.MatchString(urlPath) {
		return len(pattern) / 2 // less specific than exact/directory match
	}
	return -1
}

// SimpleCrawlDelay returns the crawl delay.
func (p *SimpleRobotsParser) CrawlDelay(useragent string) time.Duration {
	return p.crawlDelay
}

// matchesPath checks if a URL path matches a robots.txt pattern.
// Supports * wildcards and $ end-of-string matching.
// Compiles and caches regex per unique pattern using a bounded LRU cache.
func matchesPath(urlPath, pattern string) bool {
	if pattern == "" {
		return true
	}
	re := getPathRegex(pattern)
	return re.MatchString(urlPath)
}

const maxPathRegexEntries = 256

type pathRegexCache struct {
	maxEntries int
	order      *list.List
	entries    map[string]*list.Element
}

type pathRegexEntry struct {
	pattern string
	regex   *regexp.Regexp
}

func newPathRegexCache(maxEntries int) *pathRegexCache {
	return &pathRegexCache{
		maxEntries: maxEntries,
		order:      list.New(),
		entries:    make(map[string]*list.Element, maxEntries),
	}
}

func (c *pathRegexCache) Get(pattern string) (*regexp.Regexp, bool) {
	if elem, ok := c.entries[pattern]; ok {
		c.order.MoveToFront(elem)
		return elem.Value.(*pathRegexEntry).regex, true
	}
	return nil, false
}

func (c *pathRegexCache) Add(pattern string, re *regexp.Regexp) {
	if elem, ok := c.entries[pattern]; ok {
		elem.Value.(*pathRegexEntry).regex = re
		c.order.MoveToFront(elem)
		return
	}

	elem := c.order.PushFront(&pathRegexEntry{pattern: pattern, regex: re})
	c.entries[pattern] = elem
	if c.maxEntries > 0 && c.order.Len() > c.maxEntries {
		last := c.order.Back()
		if last == nil {
			return
		}
		c.order.Remove(last)
		delete(c.entries, last.Value.(*pathRegexEntry).pattern)
	}
}

func (c *pathRegexCache) Len() int {
	return c.order.Len()
}

var (
	regexCacheMu sync.Mutex
	regexCache   = newPathRegexCache(maxPathRegexEntries)
)

func getPathRegex(pattern string) *regexp.Regexp {
	regexCacheMu.Lock()
	if re, ok := regexCache.Get(pattern); ok {
		regexCacheMu.Unlock()
		return re
	}
	regexCacheMu.Unlock()

	regexPattern := regexp.QuoteMeta(pattern)
	regexPattern = strings.ReplaceAll(regexPattern, `\*`, `.*`)
	if strings.HasSuffix(pattern, `$`) {
		regexPattern = strings.TrimSuffix(regexPattern, `$`) + `$`
	} else {
		regexPattern += `.*`
	}
	re := regexp.MustCompile(regexPattern)

	regexCacheMu.Lock()
	regexCache.Add(pattern, re)
	regexCacheMu.Unlock()
	return re
}
