// Package entities provides domain models for the analytics module.
package entities

import "time"

// FirebaseSyncEvent represents a raw event ingested from Firebase Analytics.
// It stores deduplication metadata to avoid double-counting when Firebase
// Functions or BigQuery exports push batches to erg-go.
type FirebaseSyncEvent struct {
	ID            string                 `bson:"_id,omitempty" json:"id"`
	TenantID      string                 `bson:"tenant_id" json:"tenant_id"`
	EventName     string                 `bson:"event_name" json:"event_name"`
	Platform      string                 `bson:"platform" json:"platform"` // web | ios | android
	UserID        string                 `bson:"user_id,omitempty" json:"user_id,omitempty"`
	DeviceID      string                 `bson:"device_id,omitempty" json:"device_id,omitempty"`
	SessionID     string                 `bson:"session_id,omitempty" json:"session_id,omitempty"`
	Params        map[string]interface{} `bson:"params,omitempty" json:"params,omitempty"`
	Country       string                 `bson:"country,omitempty" json:"country,omitempty"`
	City          string                 `bson:"city,omitempty" json:"city,omitempty"`
	AppInstanceID string                 `bson:"app_instance_id" json:"app_instance_id"`
	ReceivedAt    time.Time              `bson:"received_at" json:"received_at"`   // Timestamp from Firebase event
	ProcessedAt   time.Time              `bson:"processed_at" json:"processed_at"` // When erg-go processed it
	CreatedAt     time.Time              `bson:"created_at" json:"createdAt"`
}

// FirebaseSyncEventCollection is the MongoDB collection name for Firebase sync events.
const FirebaseSyncEventCollection = "analytics_firebase_sync"
