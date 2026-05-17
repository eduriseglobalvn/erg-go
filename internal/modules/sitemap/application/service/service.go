package service

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"erg.ninja/internal/persistence/postgrescore"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
)

// Service provides sitemap generating logic.
type Service struct {
	db  *database.GORMPostgresClient
	log *logger.Logger
}

// NewService creates a new sitemap service.
func NewService(db *database.GORMPostgresClient, log *logger.Logger) *Service {
	return &Service{
		db:  db,
		log: log,
	}
}

// GenerateSitemapIndex returns an XML index of sitemaps.
func (s *Service) GenerateSitemapIndex(ctx context.Context, publicDomain string) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	buf.WriteString("<sitemapindex xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">\n")

	// Standard posts sitemap
	buf.WriteString(fmt.Sprintf("  <sitemap>\n    <loc>%s/sitemap.xml</loc>\n  </sitemap>\n", publicDomain))

	// If there are more subdomains or pagination, we can add them here
	buf.WriteString("</sitemapindex>")
	return buf.Bytes(), nil
}

// GenerateSitemap returns the main XML sitemap containing posts URLs.
func (s *Service) GenerateSitemap(ctx context.Context, publicDomain string) ([]byte, error) {
	if s.db == nil {
		return nil, fmt.Errorf("failed to fetch posts for sitemap: postgres client unavailable")
	}

	var posts []postgrescore.Post
	if err := s.db.DB().WithContext(ctx).
		Model(&postgrescore.Post{}).
		Select("slug", "updated_at").
		Where("deleted_at IS NULL AND is_published = ?", true).
		Order("created_at DESC").
		Limit(50000).
		Find(&posts).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch posts for sitemap: %w", err)
	}

	var buf bytes.Buffer
	buf.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	buf.WriteString("<urlset xmlns=\"http://www.sitemaps.org/schemas/sitemap/0.9\">\n")

	for _, post := range posts {
		if post.Slug == "" {
			continue
		}

		lastMod := time.Now()
		if !post.UpdatedAt.IsZero() {
			lastMod = post.UpdatedAt
		}

		buf.WriteString("  <url>\n")
		buf.WriteString(fmt.Sprintf("    <loc>%s/%s</loc>\n", publicDomain, post.Slug))
		buf.WriteString(fmt.Sprintf("    <lastmod>%s</lastmod>\n", lastMod.Format("2006-01-02")))
		buf.WriteString("    <changefreq>weekly</changefreq>\n")
		buf.WriteString("    <priority>0.8</priority>\n")
		buf.WriteString("  </url>\n")
	}

	buf.WriteString("</urlset>")
	return buf.Bytes(), nil
}

// GenerateRobotsTxt returns the contents for robots.txt.
func (s *Service) GenerateRobotsTxt(publicDomain string) ([]byte, error) {
	content := fmt.Sprintf("User-agent: *\nAllow: /\n\nSitemap: %s/sitemap_index.xml\n", publicDomain)
	return []byte(content), nil
}
