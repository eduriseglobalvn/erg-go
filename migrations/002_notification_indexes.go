// Package migrations contains MongoDB index definitions for all services.
package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

// RunMigration002 runs the notification-service index migrations.
func RunMigration002(ctx context.Context, db *mongo.Database) error {
	return Run002NotificationIndexes(ctx, db)
}
