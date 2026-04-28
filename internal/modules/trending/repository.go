package trending

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"erg.ninja/internal/modules/crawler/entities"
	trendingentities "erg.ninja/internal/modules/trending/entities"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
)

type Repository struct {
	log          *logger.Logger
	topics       *mongo.Collection
	news         *mongo.Collection
	snapshots    *mongo.Collection
	crawlHistory *mongo.Collection
	rssFeeds     *mongo.Collection
}

type RepositoryOption func(*Repository)

func WithRepositoryLogger(log *logger.Logger) RepositoryOption {
	return func(r *Repository) { r.log = log }
}

func NewRepository(mongoClient *database.MongoClient, opts ...RepositoryOption) *Repository {
	r := &Repository{
		log:          logger.NoOp(),
		topics:       mongoClient.Collection(trendingentities.TrendingTopicCollection),
		news:         mongoClient.Collection(trendingentities.NewsArticleCollection),
		snapshots:    mongoClient.Collection(trendingentities.TrendingSnapshotCollection),
		crawlHistory: mongoClient.Collection(entities.CrawlHistoryCollection),
		rssFeeds:     mongoClient.Collection(entities.RSSFeedCollection),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func (r *Repository) ReplaceTopics(ctx context.Context, topics []*trendingentities.TrendingTopic) error {
	if _, err := r.topics.DeleteMany(ctx, bson.M{}); err != nil {
		return fmt.Errorf("trending.ReplaceTopics delete: %w", err)
	}
	if len(topics) == 0 {
		return nil
	}
	docs := make([]any, 0, len(topics))
	now := time.Now().UTC()
	for _, topic := range topics {
		if topic.ID == "" {
			topic.ID = database.NewID()
		}
		if topic.CreatedAt.IsZero() {
			topic.CreatedAt = now
		}
		topic.UpdatedAt = now
		if topic.LastRefreshedAt.IsZero() {
			topic.LastRefreshedAt = now
		}
		docs = append(docs, topic)
	}
	if _, err := r.topics.InsertMany(ctx, docs); err != nil {
		return fmt.Errorf("trending.ReplaceTopics insert: %w", err)
	}
	return nil
}

func (r *Repository) ReplaceNews(ctx context.Context, articles []*trendingentities.NewsArticle) error {
	if _, err := r.news.DeleteMany(ctx, bson.M{}); err != nil {
		return fmt.Errorf("trending.ReplaceNews delete: %w", err)
	}
	if len(articles) == 0 {
		return nil
	}
	docs := make([]any, 0, len(articles))
	now := time.Now().UTC()
	for _, article := range articles {
		if article.ID == "" {
			article.ID = database.NewID()
		}
		if article.CreatedAt.IsZero() {
			article.CreatedAt = now
		}
		article.UpdatedAt = now
		docs = append(docs, article)
	}
	if _, err := r.news.InsertMany(ctx, docs); err != nil {
		return fmt.Errorf("trending.ReplaceNews insert: %w", err)
	}
	return nil
}

func (r *Repository) CreateSnapshot(ctx context.Context, snapshot *trendingentities.TrendingSnapshot) error {
	if snapshot.ID == "" {
		snapshot.ID = database.NewID()
	}
	if snapshot.CreatedAt.IsZero() {
		snapshot.CreatedAt = time.Now().UTC()
	}
	if snapshot.GeneratedAt.IsZero() {
		snapshot.GeneratedAt = snapshot.CreatedAt
	}
	_, err := r.snapshots.InsertOne(ctx, snapshot)
	if err != nil {
		return fmt.Errorf("trending.CreateSnapshot: %w", err)
	}
	return nil
}

func (r *Repository) ListTopics(ctx context.Context, limit int64) ([]*trendingentities.TrendingTopic, error) {
	if limit <= 0 {
		limit = 20
	}
	cursor, err := r.topics.Find(ctx, bson.M{}, options.Find().
		SetSort(bson.D{{Key: "score", Value: -1}, {Key: "volume", Value: -1}}).
		SetLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("trending.ListTopics: %w", err)
	}
	defer cursor.Close(ctx)

	var out []*trendingentities.TrendingTopic
	if err := cursor.All(ctx, &out); err != nil {
		return nil, fmt.Errorf("trending.ListTopics decode: %w", err)
	}
	return out, nil
}

func (r *Repository) GetTopic(ctx context.Context, slug string) (*trendingentities.TrendingTopic, error) {
	var topic trendingentities.TrendingTopic
	err := r.topics.FindOne(ctx, bson.M{"slug": slug}).Decode(&topic)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("trending.GetTopic: %w", err)
	}
	return &topic, nil
}

func (r *Repository) ListNews(ctx context.Context, limit int64) ([]*trendingentities.NewsArticle, error) {
	if limit <= 0 {
		limit = 20
	}
	cursor, err := r.news.Find(ctx, bson.M{}, options.Find().
		SetSort(bson.D{{Key: "relevance_score", Value: -1}, {Key: "published_at", Value: -1}}).
		SetLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("trending.ListNews: %w", err)
	}
	defer cursor.Close(ctx)

	var out []*trendingentities.NewsArticle
	if err := cursor.All(ctx, &out); err != nil {
		return nil, fmt.Errorf("trending.ListNews decode: %w", err)
	}
	return out, nil
}

func (r *Repository) ListSnapshots(ctx context.Context, limit int64) ([]*trendingentities.TrendingSnapshot, error) {
	if limit <= 0 {
		limit = 20
	}
	cursor, err := r.snapshots.Find(ctx, bson.M{}, options.Find().
		SetSort(bson.D{{Key: "generated_at", Value: -1}}).
		SetLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("trending.ListSnapshots: %w", err)
	}
	defer cursor.Close(ctx)

	var out []*trendingentities.TrendingSnapshot
	if err := cursor.All(ctx, &out); err != nil {
		return nil, fmt.Errorf("trending.ListSnapshots decode: %w", err)
	}
	return out, nil
}

func (r *Repository) LatestSnapshotTime(ctx context.Context) (time.Time, error) {
	var snapshot trendingentities.TrendingSnapshot
	err := r.snapshots.FindOne(ctx, bson.M{}, options.FindOne().
		SetSort(bson.D{{Key: "generated_at", Value: -1}})).
		Decode(&snapshot)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return time.Time{}, nil
		}
		return time.Time{}, fmt.Errorf("trending.LatestSnapshotTime: %w", err)
	}
	return snapshot.GeneratedAt, nil
}

func (r *Repository) SourceSignals(ctx context.Context) (map[string]int64, error) {
	last24h := time.Now().UTC().Add(-24 * time.Hour)
	historyCount, err := r.crawlHistory.CountDocuments(ctx, bson.M{"created_at": bson.M{"$gte": last24h}})
	if err != nil {
		return nil, fmt.Errorf("trending.SourceSignals history: %w", err)
	}
	feedCount, err := r.rssFeeds.CountDocuments(ctx, bson.M{"enabled": true})
	if err != nil {
		return nil, fmt.Errorf("trending.SourceSignals feeds: %w", err)
	}
	return map[string]int64{
		"crawl_history_24h": historyCount,
		"rss_feeds_enabled": feedCount,
	}, nil
}

func (r *Repository) SeedCandidates(ctx context.Context, limit int64) ([]seedDocument, error) {
	if limit <= 0 {
		limit = 100
	}
	cursor, err := r.crawlHistory.Find(ctx, bson.M{"status": entities.CrawlStatusSuccess}, options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(limit))
	if err != nil {
		return nil, fmt.Errorf("trending.SeedCandidates: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []seedDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("trending.SeedCandidates decode: %w", err)
	}
	return docs, nil
}

type seedDocument struct {
	Title        string    `bson:"title"`
	Description  string    `bson:"description"`
	URL          string    `bson:"url"`
	Tags         []string  `bson:"tags"`
	QualityScore float64   `bson:"quality_score"`
	CreatedAt    time.Time `bson:"created_at"`
}
