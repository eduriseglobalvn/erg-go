package repository

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	seodto "erg.ninja/internal/modules/seo/api/dto"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
)

type GSCData = seodto.GSCData
type Seo404Log = seodto.Seo404Log
type SeoConfig = seodto.SeoConfig
type SeoKeyword = seodto.SeoKeyword
type SeoRedirect = seodto.SeoRedirect

const (
	CollectionKeywords  = "seo_keywords"
	CollectionRedirects = "seo_redirects"
	Collection404Logs   = "seo_404_logs"
	CollectionConfigs   = "seo_configs"
	CollectionGSCData   = "seo_gsc_data"
)

type Repository struct {
	log       *logger.Logger
	keywords  *mongo.Collection
	redirects *mongo.Collection
	log404    *mongo.Collection
	configs   *mongo.Collection
	gscData   *mongo.Collection
}

type RepositoryOption func(*Repository)

func WithRepositoryLogger(log *logger.Logger) RepositoryOption {
	return func(r *Repository) { r.log = log }
}

func NewRepository(mongoClient *database.MongoClient, opts ...RepositoryOption) *Repository {
	r := &Repository{
		log:       logger.NoOp(),
		keywords:  mongoClient.Collection(CollectionKeywords),
		redirects: mongoClient.Collection(CollectionRedirects),
		log404:    mongoClient.Collection(Collection404Logs),
		configs:   mongoClient.Collection(CollectionConfigs),
		gscData:   mongoClient.Collection(CollectionGSCData),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// ─── Keywords ────────────────────────────────────────────────────────────────

func (r *Repository) ListKeywords(ctx context.Context) ([]*SeoKeyword, error) {
	cursor, err := r.keywords.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("seo.ListKeywords: %w", err)
	}
	defer cursor.Close(ctx)
	var out []*SeoKeyword
	if err := cursor.All(ctx, &out); err != nil {
		return nil, fmt.Errorf("seo.ListKeywords decode: %w", err)
	}
	return out, nil
}

func (r *Repository) CreateKeyword(ctx context.Context, k *SeoKeyword) error {
	if k.ID == "" {
		k.ID = database.NewID()
	}
	if k.CreatedAt.IsZero() {
		k.CreatedAt = time.Now().UTC()
	}
	_, err := r.keywords.InsertOne(ctx, k)
	if err != nil {
		if database.IsDuplicateKey(err) {
			return fmt.Errorf("keyword already exists")
		}
		return fmt.Errorf("seo.CreateKeyword: %w", err)
	}
	return nil
}

func (r *Repository) DeleteKeyword(ctx context.Context, id string) error {
	_, err := r.keywords.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("seo.DeleteKeyword: %w", err)
	}
	return nil
}

// ─── Redirects ───────────────────────────────────────────────────────────────

func (r *Repository) ListRedirects(ctx context.Context) ([]*SeoRedirect, error) {
	cursor, err := r.redirects.Find(ctx, bson.M{}, options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("seo.ListRedirects: %w", err)
	}
	defer cursor.Close(ctx)
	var out []*SeoRedirect
	if err := cursor.All(ctx, &out); err != nil {
		return nil, fmt.Errorf("seo.ListRedirects decode: %w", err)
	}
	return out, nil
}

func (r *Repository) CreateRedirect(ctx context.Context, red *SeoRedirect) error {
	if red.ID == "" {
		red.ID = database.NewID()
	}
	if red.CreatedAt.IsZero() {
		red.CreatedAt = time.Now().UTC()
	}
	_, err := r.redirects.InsertOne(ctx, red)
	if err != nil {
		return fmt.Errorf("seo.CreateRedirect: %w", err)
	}
	return nil
}

func (r *Repository) FindRedirectByID(ctx context.Context, id string) (*SeoRedirect, error) {
	var red SeoRedirect
	err := r.redirects.FindOne(ctx, bson.M{"_id": id}).Decode(&red)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("seo.FindRedirectByID: %w", err)
	}
	return &red, nil
}

func (r *Repository) FindRedirectMatch(ctx context.Context, url string) (*SeoRedirect, error) {
	// 1. Exact match.
	var exact SeoRedirect
	err := r.redirects.FindOne(ctx, bson.M{
		"from_pattern": url,
		"is_active":    true,
		"is_regex":     false,
	}).Decode(&exact)
	if err == nil {
		return &exact, nil
	}
	if err != mongo.ErrNoDocuments {
		return nil, fmt.Errorf("seo.FindRedirectMatch exact: %w", err)
	}

	// 2. Regex match.
	cursor, err := r.redirects.Find(ctx, bson.M{"is_active": true, "is_regex": true})
	if err != nil {
		return nil, fmt.Errorf("seo.FindRedirectMatch find: %w", err)
	}
	defer cursor.Close(ctx)
	var redirects []*SeoRedirect
	if err := cursor.All(ctx, &redirects); err != nil {
		return nil, fmt.Errorf("seo.FindRedirectMatch decode: %w", err)
	}
	for _, redirect := range redirects {
		re, err := regexp.Compile(redirect.FromPattern)
		if err != nil {
			continue
		}
		if re.MatchString(url) {
			return redirect, nil
		}
	}
	return nil, nil
}

func (r *Repository) UpdateRedirect(ctx context.Context, id string, updates bson.M) error {
	_, err := r.redirects.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": updates})
	if err != nil {
		return fmt.Errorf("seo.UpdateRedirect: %w", err)
	}
	return nil
}

func (r *Repository) DeleteRedirect(ctx context.Context, id string) error {
	_, err := r.redirects.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("seo.DeleteRedirect: %w", err)
	}
	return nil
}

func (r *Repository) IncrementRedirectHit(ctx context.Context, id string) error {
	_, err := r.redirects.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$inc": bson.M{"hit_count": 1}})
	if err != nil {
		return fmt.Errorf("seo.IncrementRedirectHit: %w", err)
	}
	return nil
}

// ─── 404 Logs ────────────────────────────────────────────────────────────────

func (r *Repository) Upsert404Log(ctx context.Context, log *Seo404Log) error {
	filter := bson.M{"url": log.URL}
	update := bson.M{
		"$set": bson.M{
			"referrer":  log.Referrer,
			"useragent": log.UserAgent,
			"last_seen": time.Now().UTC(),
		},
		"$inc": bson.M{"hit_count": 1},
	}
	opts := options.UpdateOne().SetUpsert(true)
	_, err := r.log404.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("seo.Upsert404Log: %w", err)
	}
	return nil
}

func (r *Repository) List404Logs(ctx context.Context, page, limit int) ([]*Seo404Log, int64, error) {
	skip := int64((page - 1) * limit)
	total, err := r.log404.CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, 0, fmt.Errorf("seo.List404Logs count: %w", err)
	}
	cursor, err := r.log404.Find(ctx, bson.M{}, options.Find().
		SetSort(bson.D{{Key: "last_seen", Value: -1}}).
		SetSkip(skip).
		SetLimit(int64(limit)))
	if err != nil {
		return nil, 0, fmt.Errorf("seo.List404Logs find: %w", err)
	}
	defer cursor.Close(ctx)
	var out []*Seo404Log
	if err := cursor.All(ctx, &out); err != nil {
		return nil, 0, fmt.Errorf("seo.List404Logs decode: %w", err)
	}
	return out, total, nil
}

// ─── Configs ────────────────────────────────────────────────────────────────

func (r *Repository) GetConfig(ctx context.Context, key string) (*SeoConfig, error) {
	var cfg SeoConfig
	err := r.configs.FindOne(ctx, bson.M{"_id": key}).Decode(&cfg)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("seo.GetConfig: %w", err)
	}
	return &cfg, nil
}

func (r *Repository) UpsertConfig(ctx context.Context, cfg *SeoConfig) error {
	filter := bson.M{"_id": cfg.Key}
	update := bson.M{
		"$set": bson.M{
			"value":     cfg.Value,
			"updatedBy": cfg.UpdatedBy,
			"updatedAt": time.Now().UTC(),
		},
	}
	opts := options.UpdateOne().SetUpsert(true)
	_, err := r.configs.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("seo.UpsertConfig: %w", err)
	}
	return nil
}

// ─── GSC Data ───────────────────────────────────────────────────────────────

func (r *Repository) SaveGSCData(ctx context.Context, data *GSCData) error {
	if data.ID == "" {
		data.ID = database.NewID()
	}
	_, err := r.gscData.InsertOne(ctx, data)
	if err != nil {
		return fmt.Errorf("seo.SaveGSCData: %w", err)
	}
	return nil
}

func (r *Repository) GetGSCDataForPost(ctx context.Context, postID string, days int) ([]*GSCData, error) {
	since := time.Now().UTC().AddDate(0, 0, -days)
	cursor, err := r.gscData.Find(ctx, bson.M{
		"post_id": postID,
		"date":    bson.M{"$gte": since},
	}, options.Find().SetSort(bson.D{{Key: "date", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("seo.GetGSCDataForPost: %w", err)
	}
	defer cursor.Close(ctx)
	var out []*GSCData
	if err := cursor.All(ctx, &out); err != nil {
		return nil, fmt.Errorf("seo.GetGSCDataForPost decode: %w", err)
	}
	return out, nil
}

func (r *Repository) GetTopGSCPosts(ctx context.Context, days int, limit int64) ([]*GSCData, error) {
	since := time.Now().UTC().AddDate(0, 0, -days)
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"date": bson.M{"$gte": since}}}},
		{{Key: "$group", Value: bson.M{
			"_id":         "$post_id",
			"totalClicks": bson.M{"$sum": "$clicks"},
			"totalImpr":   bson.M{"$sum": "$impressions"},
			"avgCTR":      bson.M{"$avg": "$ctr"},
			"avgPos":      bson.M{"$avg": "$position"},
		}}},
		{{Key: "$sort", Value: bson.M{"totalClicks": -1}}},
		{{Key: "$limit", Value: limit}},
	}
	cursor, err := r.gscData.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("seo.GetTopGSCPosts: %w", err)
	}
	defer cursor.Close(ctx)
	var out []*GSCData
	if err := cursor.All(ctx, &out); err != nil {
		return nil, fmt.Errorf("seo.GetTopGSCPosts decode: %w", err)
	}
	return out, nil
}

// ─── Health ─────────────────────────────────────────────────────────────────

func (r *Repository) ListGSCDataSince(ctx context.Context, since time.Time) ([]*GSCData, error) {
	cursor, err := r.gscData.Find(ctx, bson.M{"date": bson.M{"$gte": since}})
	if err != nil {
		return nil, fmt.Errorf("seo.ListGSCDataSince: %w", err)
	}
	defer cursor.Close(ctx)
	var out []*GSCData
	if err := cursor.All(ctx, &out); err != nil {
		return nil, fmt.Errorf("seo.ListGSCDataSince decode: %w", err)
	}
	return out, nil
}

func (r *Repository) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return r.keywords.Database().Client().Ping(ctx, nil)
}
