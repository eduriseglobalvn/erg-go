package ai_content

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"erg.ninja/pkg/database"
)

type Repository struct {
	col *mongo.Collection
}

func NewRepository(db *database.MongoClient) *Repository {
	return &Repository{
		col: db.Database().Collection("api_keys"),
	}
}

// GetActiveKey finds an active API key for the given provider.
func (r *Repository) GetActiveKey(ctx context.Context, provider ProviderType) (*ApiKey, error) {
	filter := bson.M{
		"provider": provider,
		"status":   ApiStatusActive,
	}
	var key ApiKey
	err := r.col.FindOne(ctx, filter).Decode(&key)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // No active key found
		}
		return nil, err
	}
	return &key, nil
}
