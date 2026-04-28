// Package entities provides domain models for the analytics module.
package entities

import "time"

// Event represents a user interaction event stored in MongoDB.
type Event struct {
	ID         string                 `bson:"_id,omitempty" json:"id"`
	TenantID   string                 `bson:"tenant_id" json:"tenant_id"`
	SessionID  string                 `bson:"session_id" json:"session_id"`
	UserID     *int64                 `bson:"user_id,omitempty" json:"user_id,omitempty"`
	EventName  string                 `bson:"event_name" json:"event_name"`
	EventType  string                 `bson:"event_type" json:"event_type"` // click, scroll, form, video, custom
	Properties map[string]interface{} `bson:"properties,omitempty" json:"properties,omitempty"`
	CreatedAt  time.Time              `bson:"created_at" json:"createdAt"`
}

// AnalyticsEvent mirrors Event for backward compatibility with FE service naming.
type AnalyticsEvent = Event

// EventCollection is the MongoDB collection name.
const EventCollection = "analytics_events"
