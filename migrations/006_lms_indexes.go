// Package migrations contains MongoDB index definitions for all services.
package migrations

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// RunMigration006 runs the LMS index migrations.
func RunMigration006(ctx context.Context, db *mongo.Database) error {
	return Run006LMSIndexes(ctx, db)
}

// Run006LMSIndexes creates production indexes for LMS collections.
func Run006LMSIndexes(ctx context.Context, db *mongo.Database) error {
	collections := []struct {
		name       string
		indexes    []mongo.IndexModel
		indexNames []string
	}{
		{
			name: "lms_students",
			indexes: []mongo.IndexModel{
				{
					Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "username", Value: 1}},
					Options: options.Index().
						SetName("idx_lms_student_unique_username").
						SetUnique(true),
				},
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "class_id", Value: 1}, {Key: "status", Value: 1}, {Key: "full_name", Value: 1}},
					Options: options.Index().SetName("idx_lms_student_class_status"),
				},
				{
					Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "student_code", Value: 1}},
					Options: options.Index().
						SetName("idx_lms_student_unique_student_code").
						SetUnique(true).
						SetPartialFilterExpression(bson.M{"student_code": bson.M{"$type": "string", "$gt": ""}}),
				},
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "center_id", Value: 1}, {Key: "class_id", Value: 1}, {Key: "status", Value: 1}, {Key: "student_code", Value: 1}},
					Options: options.Index().SetName("idx_lms_student_roster_export"),
				},
				{
					Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "auth_user_id", Value: 1}},
					Options: options.Index().
						SetName("idx_lms_student_unique_auth_user").
						SetUnique(true).
						SetPartialFilterExpression(bson.M{"auth_user_id": bson.M{"$type": "string", "$gt": ""}}),
				},
			},
		},
		{
			name: "lms_classes",
			indexes: []mongo.IndexModel{
				{
					Keys:    bson.D{{Key: "tenant_id", Value: 1}, {Key: "center_id", Value: 1}, {Key: "status", Value: 1}, {Key: "name", Value: 1}},
					Options: options.Index().SetName("idx_lms_class_center_status"),
				},
			},
		},
		{
			name: "lms_current_scopes",
			indexes: []mongo.IndexModel{
				{
					Keys: bson.D{{Key: "tenant_id", Value: 1}, {Key: "user_id", Value: 1}},
					Options: options.Index().
						SetName("idx_lms_current_scope_unique_user").
						SetUnique(true),
				},
			},
		},
	}
	return createIndexes(ctx, db, collections)
}
