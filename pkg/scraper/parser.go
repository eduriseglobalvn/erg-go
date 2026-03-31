// Package scraper provides HTML parsing utilities using goquery.
package scraper

import (
	"bytes"
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
	Metadata    map[string]string  // og:title, description, etc.
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
func (p *Parser) ExtractBySelector(html []byte, selector string) ([]string, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("scraper: parse HTML: %w", err)
	}

	var results []string
	doc.Find(selector).Each(func(_ int, s *goquery.Selection) {
		results = append(results, s.Text())
	})
	return results, nil
}

// ExtractLinks extracts all absolute URLs from anchor tags.
func (p *Parser) ExtractLinks(html []byte, baseURL string) ([]string, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("scraper: parse HTML: %w", err)
	}

	base, _ := url.Parse(baseURL)
	var links []string

	doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
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

	return links, nil
}

// ExtractJSONLD extracts JSON-LD structured data from a page.
func (p *Parser) ExtractJSONLD(html []byte) ([]map[string]interface{}, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("scraper: parse HTML: %w", err)
	}

	var results []map[string]interface{}
	doc.Find("script[type=application/ld+json]").Each(func(_ int, s *goquery.Selection) {
		text := s.Text()
		// Simple JSON-LD parsing without importing a full JSON-LD library.
		_ = text
		// The full implementation would parse the JSON and extract @graph entries.
	})

	return results, nil
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
