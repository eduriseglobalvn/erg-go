package repository

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	aicontentdto "erg.ninja/internal/modules/ai_content/api/dto"
	"erg.ninja/pkg/database"
)

type ApiKey = aicontentdto.ApiKey
type ApiKeyStatus = aicontentdto.ApiKeyStatus
type ProviderType = aicontentdto.ProviderType

const (
	ApiStatusActive = aicontentdto.ApiStatusActive
)

type Repository struct {
	col *mongo.Collection
}

func NewRepository(db *database.MongoClient) *Repository {
	return &Repository{
		col: db.Database().Collection("api_keys"),
	}
}

func (r *Repository) ListKeys(ctx context.Context) ([]ApiKey, error) {
	opts := options.Find().SetSort(bson.D{
		{Key: "selected", Value: -1},
		{Key: "updatedAt", Value: -1},
	})
	cur, err := r.col.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var keys []ApiKey
	if err := cur.All(ctx, &keys); err != nil {
		return nil, err
	}
	if keys == nil {
		keys = []ApiKey{}
	}
	return keys, nil
}

func (r *Repository) CreateKey(ctx context.Context, key *ApiKey) error {
	if key.ID == "" {
		key.ID = database.NewID()
	}
	now := time.Now().UTC()
	if key.CreatedAt.IsZero() {
		key.CreatedAt = now
	}
	key.UpdatedAt = now
	_, err := r.col.InsertOne(ctx, key)
	return err
}

func (r *Repository) UpdateKeySecret(ctx context.Context, key *ApiKey) error {
	if key == nil || key.ID == "" {
		return nil
	}
	now := time.Now().UTC()
	update := bson.M{
		"encryptedKey":     key.EncryptedKey,
		"keyNonce":         key.KeyNonce,
		"keyVersion":       key.KeyVersion,
		"keyFingerprint":   key.KeyFingerprint,
		"maskedKeyPreview": key.MaskedKeyPreview,
		"updatedAt":        now,
	}
	_, err := r.col.UpdateOne(ctx, bson.M{"_id": key.ID}, bson.M{
		"$set":   update,
		"$unset": bson.M{"key": ""},
	})
	return err
}

func (r *Repository) GetKeyByID(ctx context.Context, id string) (*ApiKey, error) {
	var key ApiKey
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&key)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &key, nil
}

func (r *Repository) DeleteKey(ctx context.Context, id string) (bool, error) {
	res, err := r.col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return false, err
	}
	return res.DeletedCount > 0, nil
}

func (r *Repository) SelectKey(ctx context.Context, id string) (*ApiKey, error) {
	now := time.Now().UTC()
	if _, err := r.col.UpdateMany(ctx, bson.M{"selected": true}, bson.M{
		"$set": bson.M{"selected": false, "updatedAt": now},
	}); err != nil {
		return nil, err
	}
	res, err := r.col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{
		"$set": bson.M{
			"selected":  true,
			"status":    ApiStatusActive,
			"updatedAt": now,
		},
	})
	if err != nil {
		return nil, err
	}
	if res.MatchedCount == 0 {
		return nil, nil
	}
	return r.GetKeyByID(ctx, id)
}

func (r *Repository) UpdateKeyStatus(ctx context.Context, id string, status ApiKeyStatus, message string) (*ApiKey, error) {
	now := time.Now().UTC()
	update := bson.M{
		"status":           status,
		"lastTestedAt":     now,
		"lastErrorMessage": message,
		"updatedAt":        now,
	}
	if status == ApiStatusActive {
		update["consecutiveErrors"] = 0
		update["cooldownUntil"] = nil
	} else {
		update["consecutiveErrors"] = 1
	}
	res, err := r.col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": update})
	if err != nil {
		return nil, err
	}
	if res.MatchedCount == 0 {
		return nil, nil
	}
	return r.GetKeyByID(ctx, id)
}

func (r *Repository) TouchKeyUsed(ctx context.Context, id string) error {
	now := time.Now().UTC()
	_, err := r.col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{
		"$inc": bson.M{"usageCount": 1, "todayUsage": 1},
		"$set": bson.M{"lastUsedAt": now, "updatedAt": now},
	})
	return err
}

func (r *Repository) GetSelectedKey(ctx context.Context) (*ApiKey, error) {
	var key ApiKey
	err := r.col.FindOne(ctx, bson.M{
		"selected": true,
		"status":   ApiStatusActive,
	}).Decode(&key)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &key, nil
}

// GetActiveKey finds an active API key for the given provider.
func (r *Repository) GetActiveKey(ctx context.Context, provider ProviderType) (*ApiKey, error) {
	filter := bson.M{
		"provider": provider,
		"status":   ApiStatusActive,
	}
	var key ApiKey
	err := r.col.FindOne(ctx, filter, options.FindOne().SetSort(bson.D{
		{Key: "selected", Value: -1},
		{Key: "updatedAt", Value: -1},
	})).Decode(&key)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // No active key found
		}
		return nil, err
	}
	return &key, nil
}
