package rss

import (
	"testing"
)

const sampleRSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
<channel>
  <title>Test Feed</title>
  <link>https://example.com</link>
  <description>A test RSS feed</description>
  <language>en-us</language>
  <item>
    <title>First Post</title>
    <link>https://example.com/post1</link>
    <description>This is the first post</description>
    <pubDate>Mon, 01 Jan 2024 12:00:00 GMT</pubDate>
    <guid>post-001</guid>
    <category>tech</category>
  </item>
  <item>
    <title>Second Post</title>
    <link>https://example.com/post2</link>
    <guid>post-002</guid>
    <pubDate>Tue, 02 Jan 2024 12:00:00 GMT</pubDate>
  </item>
</channel>
</rss>`

const sampleAtom = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Atom Test Feed</title>
  <link href="https://example.com"/>
  <subtitle>Test description</subtitle>
  <updated>2024-01-01T12:00:00Z</updated>
  <entry>
    <title>Atom Entry 1</title>
    <id>urn:uuid:entry1</id>
    <link href="https://example.com/atom1"/>
    <updated>2024-01-01T10:00:00Z</updated>
    <summary>A summary</summary>
  </entry>
</feed>`

const sampleJSONFeed = `{
  "version": "https://jsonfeed.org/version/1.1",
  "title": "JSON Feed Test",
  "home_page_url": "https://example.com",
  "description": "A JSON feed",
  "items": [
    {
      "id": "json-item-1",
      "url": "https://example.com/json1",
      "title": "JSON Item 1",
      "summary": "A JSON item",
      "date_published": "2024-01-01T12:00:00Z",
      "tags": ["json", "feed"]
    }
  ]
}`

func TestDetectFeedType(t *testing.T) {
	if detectFeedType([]byte(sampleRSS)) != FeedRSS {
		t.Error("RSS sample should be detected as RSS")
	}
	if detectFeedType([]byte(sampleAtom)) != FeedAtom {
		t.Error("Atom sample should be detected as Atom")
	}
	if detectFeedType([]byte(sampleJSONFeed)) != FeedJSON {
		t.Error("JSON feed sample should be detected as JSON")
	}
	if detectFeedType([]byte("random text")) != FeedUnknown {
		t.Error("random text should be unknown")
	}
}

func TestParseRSS(t *testing.T) {
	p := NewParser()
	feed, err := p.ParseFromBytes([]byte(sampleRSS), "https://example.com/rss")
	if err != nil {
		t.Fatalf("ParseFromBytes RSS: %v", err)
	}
	if feed.Title != "Test Feed" {
		t.Errorf("Title = %q, want 'Test Feed'", feed.Title)
	}
	if len(feed.Items) != 2 {
		t.Errorf("Item count = %d, want 2", len(feed.Items))
	}
	if feed.Items[0].Title != "First Post" {
		t.Errorf("First item title = %q, want 'First Post'", feed.Items[0].Title)
	}
	if feed.Items[0].GUID != "post-001" {
		t.Errorf("First item GUID = %q, want 'post-001'", feed.Items[0].GUID)
	}
}

func TestParseAtom(t *testing.T) {
	p := NewParser()
	feed, err := p.ParseFromBytes([]byte(sampleAtom), "https://example.com/atom")
	if err != nil {
		t.Fatalf("ParseFromBytes Atom: %v", err)
	}
	if feed.Title != "Atom Test Feed" {
		t.Errorf("Title = %q, want 'Atom Test Feed'", feed.Title)
	}
	if len(feed.Items) != 1 {
		t.Errorf("Item count = %d, want 1", len(feed.Items))
	}
	if feed.Items[0].Title != "Atom Entry 1" {
		t.Errorf("Entry title = %q, want 'Atom Entry 1'", feed.Items[0].Title)
	}
}

func TestParseJSONFeed(t *testing.T) {
	p := NewParser()
	feed, err := p.ParseFromBytes([]byte(sampleJSONFeed), "https://example.com/feed.json")
	if err != nil {
		t.Fatalf("ParseFromBytes JSON: %v", err)
	}
	if feed.Title != "JSON Feed Test" {
		t.Errorf("Title = %q, want 'JSON Feed Test'", feed.Title)
	}
	if len(feed.Items) != 1 {
		t.Errorf("Item count = %d, want 1", len(feed.Items))
	}
	if feed.Items[0].GUID != "json-item-1" {
		t.Errorf("GUID = %q, want 'json-item-1'", feed.Items[0].GUID)
	}
}

func TestParseTime(t *testing.T) {
	cases := []string{
		"Mon, 01 Jan 2024 12:00:00 GMT",
		"2024-01-01T12:00:00Z",
		"2024-01-01T12:00:00+00:00",
		"2024-01-01",
	}
	for _, c := range cases {
		t_, err := parseTime(c)
		if err != nil {
			t.Errorf("parseTime(%q): %v", c, err)
		}
		_ = t_ // Just ensure it doesn't panic.
	}
}

func TestNewParser(t *testing.T) {
	p := NewParser()
	if p == nil {
		t.Fatal("NewParser returned nil")
	}
}

func TestFeedItemFields(t *testing.T) {
	item := FeedItem{
		Title:      "Test",
		Link:       "https://example.com/test",
		GUID:       "guid-1",
		Categories: []string{"cat1", "cat2"},
	}
	if item.Title != "Test" {
		t.Errorf("Title = %q, want 'Test'", item.Title)
	}
	if len(item.Categories) != 2 {
		t.Errorf("Categories count = %d, want 2", len(item.Categories))
	}
}
