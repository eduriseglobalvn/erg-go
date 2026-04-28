// Package migrations contains MongoDB index definitions for all services.
package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

// RunMigration005 runs the notification read-index migrations.
func RunMigration005(ctx context.Context, db *mongo.Database) error {
	return Run005NotificationReadIndexes(ctx, db)
}
