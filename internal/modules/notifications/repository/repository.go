// Package repository provides MongoDB data access for the notifications module.
package repository

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"erg.ninja/internal/modules/notifications/entities"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
)

// Repository provides data access operations for notifications.
type Repository struct {
	client       *mongo.Client
	dbName       string
	notifColl    *mongo.Collection
	prefColl     *mongo.Collection
	digestColl   *mongo.Collection
	deliveryColl *mongo.Collection
	log          *logger.Logger
}

// RepositoryOption configures the Repository.
type RepositoryOption func(*Repository)

// WithRepositoryLogger sets the logger.
func WithRepositoryLogger(log *logger.Logger) RepositoryOption {
	return func(r *Repository) { r.log = log }
}

// NewRepository creates a new notifications repository.
func NewRepository(mongo *database.MongoClient, opts ...RepositoryOption) *Repository {
	r := &Repository{
		client:       mongo.Client(),
		dbName:       mongo.DatabaseName(),
		notifColl:    mongo.Collection(entities.NotificationCollection),
		prefColl:     mongo.Collection(entities.NotificationPreferenceCollection),
		digestColl:   mongo.Collection(entities.DigestCollection),
		deliveryColl: mongo.Collection(entities.DeliveryLogCollection),
		log:          logger.NoOp(),
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

// ─── Notification CRUD ───────────────────────────────────────────────────────

// Create inserts a new notification and returns its generated ID.
func (r *Repository) Create(ctx context.Context, n *entities.Notification) error {
	if n.ID == bson.NilObjectID {
		n.ID = bson.NewObjectID()
	}
	n.CreatedAt = time.Now().UTC()
	n.UpdatedAt = n.CreatedAt
	if n.Status == "" {
		n.Status = entities.StatusPending
	}

	_, err := r.notifColl.InsertOne(ctx, n)
	if err != nil {
		return fmt.Errorf("notifications.Create: %w", err)
	}
	return nil
}

// GetByID retrieves a notification by ID.
func (r *Repository) GetByID(ctx context.Context, id string) (*entities.Notification, error) {
	objID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, fmt.Errorf("notifications.GetByID: invalid ID: %w", err)
	}
	var n entities.Notification
	err = r.notifColl.FindOne(ctx, bson.M{"_id": objID}).Decode(&n)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("notifications.GetByID: %w", err)
	}
	return &n, nil
}

// UpdateStatus updates the status and error message of a notification.
func (r *Repository) UpdateStatus(ctx context.Context, id string, status entities.NotificationStatus, errMsg string) error {
	objID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return fmt.Errorf("notifications.UpdateStatus: invalid ID: %w", err)
	}
	update := bson.M{
		"$set": bson.M{
			"status":     status,
			"error_msg":  errMsg,
			"updated_at": time.Now().UTC(),
		},
	}
	if status == entities.StatusSent {
		now := time.Now().UTC()
		update["$set"].(bson.M)["sent_at"] = now
	}
	_, err = r.notifColl.UpdateOne(ctx, bson.M{"_id": objID}, update)
	if err != nil {
		return fmt.Errorf("notifications.UpdateStatus: %w", err)
	}
	return nil
}

// MarkDelivered marks a notification as delivered.
func (r *Repository) MarkDelivered(ctx context.Context, id string) error {
	objID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return fmt.Errorf("notifications.MarkDelivered: invalid ID: %w", err)
	}
	_, err = r.notifColl.UpdateOne(ctx, bson.M{"_id": objID}, bson.M{
		"$set": bson.M{
			"status":       entities.StatusDelivered,
			"delivered_at": time.Now().UTC(),
			"updated_at":   time.Now().UTC(),
		},
	})
	if err != nil {
		return fmt.Errorf("notifications.MarkDelivered: %w", err)
	}
	return nil
}

// MarkRead marks a notification as read.
func (r *Repository) MarkRead(ctx context.Context, id string) error {
	objID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return fmt.Errorf("notifications.MarkRead: invalid ID: %w", err)
	}
	now := time.Now().UTC()
	_, err = r.notifColl.UpdateOne(ctx, bson.M{"_id": objID}, bson.M{
		"$set": bson.M{
			"read":       true,
			"read_at":    now,
			"readAt":     now,
			"status":     "READ",
			"updated_at": now,
		},
	})
	if err != nil {
		return fmt.Errorf("notifications.MarkRead: %w", err)
	}
	return nil
}

// MarkFailed marks a notification as failed permanently.
func (r *Repository) MarkFailed(ctx context.Context, id string, errMsg string) error {
	objID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return fmt.Errorf("notifications.MarkFailed: invalid ID: %w", err)
	}
	_, err = r.notifColl.UpdateOne(ctx, bson.M{"_id": objID}, bson.M{
		"$set": bson.M{
			"status":     entities.StatusFailed,
			"error_msg":  errMsg,
			"updated_at": time.Now().UTC(),
		},
	})
	if err != nil {
		return fmt.Errorf("notifications.MarkFailed: %w", err)
	}
	return nil
}

// IncrementRetry increments the retry count.
func (r *Repository) IncrementRetry(ctx context.Context, id string, lastErr string) error {
	objID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return fmt.Errorf("notifications.IncrementRetry: invalid ID: %w", err)
	}
	_, err = r.notifColl.UpdateOne(ctx, bson.M{"_id": objID}, bson.M{
		"$inc": bson.M{"retry_count": 1},
		"$set": bson.M{
			"status":     entities.StatusRetrying,
			"error_msg":  lastErr,
			"updated_at": time.Now().UTC(),
		},
	})
	if err != nil {
		return fmt.Errorf("notifications.IncrementRetry: %w", err)
	}
	return nil
}

// MarkDigested marks notifications as part of a digest.
func (r *Repository) MarkDigested(ctx context.Context, ids []string, digestID string) error {
	var objIDs []bson.ObjectID
	for _, id := range ids {
		if oid, err := bson.ObjectIDFromHex(id); err == nil {
			objIDs = append(objIDs, oid)
		}
	}
	update := bson.M{
		"$set": bson.M{
			"digested":   true,
			"digest_id":  digestID,
			"updated_at": time.Now().UTC(),
		},
	}
	_, err := r.notifColl.UpdateMany(ctx, bson.M{"_id": bson.M{"$in": objIDs}}, update)
	if err != nil {
		return fmt.Errorf("notifications.MarkDigested: %w", err)
	}
	return nil
}

// ListParams filters for List.
type ListParams struct {
	UserID  string
	Channel entities.ChannelType
	Status  entities.NotificationStatus
	Limit   int64
	Offset  int64
}

// List returns a list of notifications.
func (r *Repository) List(ctx context.Context, p ListParams) ([]*entities.Notification, int64, error) {
	filter := bson.M{}
	if p.UserID != "" {
		mergeFilter(filter, userNotificationFilter(p.UserID))
	}
	if p.Channel != "" {
		filter["channel"] = p.Channel
	}
	if p.Status != "" {
		if string(p.Status) == "READ" || string(p.Status) == "UNREAD" {
			filter["status"] = string(p.Status)
		} else {
			filter["status"] = p.Status
		}
	}

	total, err := r.notifColl.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("notifications.List count: %w", err)
	}

	opts := options.Find().SetLimit(p.Limit).SetSkip(p.Offset).SetSort(bson.M{"created_at": -1})
	cursor, err := r.notifColl.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("notifications.List find: %w", err)
	}
	defer cursor.Close(ctx)

	var results []*entities.Notification
	if err := cursor.All(ctx, &results); err != nil {
		return nil, 0, fmt.Errorf("notifications.List decode: %w", err)
	}

	return results, total, nil
}

// GetUnreadCount returns unread count for user.
func (r *Repository) GetUnreadCount(ctx context.Context, userID string) (int64, error) {
	filter := userNotificationFilter(userID)
	mergeFilter(filter, bson.M{
		"$or": []bson.M{
			{"status": "UNREAD"},
			{"read": false},
			{"read": bson.M{"$exists": false}, "readAt": bson.M{"$exists": false}},
			{"read": bson.M{"$exists": false}, "read_at": bson.M{"$exists": false}},
		},
	})
	count, err := r.notifColl.CountDocuments(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("notifications.GetUnreadCount: %w", err)
	}
	return count, nil
}

// MarkAllRead marks all notifications as read for a user, including legacy docs.
func (r *Repository) MarkAllRead(ctx context.Context, userID string) (int64, error) {
	filter := userNotificationFilter(userID)
	now := time.Now().UTC()
	result, err := r.notifColl.UpdateMany(ctx, filter, bson.M{
		"$set": bson.M{
			"read":       true,
			"read_at":    now,
			"readAt":     now,
			"status":     "READ",
			"updated_at": now,
		},
	})
	if err != nil {
		return 0, fmt.Errorf("notifications.MarkAllRead: %w", err)
	}
	return result.ModifiedCount, nil
}

// Delete removes a notification owned by a user.
func (r *Repository) Delete(ctx context.Context, id string, userID string) (bool, error) {
	objID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return false, fmt.Errorf("notifications.Delete: invalid ID: %w", err)
	}
	filter := userNotificationFilter(userID)
	filter["_id"] = objID
	result, err := r.notifColl.DeleteOne(ctx, filter)
	if err != nil {
		return false, fmt.Errorf("notifications.Delete: %w", err)
	}
	return result.DeletedCount > 0, nil
}

func userNotificationFilter(userID string) bson.M {
	or := []bson.M{{"userId": userID}}
	if oid, err := bson.ObjectIDFromHex(userID); err == nil {
		or = append(or, bson.M{"user_id": oid})
	}
	return bson.M{"$or": or}
}

func mergeFilter(dst, src bson.M) {
	for key, value := range src {
		if key == "$or" {
			if existing, ok := dst[key]; ok {
				dst["$and"] = appendAnd(dst["$and"], bson.M{key: existing}, bson.M{key: value})
				delete(dst, key)
				continue
			}
		}
		dst[key] = value
	}
}

func appendAnd(existing any, conditions ...bson.M) []bson.M {
	out := make([]bson.M, 0, len(conditions)+1)
	if current, ok := existing.([]bson.M); ok {
		out = append(out, current...)
	}
	out = append(out, conditions...)
	return out
}

// GetStats returns summary counts by status.
func (r *Repository) GetStats(ctx context.Context) (map[string]int64, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$group", Value: bson.M{"_id": "$status", "count": bson.M{"$sum": 1}}}},
	}
	cursor, err := r.notifColl.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("notifications.GetStats: %w", err)
	}
	defer cursor.Close(ctx)

	var results []struct {
		ID    string `bson:"_id"`
		Count int64  `bson:"count"`
	}
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("notifications.GetStats decode: %w", err)
	}

	stats := make(map[string]int64)
	for _, res := range results {
		stats[res.ID] = res.Count
	}
	return stats, nil
}

// GetPendingByUser returns items pending for digest for a user.
func (r *Repository) GetPendingByUser(ctx context.Context, userID string) ([]*entities.Notification, error) {
	objUserID, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return nil, fmt.Errorf("notifications.GetPendingByUser: invalid userID: %w", err)
	}
	filter := bson.M{
		"user_id":  objUserID,
		"status":   entities.StatusPending,
		"digested": false,
	}
	cursor, err := r.notifColl.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("notifications.GetPendingByUser: %w", err)
	}
	defer cursor.Close(ctx)

	var results []*entities.Notification
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("notifications.GetPendingByUser decode: %w", err)
	}
	return results, nil
}

// ─── Digest CRUD ───────────────────────────────────────────────────────────────

// CreateDigest inserts a digest record.
func (r *Repository) CreateDigest(ctx context.Context, d *entities.Digest) error {
	if d.ID == bson.NilObjectID {
		d.ID = bson.NewObjectID()
	}
	d.CreatedAt = time.Now().UTC()
	d.UpdatedAt = d.CreatedAt
	_, err := r.digestColl.InsertOne(ctx, d)
	if err != nil {
		return fmt.Errorf("notifications.CreateDigest: %w", err)
	}
	return nil
}

// MarkDigestSent marks a digest as sent.
func (r *Repository) MarkDigestSent(ctx context.Context, id string) error {
	objID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return fmt.Errorf("notifications.MarkDigestSent: invalid ID: %w", err)
	}
	now := time.Now().UTC()
	_, err = r.digestColl.UpdateOne(ctx, bson.M{"_id": objID}, bson.M{
		"$set": bson.M{
			"status":     entities.StatusSent,
			"sent_at":    now,
			"updated_at": now,
		},
	})
	if err != nil {
		return fmt.Errorf("notifications.MarkDigestSent: %w", err)
	}
	return nil
}

// ─── Delivery Log ────────────────────────────────────────────────────────────

// LogDelivery records a delivery attempt.
func (r *Repository) LogDelivery(ctx context.Context, log *entities.DeliveryLog) error {
	if log.ID == bson.NilObjectID {
		log.ID = bson.NewObjectID()
	}
	log.CreatedAt = time.Now().UTC()
	_, err := r.deliveryColl.InsertOne(ctx, log)
	if err != nil {
		return fmt.Errorf("notifications.LogDelivery: %w", err)
	}
	return nil
}

// ─── Preferences ─────────────────────────────────────────────────────────────

// GetPreference returns the preference record for a user.
func (r *Repository) GetPreference(ctx context.Context, userID string) (*entities.NotificationPreference, error) {
	objUserID, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return nil, fmt.Errorf("notifications.GetPreference: invalid userID: %w", err)
	}
	var pref entities.NotificationPreference
	err = r.prefColl.FindOne(ctx, bson.M{"user_id": objUserID}).Decode(&pref)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("notifications.GetPreference: %w", err)
	}
	return &pref, nil
}

// UpsertPreference creates or updates a user's preference.
func (r *Repository) UpsertPreference(ctx context.Context, pref *entities.NotificationPreference) error {
	pref.UpdatedAt = time.Now().UTC()
	opts := options.UpdateOne().SetUpsert(true)
	filter := bson.M{"user_id": pref.UserID}
	update := bson.M{
		"$set": bson.M{
			"email":       pref.Email,
			"discord":     pref.Discord,
			"telegram":    pref.Telegram,
			"whatsapp":    pref.WhatsApp,
			"slack":       pref.Slack,
			"webhooks":    pref.Webhooks,
			"digest_freq": pref.DigestFreq,
			"digest_time": pref.DigestTime,
			"language":    pref.Language,
			"updated_at":  pref.UpdatedAt,
		},
		"$setOnInsert": bson.M{
			"user_id":    pref.UserID,
			"created_at": time.Now().UTC(),
		},
	}
	_, err := r.prefColl.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("notifications.UpsertPreference: %w", err)
	}
	return nil
}
