// Package repository provides MongoDB data access for the crawler module.
package repository

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"gorm.io/gorm"

	"erg.ninja/internal/modules/crawler/entities"
	"erg.ninja/internal/persistence/postgrescore"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/rss"
)

// Repository provides data access for crawler entities.
type Repository struct {
	mongo         *database.MongoClient
	pg            *gorm.DB
	log           *logger.Logger
	feeds         *mongo.Collection
	history       *mongo.Collection
	legacyFeeds   *mongo.Collection
	legacyHistory *mongo.Collection
	fingerprints  *mongo.Collection
	blacklist     *mongo.Collection
	reputation    *mongo.Collection
}

// RepositoryOption configures the Repository.
type RepositoryOption func(*Repository)

// WithCrawlerRepositoryLogger sets the logger.
func WithCrawlerRepositoryLogger(log *logger.Logger) RepositoryOption {
	return func(r *Repository) { r.log = log }
}

// NewRepository creates a new crawler repository.
func NewRepository(mongo *database.MongoClient, opts ...RepositoryOption) *Repository {
	r := &Repository{
		mongo:         mongo,
		log:           logger.NoOp(),
		feeds:         mongo.Collection(entities.RSSFeedCollection),
		history:       mongo.Collection(entities.CrawlHistoryCollection),
		legacyFeeds:   mongo.Collection(legacyRSSFeedCollection),
		legacyHistory: mongo.Collection(legacyCrawlHistoryCollection),
		fingerprints:  mongo.Collection(entities.ContentFingerprintCollection),
		blacklist:     mongo.Collection(entities.ContentBlacklistCollection),
		reputation:    mongo.Collection(entities.DomainReputationCollection),
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

// WithCrawlerRepositoryPostgres sets the optional PostgreSQL handle for cross-module stats.
func WithCrawlerRepositoryPostgres(client *database.GORMPostgresClient) RepositoryOption {
	return func(r *Repository) {
		if client != nil {
			r.pg = client.DB()
		}
	}
}

// ─── RSS Feeds ───────────────────────────────────────────────────────────────

// CreateFeed inserts a new RSS feed.
func (r *Repository) CreateFeed(ctx context.Context, feed *entities.RSSFeed) error {
	if feed.ID == "" {
		feed.ID = database.NewID()
	}
	feed.CreatedAt = time.Now().UTC()
	feed.UpdatedAt = feed.CreatedAt
	feed.Enabled = true
	feed.ItemCount = 0
	feed.ErrorCount = 0

	_, err := r.feeds.InsertOne(ctx, feed)
	if err != nil {
		return fmt.Errorf("crawler.CreateFeed: %w", err)
	}
	return nil
}

// GetFeed retrieves a feed by ID.
func (r *Repository) GetFeed(ctx context.Context, id string) (*entities.RSSFeed, error) {
	var feed entities.RSSFeed
	err := r.feeds.FindOne(ctx, crawlerIDFilter(id)).Decode(&feed)
	if err != nil {
		if err != mongo.ErrNoDocuments {
			return nil, fmt.Errorf("crawler.GetFeed: %w", err)
		}
		var doc bson.M
		legacyErr := r.legacyFeeds.FindOne(ctx, crawlerIDFilter(id)).Decode(&doc)
		if legacyErr != nil {
			if legacyErr == mongo.ErrNoDocuments {
				return nil, nil
			}
			return nil, fmt.Errorf("crawler.GetFeed legacy: %w", legacyErr)
		}
		return normalizeLegacyFeedDoc(doc), nil
	}
	return &feed, nil
}

// ListFeeds returns all RSS feeds with optional filters.
func (r *Repository) ListFeeds(ctx context.Context, enabled *bool, category string) ([]*entities.RSSFeed, error) {
	filter := bson.M{}
	if enabled != nil {
		filter["enabled"] = *enabled
	}
	if category != "" {
		filter["category"] = category
	}

	cursor, err := r.feeds.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("crawler.ListFeeds: %w", err)
	}
	defer cursor.Close(ctx)

	var feeds []*entities.RSSFeed
	if err := cursor.All(ctx, &feeds); err != nil {
		return nil, fmt.Errorf("crawler.ListFeeds decode: %w", err)
	}

	legacyFilter := bson.M{}
	if enabled != nil {
		legacyFilter["isActive"] = *enabled
	}
	if category != "" {
		legacyFilter["targetCategoryId"] = category
	}
	legacyCursor, err := r.legacyFeeds.Find(ctx, legacyFilter, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("crawler.ListFeeds legacy: %w", err)
	}
	defer legacyCursor.Close(ctx)

	for legacyCursor.Next(ctx) {
		var doc bson.M
		if err := legacyCursor.Decode(&doc); err != nil {
			continue
		}
		if feed := normalizeLegacyFeedDoc(doc); feed != nil {
			feeds = append(feeds, feed)
		}
	}
	if err := legacyCursor.Err(); err != nil {
		return nil, fmt.Errorf("crawler.ListFeeds legacy decode: %w", err)
	}

	return dedupeFeeds(feeds), nil
}

// UpdateFeed updates a feed's metadata.
func (r *Repository) UpdateFeed(ctx context.Context, id string, update bson.M) error {
	update["updated_at"] = time.Now().UTC()
	result, err := r.feeds.UpdateOne(ctx, crawlerIDFilter(id), bson.M{"$set": update})
	if err != nil {
		return fmt.Errorf("crawler.UpdateFeed: %w", err)
	}
	if result.MatchedCount == 0 {
		legacyUpdate := bson.M{}
		if name, ok := update["name"]; ok {
			legacyUpdate["name"] = name
		}
		if category, ok := update["category"]; ok {
			legacyUpdate["targetCategoryId"] = category
		}
		if frequency, ok := update["frequency"]; ok {
			legacyUpdate["cronExpression"] = frequency
		}
		if enabled, ok := update["enabled"]; ok {
			legacyUpdate["isActive"] = enabled
		}
		legacyUpdate["updatedAt"] = time.Now().UTC()
		if len(legacyUpdate) == 1 {
			return nil
		}
		if _, err := r.legacyFeeds.UpdateOne(ctx, crawlerIDFilter(id), bson.M{"$set": legacyUpdate}); err != nil {
			return fmt.Errorf("crawler.UpdateFeed legacy: %w", err)
		}
	}
	return nil
}

// DeleteFeed removes a feed by ID.
func (r *Repository) DeleteFeed(ctx context.Context, id string) error {
	result, err := r.feeds.DeleteOne(ctx, crawlerIDFilter(id))
	if err != nil {
		return fmt.Errorf("crawler.DeleteFeed: %w", err)
	}
	if result.DeletedCount == 0 {
		if _, err := r.legacyFeeds.DeleteOne(ctx, crawlerIDFilter(id)); err != nil {
			return fmt.Errorf("crawler.DeleteFeed legacy: %w", err)
		}
	}
	return nil
}

// UpdateFeedLastItem updates a feed's last_fetch_at and last_item_at timestamps.
func (r *Repository) UpdateFeedLastItem(ctx context.Context, feedID string) error {
	now := time.Now().UTC()
	update := bson.M{
		"$set": bson.M{
			"last_fetch_at": now,
			"last_item_at":  now,
			"updated_at":    now,
		},
		"$inc": bson.M{
			"item_count": 1,
		},
	}
	_, err := r.feeds.UpdateOne(ctx, bson.M{"_id": feedID}, update)
	if err != nil {
		return fmt.Errorf("crawler.UpdateFeedLastItem: %w", err)
	}
	return nil
}

// UpdateFeedLastFetch updates the feed's last_fetch_at timestamp and item counts.
// Called by the refresh-feed job after fetching a feed.
func (r *Repository) UpdateFeedLastFetch(ctx context.Context, feedID string, fetchedAt time.Time, totalItems, newItems int) error {
	update := bson.M{
		"$set": bson.M{
			"last_fetch_at": fetchedAt,
			"updated_at":    fetchedAt,
		},
		"$inc": bson.M{
			"item_count": newItems,
		},
	}
	if newItems == 0 {
		update["$set"].(bson.M)["last_item_at"] = fetchedAt
	}
	_, err := r.feeds.UpdateOne(ctx, bson.M{"_id": feedID}, update)
	if err != nil {
		return fmt.Errorf("crawler.UpdateFeedLastFetch: %w", err)
	}
	return nil
}

// UpdateFeedError increments the feed's error count and records the last error.
func (r *Repository) UpdateFeedError(ctx context.Context, feedID string, errMsg string) error {
	update := bson.M{
		"$set": bson.M{
			"last_error": errMsg,
			"updated_at": time.Now().UTC(),
		},
		"$inc": bson.M{
			"error_count": 1,
		},
	}
	_, err := r.feeds.UpdateOne(ctx, bson.M{"_id": feedID}, update)
	if err != nil {
		return fmt.Errorf("crawler.UpdateFeedError: %w", err)
	}
	return nil
}

// URLExists checks if a URL has already been crawled (exists in crawl_history).
func (r *Repository) URLExists(ctx context.Context, url string) (bool, error) {
	if url == "" {
		return false, nil
	}
	count, err := r.history.CountDocuments(ctx, bson.M{"url": url})
	if err != nil {
		return false, fmt.Errorf("crawler.URLExists: %w", err)
	}
	legacyCount, err := r.legacyHistory.CountDocuments(ctx, bson.M{"url": url})
	if err != nil {
		return false, fmt.Errorf("crawler.URLExists legacy: %w", err)
	}
	return count+legacyCount > 0, nil
}

// RecordFeedItem records a feed item for a feed (tracks feed items to know what's new).
// Stores minimal metadata: url, feed_id, published_at.
func (r *Repository) RecordFeedItem(ctx context.Context, feedID string, item *rss.FeedItem) error {
	doc := bson.M{
		"_id":        database.NewID(),
		"feed_id":    feedID,
		"url":        item.Link,
		"title":      item.Title,
		"published":  item.PubDate,
		"created_at": time.Now().UTC(),
	}
	// Use upsert to avoid duplicates if same URL appears in multiple fetches.
	_, err := r.history.UpdateOne(ctx,
		bson.M{"url": item.Link},
		bson.M{"$setOnInsert": doc},
		options.UpdateOne().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("crawler.RecordFeedItem: %w", err)
	}
	return nil
}

// CreateHistoryID generates a new ID for crawl history.
func (r *Repository) CreateHistoryID() string {
	return database.NewID()
}

// GetEnabledFeeds returns all enabled feeds.
func (r *Repository) GetEnabledFeeds(ctx context.Context) ([]*entities.RSSFeed, error) {
	return r.ListFeeds(ctx, ptrBool(true), "")
}

// ─── Crawl History ───────────────────────────────────────────────────────────

// CreateCrawlHistory inserts a new crawl history record.
func (r *Repository) CreateCrawlHistory(ctx context.Context, h *entities.CrawlHistory) error {
	if h.ID == "" {
		h.ID = database.NewID()
	}
	h.CreatedAt = time.Now().UTC()
	h.UpdatedAt = h.CreatedAt

	_, err := r.history.InsertOne(ctx, h)
	if err != nil {
		return fmt.Errorf("crawler.CreateCrawlHistory: %w", err)
	}
	return nil
}

// GetCrawlHistory retrieves a crawl record by ID.
func (r *Repository) GetCrawlHistory(ctx context.Context, id string) (*entities.CrawlHistory, error) {
	var h entities.CrawlHistory
	err := r.history.FindOne(ctx, crawlerIDFilter(id)).Decode(&h)
	if err != nil {
		if err != mongo.ErrNoDocuments {
			return nil, fmt.Errorf("crawler.GetCrawlHistory: %w", err)
		}
		var doc bson.M
		legacyErr := r.legacyHistory.FindOne(ctx, crawlerIDFilter(id)).Decode(&doc)
		if legacyErr != nil {
			if legacyErr == mongo.ErrNoDocuments {
				return nil, nil
			}
			return nil, fmt.Errorf("crawler.GetCrawlHistory legacy: %w", legacyErr)
		}
		return normalizeLegacyHistoryDoc(doc), nil
	}
	return &h, nil
}

// GetCrawlHistoryByJobID retrieves a crawl record by job_id (UUID assigned by controller).
func (r *Repository) GetCrawlHistoryByJobID(ctx context.Context, jobID string) (*entities.CrawlHistory, error) {
	var h entities.CrawlHistory
	err := r.history.FindOne(ctx, bson.M{"job_id": jobID}).Decode(&h)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("crawler.GetCrawlHistoryByJobID: %w", err)
	}
	return &h, nil
}

// ListCrawlHistoryParams controls listing behavior.
type ListCrawlHistoryParams struct {
	FeedID string
	Status entities.CrawlStatus
	Limit  int64
	Offset int64
}

// ListCrawlHistory returns crawl history with pagination.
func (r *Repository) ListCrawlHistory(ctx context.Context, p ListCrawlHistoryParams) ([]*entities.CrawlHistory, int64, error) {
	if p.Limit <= 0 {
		p.Limit = 20
	}

	currentFilter := bson.M{}
	if p.FeedID != "" {
		currentFilter["feed_id"] = p.FeedID
	}
	if p.Status != "" {
		currentFilter["status"] = p.Status
	}
	cursor, err := r.history.Find(ctx, currentFilter, options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}))
	if err != nil {
		return nil, 0, fmt.Errorf("crawler.ListCrawlHistory: %w", err)
	}
	defer cursor.Close(ctx)

	var results []*entities.CrawlHistory
	if err := cursor.All(ctx, &results); err != nil {
		return nil, 0, fmt.Errorf("crawler.ListCrawlHistory decode: %w", err)
	}

	legacyFilter := bson.M{}
	if p.FeedID != "" {
		legacyFilter["sourceId"] = p.FeedID
	}
	if p.Status != "" {
		legacyFilter["status"] = strings.ToUpper(string(p.Status))
	}
	legacyCursor, err := r.legacyHistory.Find(ctx, legacyFilter, options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}))
	if err != nil {
		return nil, 0, fmt.Errorf("crawler.ListCrawlHistory legacy: %w", err)
	}
	defer legacyCursor.Close(ctx)

	for legacyCursor.Next(ctx) {
		var doc bson.M
		if err := legacyCursor.Decode(&doc); err != nil {
			continue
		}
		if history := normalizeLegacyHistoryDoc(doc); history != nil {
			results = append(results, history)
		}
	}
	if err := legacyCursor.Err(); err != nil {
		return nil, 0, fmt.Errorf("crawler.ListCrawlHistory legacy decode: %w", err)
	}

	sort.Slice(results, func(i, j int) bool {
		left := results[i].CrawledAt
		if left.IsZero() {
			left = results[i].CreatedAt
		}
		right := results[j].CrawledAt
		if right.IsZero() {
			right = results[j].CreatedAt
		}
		return left.After(right)
	})

	total := int64(len(results))
	if p.Offset >= total {
		return []*entities.CrawlHistory{}, total, nil
	}
	end := p.Offset + p.Limit
	if end > total {
		end = total
	}
	return results[int(p.Offset):int(end)], total, nil
}

// UpdateCrawlStatus updates the status and step of a crawl record.
func (r *Repository) UpdateCrawlStatus(ctx context.Context, id string, status entities.CrawlStatus, step int, errMsg string) error {
	update := bson.M{
		"$set": bson.M{
			"status":     status,
			"step":       step,
			"error_msg":  errMsg,
			"updated_at": time.Now().UTC(),
		},
	}
	if status == entities.CrawlStatusSuccess {
		now := time.Now().UTC()
		update["$set"].(bson.M)["crawled_at"] = now
	}

	_, err := r.history.UpdateOne(ctx, bson.M{"_id": id}, update)
	if err != nil {
		return fmt.Errorf("crawler.UpdateCrawlStatus: %w", err)
	}
	return nil
}

// UpdateCrawlMetadata updates quality score, title, description after crawl.
func (r *Repository) UpdateCrawlMetadata(ctx context.Context, id string, title, description string, qualityScore float64, httpStatus int, respSize int64, durationMS int64) error {
	update := bson.M{
		"$set": bson.M{
			"status":        entities.CrawlStatusSuccess,
			"title":         title,
			"description":   description,
			"quality_score": qualityScore,
			"http_status":   httpStatus,
			"response_size": respSize,
			"duration_ms":   durationMS,
			"updated_at":    time.Now().UTC(),
			"crawled_at":    time.Now().UTC(),
		},
	}
	_, err := r.history.UpdateOne(ctx, bson.M{"_id": id}, update)
	if err != nil {
		return fmt.Errorf("crawler.UpdateCrawlMetadata: %w", err)
	}
	return nil
}

// ─── Fingerprints ───────────────────────────────────────────────────────────────

// StoreFingerprint stores a content fingerprint.
func (r *Repository) StoreFingerprint(ctx context.Context, fp *entities.ContentFingerprint) error {
	if fp.ID == "" {
		fp.ID = database.NewID()
	}
	fp.Bucket = uint16(fp.SimHash >> 48)
	fp.CrawledAt = time.Now().UTC()

	_, err := r.fingerprints.InsertOne(ctx, fp)
	if err != nil {
		if database.IsDuplicateKey(err) {
			return nil // already stored
		}
		return fmt.Errorf("crawler.StoreFingerprint: %w", err)
	}
	return nil
}

// FindFingerprintBySHA256 finds a fingerprint by exact SHA-256 match.
func (r *Repository) FindFingerprintBySHA256(ctx context.Context, sha256 string) (*entities.ContentFingerprint, error) {
	var fp entities.ContentFingerprint
	err := r.fingerprints.FindOne(ctx, bson.M{"sha256": sha256}).Decode(&fp)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("crawler.FindFingerprintBySHA256: %w", err)
	}
	return &fp, nil
}

// FetchFingerprintsByBucket retrieves fingerprints in a bucket for dedup comparison.
func (r *Repository) FetchFingerprintsByBucket(ctx context.Context, bucket uint16, limit int) ([]entities.ContentFingerprint, error) {
	filter := bson.M{"bucket": bucket}
	opts := options.Find().SetLimit(int64(limit))

	cursor, err := r.fingerprints.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("crawler.FetchFingerprintsByBucket: %w", err)
	}
	defer cursor.Close(ctx)

	var results []entities.ContentFingerprint
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("crawler.FetchFingerprintsByBucket decode: %w", err)
	}
	return results, nil
}

// ─── Blacklist ───────────────────────────────────────────────────────────────

// CreateBlacklistEntry adds a URL/domain/keyword to the blacklist.
func (r *Repository) CreateBlacklistEntry(ctx context.Context, entry *entities.ContentBlacklist) error {
	if entry.ID == "" {
		entry.ID = database.NewID()
	}
	entry.CreatedAt = time.Now().UTC()
	entry.UpdatedAt = entry.CreatedAt
	entry.BlockedCount = 0
	entry.Enabled = true

	_, err := r.blacklist.InsertOne(ctx, entry)
	if err != nil {
		return fmt.Errorf("crawler.CreateBlacklistEntry: %w", err)
	}
	return nil
}

// ListBlacklist returns all blacklist entries.
func (r *Repository) ListBlacklist(ctx context.Context, blType *entities.BlacklistType, enabled *bool) ([]*entities.ContentBlacklist, error) {
	filter := bson.M{}
	if blType != nil {
		filter["type"] = *blType
	}
	if enabled != nil {
		filter["enabled"] = *enabled
	}

	cursor, err := r.blacklist.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("crawler.ListBlacklist: %w", err)
	}
	defer cursor.Close(ctx)

	var entries []*entities.ContentBlacklist
	if err := cursor.All(ctx, &entries); err != nil {
		return nil, fmt.Errorf("crawler.ListBlacklist decode: %w", err)
	}
	return entries, nil
}

// DeleteBlacklistEntry removes a blacklist entry.
func (r *Repository) DeleteBlacklistEntry(ctx context.Context, id string) error {
	_, err := r.blacklist.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("crawler.DeleteBlacklistEntry: %w", err)
	}
	return nil
}

// GetBlacklistEntry retrieves a blacklist entry by ID.
func (r *Repository) GetBlacklistEntry(ctx context.Context, id string) (*entities.ContentBlacklist, error) {
	var entry entities.ContentBlacklist
	err := r.blacklist.FindOne(ctx, bson.M{"_id": id}).Decode(&entry)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("crawler.GetBlacklistEntry: %w", err)
	}
	return &entry, nil
}

// UpdateBlacklistEntry updates mutable blacklist fields.
func (r *Repository) UpdateBlacklistEntry(ctx context.Context, id string, update bson.M) error {
	if len(update) == 0 {
		return nil
	}
	update["updated_at"] = time.Now().UTC()
	_, err := r.blacklist.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": update})
	if err != nil {
		return fmt.Errorf("crawler.UpdateBlacklistEntry: %w", err)
	}
	return nil
}

// IncrementBlacklistCount increments the blocked count for a blacklist entry.
func (r *Repository) IncrementBlacklistCount(ctx context.Context, pattern string) error {
	_, err := r.blacklist.UpdateOne(ctx,
		bson.M{"pattern": pattern, "type": entities.BlacklistURL},
		bson.M{"$inc": bson.M{"blocked_count": 1}, "$set": bson.M{"updated_at": time.Now().UTC()}},
	)
	if err != nil {
		return fmt.Errorf("crawler.IncrementBlacklistCount: %w", err)
	}
	return nil
}

// CountFingerprints returns the number of stored content fingerprints.
func (r *Repository) CountFingerprints(ctx context.Context) (int64, error) {
	count, err := r.fingerprints.CountDocuments(ctx, bson.M{})
	if err != nil {
		return 0, fmt.Errorf("crawler.CountFingerprints: %w", err)
	}
	return count, nil
}

// IsURLBlocked checks if a URL is blocked by the blacklist.
func (r *Repository) IsURLBlocked(ctx context.Context, rawURL string) (bool, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false, fmt.Errorf("crawler.IsURLBlocked parse: %w", err)
	}
	host := strings.ToLower(parsed.Hostname())
	path := rawURL

	// Check URL exact match.
	count, err := r.blacklist.CountDocuments(ctx, bson.M{
		"type":    entities.BlacklistURL,
		"pattern": path,
		"enabled": true,
	})
	if err != nil {
		return false, fmt.Errorf("crawler.IsURLBlocked url check: %w", err)
	}
	if count > 0 {
		return true, nil
	}

	// Check domain match.
	count, err = r.blacklist.CountDocuments(ctx, bson.M{
		"type":    entities.BlacklistDomain,
		"pattern": host,
		"enabled": true,
	})
	if err != nil {
		return false, fmt.Errorf("crawler.IsURLBlocked domain check: %w", err)
	}
	if count > 0 {
		return true, nil
	}

	return false, nil
}

// ─── Domain Reputation ───────────────────────────────────────────────────────

// GetDomainReputation retrieves or creates a reputation record for a domain.
func (r *Repository) GetDomainReputation(ctx context.Context, domain string) (*entities.DomainReputation, error) {
	var rep entities.DomainReputation
	err := r.reputation.FindOne(ctx, bson.M{"domain": domain}).Decode(&rep)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("crawler.GetDomainReputation: %w", err)
	}
	return &rep, nil
}

// UpdateDomainReputation records a crawl result for a domain.
func (r *Repository) UpdateDomainReputation(ctx context.Context, domain string, success bool, durationMS int64, blocked bool) error {
	now := time.Now().UTC()
	filter := bson.M{"domain": domain}

	rep, err := r.GetDomainReputation(ctx, domain)
	if err != nil {
		return err
	}

	if rep == nil {
		// Create new record.
		rep = &entities.DomainReputation{
			ID:           database.NewID(),
			Domain:       domain,
			BlockCount:   0,
			SuccessCount: 0,
			FailCount:    0,
			AvgDuration:  durationMS,
			Score:        100,
			LastSeenAt:   now,
			UpdatedAt:    now,
		}
		if blocked {
			rep.BlockCount = 1
			rep.Score = 0
		} else if success {
			rep.SuccessCount = 1
		} else {
			rep.FailCount = 1
		}
		_, err := r.reputation.InsertOne(ctx, rep)
		return err
	}

	// Update existing record.
	avg := (rep.AvgDuration*int64(rep.SuccessCount+rep.FailCount) + durationMS) / int64(rep.SuccessCount+rep.FailCount+1)
	blockInc := 0
	successInc := 0
	failInc := 0
	if blocked {
		blockInc = 1
	} else if success {
		successInc = 1
	} else {
		failInc = 1
	}

	// Score: reduce by 10 per block, increase by 1 per success.
	newScore := rep.Score - float64(blockInc)*10 + float64(successInc)*1
	if newScore < 0 {
		newScore = 0
	}
	if newScore > 100 {
		newScore = 100
	}

	update := bson.M{
		"$set": bson.M{
			"avg_duration_ms": avg,
			"score":           newScore,
			"last_seen_at":    now,
			"updated_at":      now,
		},
		"$inc": bson.M{
			"block_count":   int64(blockInc),
			"success_count": int64(successInc),
			"fail_count":    int64(failInc),
		},
	}
	_, err = r.reputation.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("crawler.UpdateDomainReputation: %w", err)
	}
	return nil
}

// ShouldSkipDomain returns true if a domain should be skipped (score too low).
func (r *Repository) ShouldSkipDomain(ctx context.Context, domain string, threshold float64) (bool, error) {
	rep, err := r.GetDomainReputation(ctx, domain)
	if err != nil {
		return false, err
	}
	if rep == nil {
		return false, nil
	}
	return rep.Score < threshold, nil
}

// GetCrawlHistoryBatch returns a batch of crawl history entries ordered by _id for reindexing.
// Uses cursor-based pagination with lastID as the starting point.
func (r *Repository) GetCrawlHistoryBatch(ctx context.Context, limit int, lastID string) ([]*entities.CrawlHistory, error) {
	filter := bson.M{}
	if lastID != "" {
		filter["_id"] = bson.M{"$gt": lastID}
	}
	opts := options.Find().
		SetSort(bson.D{{Key: "_id", Value: 1}}).
		SetLimit(int64(limit)).
		SetProjection(bson.D{
			{Key: "_id", Value: 1},
			{Key: "url", Value: 1},
			{Key: "content_hash", Value: 1},
			{Key: "status", Value: 1},
		})
	cursor, err := r.history.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("crawler.GetCrawlHistoryBatch: %w", err)
	}
	defer cursor.Close(ctx)

	var entries []*entities.CrawlHistory
	if err := cursor.All(ctx, &entries); err != nil {
		return nil, fmt.Errorf("crawler.GetCrawlHistoryBatch: decode: %w", err)
	}
	return entries, nil
}

// CrawlStats holds aggregated crawl statistics.
type CrawlStats struct {
	TotalCrawled    int64   `bson:"total_crawled"`
	TotalSuccess    int64   `bson:"total_success"`
	TotalFailed     int64   `bson:"total_failed"`
	TotalDuplicate  int64   `bson:"total_duplicate"`
	PassRate        float64 `bson:"pass_rate"`
	AvgQualityScore float64 `bson:"avg_quality_score"`
}

// DashboardStats holds frontend-oriented crawler dashboard metrics.
type DashboardStats struct {
	TotalRss        int64
	TotalConfigs    int64
	TotalHistory    int64
	SuccessCrawl    int64
	FailedCrawl     int64
	TotalPosts      int64
	TotalCategories int64
	AvgQualityScore float64
	PassRate        float64
}

// GetCrawlStats returns aggregated crawl statistics for all time.
func (r *Repository) GetCrawlStats(ctx context.Context) (*CrawlStats, error) {
	stats := &CrawlStats{}
	histories, _, err := r.ListCrawlHistory(ctx, ListCrawlHistoryParams{Limit: 1_000_000})
	if err != nil {
		return nil, fmt.Errorf("crawler.GetCrawlStats: %w", err)
	}
	var totalScore float64
	var scoredCount int64
	for _, history := range histories {
		stats.TotalCrawled++
		switch history.Status {
		case entities.CrawlStatusSuccess:
			stats.TotalSuccess++
			if history.QualityScore > 0 {
				totalScore += history.QualityScore
				scoredCount++
			}
		case entities.CrawlStatusFailed:
			stats.TotalFailed++
		case entities.CrawlStatusDuplicate:
			stats.TotalDuplicate++
		}
	}
	if stats.TotalCrawled > 0 {
		stats.PassRate = float64(stats.TotalSuccess) / float64(stats.TotalCrawled)
	}
	if scoredCount > 0 {
		stats.AvgQualityScore = totalScore / float64(scoredCount)
	}
	return stats, nil
}

// GetDashboardStats returns the aggregate metrics used by the admin crawler dashboard.
func (r *Repository) GetDashboardStats(ctx context.Context) (*DashboardStats, error) {
	enabled := true
	feeds, err := r.ListFeeds(ctx, &enabled, "")
	if err != nil {
		return nil, fmt.Errorf("crawler.GetDashboardStats feeds: %w", err)
	}
	configs, err := r.ListScraperConfigs(ctx)
	if err != nil {
		return nil, fmt.Errorf("crawler.GetDashboardStats configs: %w", err)
	}
	crawlStats, err := r.GetCrawlStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("crawler.GetDashboardStats crawl stats: %w", err)
	}
	postsCount, categoryCount, err := r.countContentStats(ctx)
	if err != nil {
		return nil, err
	}

	return &DashboardStats{
		TotalRss:        int64(len(feeds)),
		TotalConfigs:    int64(len(configs)),
		TotalHistory:    crawlStats.TotalCrawled,
		SuccessCrawl:    crawlStats.TotalSuccess,
		FailedCrawl:     crawlStats.TotalFailed,
		TotalPosts:      postsCount,
		TotalCategories: categoryCount,
		AvgQualityScore: crawlStats.AvgQualityScore,
		PassRate:        crawlStats.PassRate,
	}, nil
}

func (r *Repository) countContentStats(ctx context.Context) (int64, int64, error) {
	if r.pg != nil {
		var postsCount int64
		if err := r.pg.WithContext(ctx).Model(&postgrescore.Post{}).Count(&postsCount).Error; err != nil {
			return 0, 0, fmt.Errorf("crawler.GetDashboardStats posts: %w", err)
		}
		var categoryCount int64
		if err := r.pg.WithContext(ctx).Model(&postgrescore.PostCategory{}).Count(&categoryCount).Error; err != nil {
			return 0, 0, fmt.Errorf("crawler.GetDashboardStats categories: %w", err)
		}
		return postsCount, categoryCount, nil
	}

	postsCount, err := r.mongo.Collection("posts").CountDocuments(ctx, bson.M{})
	if err != nil {
		return 0, 0, fmt.Errorf("crawler.GetDashboardStats posts: %w", err)
	}
	categoryCount, err := r.mongo.Collection("post_categories").CountDocuments(ctx, bson.M{})
	if err != nil {
		return 0, 0, fmt.Errorf("crawler.GetDashboardStats categories: %w", err)
	}
	return postsCount, categoryCount, nil
}

// ─── Scraper Configs ─────────────────────────────────────────────────────────

// ListScraperConfigs returns all scraper configs.
func (r *Repository) ListScraperConfigs(ctx context.Context) ([]map[string]any, error) {
	currentColl := r.mongo.Collection(currentScraperConfigCollection)
	cursor, err := currentColl.Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("crawler.ListScraperConfigs: %w", err)
	}
	defer cursor.Close(ctx)

	var results []map[string]any
	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
		if cfg := normalizeConfigDoc(doc); cfg != nil {
			results = append(results, cfg)
		}
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("crawler.ListScraperConfigs decode: %w", err)
	}

	legacyColl := r.mongo.Collection(legacyScraperConfigCollection)
	legacyCursor, err := legacyColl.Find(ctx, bson.M{})
	if err != nil {
		return nil, fmt.Errorf("crawler.ListScraperConfigs legacy: %w", err)
	}
	defer legacyCursor.Close(ctx)

	for legacyCursor.Next(ctx) {
		var doc bson.M
		if err := legacyCursor.Decode(&doc); err != nil {
			continue
		}
		if cfg := normalizeConfigDoc(doc); cfg != nil {
			results = append(results, cfg)
		}
	}
	if err := legacyCursor.Err(); err != nil {
		return nil, fmt.Errorf("crawler.ListScraperConfigs legacy decode: %w", err)
	}
	if results == nil {
		results = []map[string]any{}
	}
	return dedupeConfigs(results), nil
}

// CreateScraperConfig inserts a new scraper config and returns the ID.
func (r *Repository) CreateScraperConfig(ctx context.Context, data map[string]any) (string, error) {
	coll := r.mongo.Collection(currentScraperConfigCollection)
	id := database.NewID()
	data["_id"] = id
	data["created_at"] = time.Now().UTC()
	data["updated_at"] = data["created_at"]

	_, err := coll.InsertOne(ctx, data)
	if err != nil {
		return "", fmt.Errorf("crawler.CreateScraperConfig: %w", err)
	}
	return id, nil
}

// UpdateScraperConfig updates an existing scraper config.
func (r *Repository) UpdateScraperConfig(ctx context.Context, id string, data map[string]any) error {
	coll := r.mongo.Collection(currentScraperConfigCollection)
	data["updated_at"] = time.Now().UTC()
	delete(data, "_id") // remove _id from $set to avoid immutable field error

	result, err := coll.UpdateOne(ctx, crawlerIDFilter(id), bson.M{"$set": data})
	if err != nil {
		return fmt.Errorf("crawler.UpdateScraperConfig: %w", err)
	}
	if result.MatchedCount == 0 {
		legacyUpdate := bson.M{}
		for key, value := range data {
			switch key {
			case "selector_config":
				legacyUpdate["selectorConfig"] = value
			default:
				legacyUpdate[key] = value
			}
		}
		legacyUpdate["updatedAt"] = time.Now().UTC()
		if _, err := r.mongo.Collection(legacyScraperConfigCollection).UpdateOne(ctx, crawlerIDFilter(id), bson.M{"$set": legacyUpdate}); err != nil {
			return fmt.Errorf("crawler.UpdateScraperConfig legacy: %w", err)
		}
	}
	return nil
}

// DeleteScraperConfig removes a scraper config by ID.
func (r *Repository) DeleteScraperConfig(ctx context.Context, id string) error {
	coll := r.mongo.Collection(currentScraperConfigCollection)
	result, err := coll.DeleteOne(ctx, crawlerIDFilter(id))
	if err != nil {
		return fmt.Errorf("crawler.DeleteScraperConfig: %w", err)
	}
	if result.DeletedCount == 0 {
		if _, err := r.mongo.Collection(legacyScraperConfigCollection).DeleteOne(ctx, crawlerIDFilter(id)); err != nil {
			return fmt.Errorf("crawler.DeleteScraperConfig legacy: %w", err)
		}
	}
	return nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func ptrBool(b bool) *bool { return &b }
