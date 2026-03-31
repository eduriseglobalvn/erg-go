// Package migrations contains MongoDB index definitions for all services.
package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

// RunMigration004 runs the trending-service index migrations.
func RunMigration004(ctx context.Context, db *mongo.Database) error {
	return Run004TrendingIndexes(ctx, db)
}
