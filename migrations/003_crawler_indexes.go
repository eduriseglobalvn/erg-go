// Package migrations contains MongoDB index definitions for all services.
package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

// RunMigration003 runs the crawler-service index migrations.
func RunMigration003(ctx context.Context, db *mongo.Database) error {
	return Run003CrawlerIndexes(ctx, db)
}
