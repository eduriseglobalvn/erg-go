// Package rss provides parsers for RSS 2.0, Atom 1.0, and JSON Feed 1.1.
package rss

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	httpcli "erg.ninja/pkg/http"
)

// FeedType identifies the type of a feed.
type FeedType string

const (
	FeedRSS     FeedType = "rss"
	FeedAtom    FeedType = "atom"
	FeedJSON    FeedType = "json"
	FeedUnknown FeedType = "unknown"
)

// FeedItem represents a single item/entry from any feed type.
type FeedItem struct {
	Title         string
	Link          string
	Description   string
	Content       string // full content (may be HTML or plain text)
	PubDate       time.Time
	Author        string
	Categories    []string
	GUID          string // unique identifier
	ImageURL      string
	EnclosureURL  string
	EnclosureType string
	Source        string // feed source URL
}

// Feed represents a parsed feed with metadata.
type Feed struct {
	Type         FeedType
	Title        string
	Description  string
	Link         string
	Items        []FeedItem
	UpdatedAt    time.Time
	Language     string
	Generator    string
	ImageURL     string
	ETag         string
	LastModified string
	Source       string // original feed URL
}

// Parser extracts structured items from RSS, Atom, and JSON feeds.
type Parser struct {
	httpClient *httpcli.Client
	timeout    time.Duration
}

// ParserOption configures a Parser.
type ParserOption func(*Parser)

// WithHTTPParserClient sets a custom HTTP client for fetching feeds.
func WithHTTPParserClient(c *httpcli.Client) ParserOption {
	return func(p *Parser) {
		p.httpClient = c
	}
}

// NewParser creates a new feed parser.
func NewParser(opts ...ParserOption) *Parser {
	p := &Parser{
		httpClient: httpcli.NewClient(),
		timeout:    15 * time.Second,
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

// Fetch fetches and parses a feed from the given URL.
// It uses ETag/Last-Modified headers for conditional requests when provided.
func (p *Parser) Fetch(ctx context.Context, feedURL string, etag, lastModified string) (*Feed, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	headers := make(http.Header)
	if etag != "" {
		headers.Set("If-None-Match", etag)
	}
	if lastModified != "" {
		headers.Set("If-Modified-Since", lastModified)
	}

	resp, err := p.httpClient.Get(ctx, feedURL, headers)
	if err != nil {
		return nil, fmt.Errorf("rss: fetch %s: %w", feedURL, err)
	}
	defer resp.Body.Close()

	// 304 Not Modified — no update needed.
	if resp.StatusCode == http.StatusNotModified {
		return &Feed{Source: feedURL, ETag: etag, LastModified: lastModified}, nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10 MB max
	if err != nil {
		return nil, fmt.Errorf("rss: read body: %w", err)
	}

	// Update cache headers.
	feed := &Feed{
		Source:       feedURL,
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
	}

	// Detect feed type.
	feedType := detectFeedType(body)
	feed.Type = feedType

	switch feedType {
	case FeedRSS:
		if err := p.parseRSS(body, feed); err != nil {
			return nil, fmt.Errorf("rss: parse RSS: %w", err)
		}
	case FeedAtom:
		if err := p.parseAtom(body, feed); err != nil {
			return nil, fmt.Errorf("rss: parse Atom: %w", err)
		}
	case FeedJSON:
		if err := p.parseJSONFeed(body, feed); err != nil {
			return nil, fmt.Errorf("rss: parse JSON Feed: %w", err)
		}
	default:
		return nil, fmt.Errorf("rss: unknown feed type for %s", feedURL)
	}

	return feed, nil
}

// ParseFromBytes parses feed content from raw bytes, auto-detecting the feed type.
func (p *Parser) ParseFromBytes(data []byte, sourceURL string) (*Feed, error) {
	feed := &Feed{Source: sourceURL}
	feedType := detectFeedType(data)
	feed.Type = feedType

	switch feedType {
	case FeedRSS:
		if err := p.parseRSS(data, feed); err != nil {
			return nil, fmt.Errorf("rss: parse RSS: %w", err)
		}
	case FeedAtom:
		if err := p.parseAtom(data, feed); err != nil {
			return nil, fmt.Errorf("rss: parse Atom: %w", err)
		}
	case FeedJSON:
		if err := p.parseJSONFeed(data, feed); err != nil {
			return nil, fmt.Errorf("rss: parse JSON Feed: %w", err)
		}
	default:
		return nil, fmt.Errorf("rss: unknown feed type")
	}

	return feed, nil
}

// detectFeedType determines the feed type from the XML/JSON content.
func detectFeedType(data []byte) FeedType {
	trimmed := strings.TrimSpace(string(data))
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return FeedJSON
	}
	if strings.Contains(trimmed, "<rss") || strings.Contains(trimmed, "<channel>") {
		return FeedRSS
	}
	if strings.Contains(trimmed, "<feed") || strings.Contains(trimmed, "xmlns=\"http://www.w3.org/2005/Atom\"") {
		return FeedAtom
	}
	return FeedUnknown
}

// parseRSS parses RSS 2.0 feed content.
func (p *Parser) parseRSS(data []byte, feed *Feed) error {
	var rss rssFeed
	if err := xml.Unmarshal(data, &rss); err != nil {
		return fmt.Errorf("xml unmarshal: %w", err)
	}

	feed.Title = rss.Channel.Title
	feed.Description = rss.Channel.Description
	feed.Link = rss.Channel.Link
	feed.Language = rss.Channel.Language
	feed.Generator = rss.Channel.Generator
	feed.ImageURL = rss.Channel.Image.URL

	if rss.Channel.LastBuildDate != "" {
		if t, err := parseTime(rss.Channel.LastBuildDate); err == nil {
			feed.UpdatedAt = t
		}
	}

	for _, item := range rss.Channel.Items {
		fi := FeedItem{
			Title:       item.Title,
			Link:        item.Link,
			Description: item.Description,
			Content:     item.Content.Encoded,
			Author:      item.Author,
			GUID:        item.GUID,
			Source:      feed.Source,
		}
		if item.PubDate != "" {
			if t, err := parseTime(item.PubDate); err == nil {
				fi.PubDate = t
			}
		}
		fi.Categories = item.Categories
		if item.Enclosure.URL != "" {
			fi.EnclosureURL = item.Enclosure.URL
			fi.EnclosureType = item.Enclosure.Type
		}
		feed.Items = append(feed.Items, fi)
	}

	return nil
}

// parseAtom parses Atom 1.0 feed content.
func (p *Parser) parseAtom(data []byte, feed *Feed) error {
	var atomFeed atomFeedXML
	if err := xml.Unmarshal(data, &atomFeed); err != nil {
		return fmt.Errorf("xml unmarshal: %w", err)
	}

	feed.Title = atomFeed.Title
	feed.Description = atomFeed.Subtitle
	feed.Language = atomFeed.Lang
	feed.Generator = atomFeed.Generator

	if len(atomFeed.Links) > 0 {
		feed.Link = atomFeed.Links[0].Href
	}

	if !atomFeed.Updated.IsZero() {
		feed.UpdatedAt = atomFeed.Updated.Time
	}

	for _, entry := range atomFeed.Entries {
		fi := FeedItem{
			Title:  entry.Title,
			GUID:   entry.ID,
			Source: feed.Source,
		}
		if len(entry.Links) > 0 {
			for _, link := range entry.Links {
				if link.Rel == "alternate" || link.Rel == "" {
					fi.Link = link.Href
					break
				}
			}
		}
		if len(entry.Content) > 0 {
			fi.Content = entry.Content[0].Body
		}
		for _, author := range entry.Authors {
			if fi.Author == "" {
				fi.Author = author.Name
			}
		}
		if !entry.Published.IsZero() {
			fi.PubDate = entry.Published.Time
		} else if !entry.Updated.IsZero() {
			fi.PubDate = entry.Updated.Time
		}
		fi.Categories = make([]string, 0)
		for _, cat := range entry.Categories {
			fi.Categories = append(fi.Categories, cat.Term)
		}
		feed.Items = append(feed.Items, fi)
	}

	return nil
}

// parseJSONFeed parses JSON Feed 1.1.
func (p *Parser) parseJSONFeed(data []byte, feed *Feed) error {
	var jf jsonFeed
	if err := json.Unmarshal(data, &jf); err != nil {
		return fmt.Errorf("json unmarshal: %w", err)
	}

	feed.Title = jf.Title
	feed.Description = jf.Description
	feed.Link = jf.HomePageURL
	feed.Language = jf.Language
	feed.ImageURL = jf.Icon

	if !jf.DateModified.IsZero() {
		feed.UpdatedAt = jf.DateModified.Time
	}

	for _, item := range jf.Items {
		fi := FeedItem{
			Title:       item.Title,
			Link:        item.URL,
			Description: item.Summary,
			Content:     item.ContentHtml,
			Author:      item.Author.Name,
			GUID:        item.ID,
			Source:      feed.Source,
		}
		if !item.DatePublished.IsZero() {
			fi.PubDate = item.DatePublished.Time
		}
		fi.Categories = item.Tags
		if len(item.Attachments) > 0 {
			fi.EnclosureURL = item.Attachments[0].URL
			fi.EnclosureType = item.Attachments[0].MimeType
		}
		feed.Items = append(feed.Items, fi)
	}

	return nil
}

// parseTime attempts to parse a date string in various common formats.
func parseTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	formats := []string{
		time.RFC1123Z,
		time.RFC1123,
		time.RFC822Z,
		time.RFC822,
		time.RFC3339,
		"Mon, 02 Jan 2006 15:04:05 MST",
		"Mon, 2 Jan 2006 15:04:05 MST",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("rss: cannot parse date %q", s)
}

// ---- Atom XML Structures (inline to avoid external dependency) ----

// atomTime is a wrapper around time.Time that adds an IsZero() method
// compatible with how Atom timestamps are used in feeds.
type atomTime struct {
	time.Time
}

func (a atomTime) IsZero() bool {
	return a.Time.IsZero()
}

// atomFeedXML represents an Atom 1.0 feed.
type atomFeedXML struct {
	XMLName   xml.Name       `xml:"feed"`
	Title     string         `xml:"title"`
	Subtitle  string         `xml:"subtitle"`
	Links     []atomLinkXML  `xml:"link"`
	Updated   atomTime       `xml:"updated"`
	Generator string         `xml:"generator"`
	Lang      string         `xml:"lang"`
	Entries   []atomEntryXML `xml:"entry"`
}

// atomLinkXML represents a link element in Atom.
type atomLinkXML struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
}

// atomEntryXML represents an Atom 1.0 entry.
type atomEntryXML struct {
	Title      string            `xml:"title"`
	ID         string            `xml:"id"`
	Links      []atomLinkXML     `xml:"link"`
	Updated    atomTime          `xml:"updated"`
	Published  atomTime          `xml:"published"`
	Summary    string            `xml:"summary"`
	Content    []atomContentXML  `xml:"content"`
	Authors    []atomAuthorXML   `xml:"author"`
	Categories []atomCategoryXML `xml:"category"`
}

// atomContentXML represents the content element in Atom.
type atomContentXML struct {
	Body string `xml:",chardata"`
	Type string `xml:"type,attr"`
}

// atomAuthorXML represents an author element in Atom.
type atomAuthorXML struct {
	Name  string `xml:"name"`
	Email string `xml:"email"`
}

// atomCategoryXML represents a category element in Atom.
type atomCategoryXML struct {
	Term string `xml:"term,attr"`
}

// ---- XML Structures ----

type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title         string    `xml:"title"`
	Description   string    `xml:"description"`
	Link          string    `xml:"link"`
	Language      string    `xml:"language"`
	Generator     string    `xml:"generator"`
	LastBuildDate string    `xml:"lastBuildDate"`
	Image         rssImage  `xml:"image"`
	Items         []rssItem `xml:"item"`
}

type rssImage struct {
	URL string `xml:"url"`
}

type rssItem struct {
	Title       string       `xml:"title"`
	Link        string       `xml:"link"`
	Description string       `xml:"description"`
	Content     rssContent   `xml:"content:encoded"`
	Author      string       `xml:"author"`
	GUID        string       `xml:"guid"`
	PubDate     string       `xml:"pubDate"`
	Categories  []string     `xml:"category"`
	Enclosure   rssEnclosure `xml:"enclosure"`
}

type rssContent struct {
	Encoded string `xml:",chardata"`
}

type rssEnclosure struct {
	URL    string `xml:"url,attr"`
	Type   string `xml:"type,attr"`
	Length string `xml:"length,attr"`
}

// ---- JSON Feed Structures ----

type jsonFeed struct {
	Version      string         `json:"version"`
	Title        string         `json:"title"`
	Description  string         `json:"description"`
	HomePageURL  string         `json:"home_page_url"`
	Icon         string         `json:"icon"`
	Language     string         `json:"language"`
	DateModified jsonTime       `json:"date_modified"`
	Items        []jsonFeedItem `json:"items"`
}

type jsonTime struct {
	time.Time
}

func (j *jsonTime) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return err
	}
	j.Time = t
	return nil
}

type jsonFeedItem struct {
	ID            string           `json:"id"`
	URL           string           `json:"url"`
	Title         string           `json:"title"`
	Summary       string           `json:"summary"`
	ContentHtml   string           `json:"content_html"`
	Author        jsonAuthor       `json:"author"`
	DatePublished jsonTime         `json:"date_published"`
	Tags          []string         `json:"tags"`
	Attachments   []jsonAttachment `json:"attachments"`
}

type jsonAuthor struct {
	Name string `json:"name"`
}

type jsonAttachment struct {
	URL      string `json:"url"`
	MimeType string `json:"mime_type"`
}
