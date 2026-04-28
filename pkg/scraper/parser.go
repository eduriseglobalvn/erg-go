// Package scraper provides HTML parsing utilities using goquery.
package scraper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Parser extracts structured data from HTML documents.
type Parser struct{}

// NewParser creates a new HTML parser.
func NewParser() *Parser {
	return &Parser{}
}

// ParseResult holds the extracted data from a page.
type ParseResult struct {
	Title       string
	Description string
	Links       []string
	Images      []string
	Headings    map[string][]string // h1, h2, h3 → text content
	Metadata    map[string]string   // og:title, description, etc.
	RawText     string
	Error       error
}

// ParseHTML parses an HTML document and extracts structured data.
func (p *Parser) ParseHTML(html []byte, baseURL string) *ParseResult {
	r := &ParseResult{
		Headings: make(map[string][]string),
		Metadata: make(map[string]string),
		Links:    make([]string, 0),
		Images:   make([]string, 0),
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		r.Error = fmt.Errorf("scraper: parse HTML: %w", err)
		return r
	}

	// Extract title.
	r.Title = doc.Find("title").First().Text()

	// Extract meta description.
	doc.Find("meta[name=description]").Each(func(_ int, s *goquery.Selection) {
		if val, ok := s.Attr("content"); ok {
			r.Description = val
		}
	})

	// Extract Open Graph metadata.
	doc.Find("meta[property^=og:]").Each(func(_ int, s *goquery.Selection) {
		prop, _ := s.Attr("property")
		content, _ := s.Attr("content")
		r.Metadata[prop] = content
	})

	// Extract headings.
	for _, tag := range []string{"h1", "h2", "h3", "h4", "h5", "h6"} {
		doc.Find(tag).Each(func(_ int, s *goquery.Selection) {
			text := strings.TrimSpace(s.Text())
			if text != "" {
				r.Headings[tag] = append(r.Headings[tag], text)
			}
		})
	}

	// Extract links.
	base, _ := url.Parse(baseURL)
	doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		href, ok := s.Attr("href")
		if !ok || href == "" || strings.HasPrefix(href, "#") || strings.HasPrefix(href, "javascript:") {
			return
		}
		absURL := href
		if u, err := url.Parse(href); err == nil && !u.IsAbs() && base != nil {
			absURL = base.ResolveReference(u).String()
		}
		r.Links = append(r.Links, absURL)
	})

	// Extract images.
	doc.Find("img[src]").Each(func(_ int, s *goquery.Selection) {
		src, _ := s.Attr("src")
		if src != "" {
			r.Images = append(r.Images, src)
		}
	})

	// Extract visible text content.
	var textBuilder strings.Builder
	doc.Find("p,li,td,th,div,span,article,section").Each(func(_ int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if len(text) > 20 {
			textBuilder.WriteString(text)
			textBuilder.WriteString("\n")
		}
	})
	r.RawText = strings.TrimSpace(textBuilder.String())

	return r
}

// ExtractBySelector extracts all text content matching a CSS selector.
func (p *Parser) ExtractBySelector(ctx context.Context, html []byte, selector string) ([]string, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("scraper: parse HTML: %w", err)
	}

	var results []string
	doc.Find(selector).Each(func(_ int, s *goquery.Selection) {
		if ctx != nil && ctx.Err() != nil {
			return
		}
		results = append(results, s.Text())
	})
	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	return results, nil
}

// ExtractLinks extracts all absolute URLs from anchor tags.
func (p *Parser) ExtractLinks(ctx context.Context, html []byte, baseURL string) ([]string, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("scraper: parse HTML: %w", err)
	}

	base, _ := url.Parse(baseURL)
	var links []string

	doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		if ctx != nil && ctx.Err() != nil {
			return
		}
		href, _ := s.Attr("href")
		if href == "" || strings.HasPrefix(href, "#") {
			return
		}
		u, err := url.Parse(href)
		if err != nil {
			return
		}
		if base != nil {
			links = append(links, base.ResolveReference(u).String())
		} else {
			links = append(links, u.String())
		}
	})

	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	return links, nil
}

// ExtractJSONLD extracts JSON-LD structured data from a page.
// Handles both @type array and single @type, and unwraps @graph entries.
func (p *Parser) ExtractJSONLD(ctx context.Context, html []byte) ([]map[string]interface{}, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("scraper: parse HTML: %w", err)
	}

	var results []map[string]interface{}
	doc.Find("script[type=application/ld+json]").Each(func(_ int, s *goquery.Selection) {
		if ctx != nil && ctx.Err() != nil {
			return
		}
		text := strings.TrimSpace(s.Text())
		if text == "" {
			return
		}

		// Try parsing as a JSON array first.
		var arr []map[string]interface{}
		if err := json.Unmarshal([]byte(text), &arr); err == nil {
			for _, item := range arr {
				if m := extractGraph(item); m != nil {
					results = append(results, m)
				}
			}
			return
		}

		// Fall back to single object.
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(text), &obj); err == nil {
			if m := extractGraph(obj); m != nil {
				results = append(results, m)
			}
		}
	})

	if err := contextErr(ctx); err != nil {
		return nil, err
	}
	return results, nil
}

func contextErr(ctx context.Context) error {
	if ctx == nil || ctx.Err() == nil {
		return nil
	}
	return fmt.Errorf("scraper: parse cancelled: %w", ctx.Err())
}

// extractGraph unwraps JSON-LD @graph arrays, returning the innermost item
// or the original if no @graph is present.
func extractGraph(item map[string]interface{}) map[string]interface{} {
	if graph, ok := item["@graph"].([]interface{}); ok && len(graph) > 0 {
		// Prefer the main entity (last @type != Article/ItemPage/WebPage).
		for i := len(graph) - 1; i >= 0; i-- {
			if m, ok := graph[i].(map[string]interface{}); ok {
				item = m
				break
			}
		}
	}
	return item
}

// GetFaviconURL returns the favicon URL for a page.
func GetFaviconURL(baseURL string) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s://%s/favicon.ico", u.Scheme, u.Host)
}

// ScrapeMetaTags extracts all meta tags from an HTML document.
func ScrapeMetaTags(html io.Reader) (map[string]string, error) {
	doc, err := goquery.NewDocumentFromReader(html)
	if err != nil {
		return nil, fmt.Errorf("scraper: parse HTML: %w", err)
	}

	meta := make(map[string]string)
	doc.Find("meta").Each(func(_ int, s *goquery.Selection) {
		name, _ := s.Attr("name")
		property, _ := s.Attr("property")
		content, _ := s.Attr("content")
		if name != "" {
			meta[name] = content
		}
		if property != "" {
			meta[property] = content
		}
	})

	return meta, nil
}
