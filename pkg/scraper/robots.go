// Package scraper provides robots.txt parsing and path compliance checking.
package scraper

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/temoto/robotstxt"
)

// RobotsTxt wraps the temoto/robotstxt library with caching and crawl delay support.
type RobotsTxt struct {
	parser    *robotstxt.RobotsData
	crawlDelay time.Duration
	baseURL  string
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
	group := r.parser.FindGroup(useragent)
	if group == nil {
		return true
	}
	return group.Test(urlStr)
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

// RobotsCache holds cached robots.txt data per host.
type RobotsCache struct {
	parsers map[string]*RobotsTxt
	delay   time.Duration
}

// NewRobotsCache creates a new robots cache with the given TTL.
func NewRobotsCache(ttl time.Duration) *RobotsCache {
	return &RobotsCache{
		parsers: make(map[string]*RobotsTxt),
		delay:   ttl,
	}
}

// GetOrFetch returns the cached RobotsTxt for the host, or parses and caches new content.
func (c *RobotsCache) GetOrFetch(host string, content []byte) (*RobotsTxt, error) {
	if r, ok := c.parsers[host]; ok {
		return r, nil
	}
	r, err := NewRobotsTxt(content, "https://"+host)
	if err != nil {
		return nil, err
	}
	c.parsers[host] = r
	return r, nil
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
func (p *SimpleRobotsParser) CanBeFetched(useragent, urlStr string) bool {
	if len(p.rules) == 0 {
		return true
	}

	u, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	allowed := true
	for _, rule := range p.rules {
		if matchesPath(u.Path, rule.Path) {
			allowed = rule.Allow
		}
	}

	return allowed
}

// SimpleCrawlDelay returns the crawl delay.
func (p *SimpleRobotsParser) CrawlDelay(useragent string) time.Duration {
	return p.crawlDelay
}

// matchesPath checks if a URL path matches a robots.txt pattern.
// Supports * wildcards and $ end-of-string matching.
func matchesPath(urlPath, pattern string) bool {
	if pattern == "" {
		return true
	}

	regexPattern := regexp.QuoteMeta(pattern)
	regexPattern = strings.ReplaceAll(regexPattern, `\*`, `.*`)
	if strings.HasSuffix(pattern, `$`) {
		regexPattern = strings.TrimSuffix(regexPattern, `$`) + `$`
	} else {
		regexPattern += `.*`
	}

	ok, _ := regexp.MatchString("^"+regexPattern, urlPath)
	return ok
}

// pathDir returns the directory portion of a URL path.
func pathDir(p string) string {
	i := len(p) - 1
	for i >= 0 {
		if p[i] == '/' {
			break
		}
		i--
	}
	if i < 0 {
		return "/"
	}
	return p[:i+1]
}
