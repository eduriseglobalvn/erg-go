// Package dto provides request/response types for the analytics module.
package dto

import "time"

// FirebaseEventPayload represents a single event from Firebase Analytics export.
type FirebaseEventPayload struct {
	EventName     string                 `json:"event_name" bson:"event_name"`
	Timestamp     time.Time              `json:"timestamp" bson:"timestamp"`
	Params        map[string]interface{} `json:"params,omitempty" bson:"params,omitempty"`
	UserID        string                 `json:"user_id,omitempty" bson:"user_id,omitempty"`
	DeviceID      string                 `json:"device_id,omitempty" bson:"device_id,omitempty"`
	Platform      string                 `json:"platform" bson:"platform"`
	AppInstanceID string                 `json:"app_instance_id" bson:"app_instance_id"`
	SessionID     string                 `json:"session_id,omitempty" bson:"session_id,omitempty"`
	Country       string                 `json:"country,omitempty" bson:"country,omitempty"`
	City          string                 `json:"city,omitempty" bson:"city,omitempty"`
}

// SyncFirebaseEventsRequest is the payload for POST /api/insight/firebase/sync.
// This endpoint is called server-to-server by Firebase Functions or BigQuery export.
type SyncFirebaseEventsRequest struct {
	Events []FirebaseEventPayload `json:"events" validate:"required"`
}

// SyncFirebaseEventsResponse is the response after syncing Firebase events.
type SyncFirebaseEventsResponse struct {
	Synced     int `json:"synced"`
	Duplicates int `json:"duplicates"`
}
