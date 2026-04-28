// Package entities provides domain models for the analytics module.
package entities

import "time"

// Visit represents a page-view session stored in MongoDB.
type Visit struct {
	ID              string    `bson:"_id,omitempty" json:"id"`
	TenantID        string    `bson:"tenant_id" json:"tenant_id"`
	UserID          *int64    `bson:"user_id,omitempty" json:"user_id,omitempty"`
	SessionID       string    `bson:"session_id" json:"session_id"`
	PageURL         string    `bson:"url" json:"url"`
	Referrer        string    `bson:"referrer,omitempty" json:"referrer,omitempty"`
	IPHash          string    `bson:"ip_hash,omitempty" json:"ip_hash,omitempty"`
	UserAgent       string    `bson:"user_agent,omitempty" json:"user_agent,omitempty"`
	DeviceType      string    `bson:"device_type,omitempty" json:"device_type,omitempty"`
	Browser         string    `bson:"browser,omitempty" json:"browser,omitempty"`
	OS              string    `bson:"os,omitempty" json:"os,omitempty"`
	Country         string    `bson:"country,omitempty" json:"country,omitempty"`
	City            string    `bson:"city,omitempty" json:"city,omitempty"`
	Region          string    `bson:"region,omitempty" json:"region,omitempty"`
	Timezone        string    `bson:"timezone,omitempty" json:"timezone,omitempty"`
	EntityType      string    `bson:"entity_type,omitempty" json:"entity_type,omitempty"`
	EntityID        string    `bson:"entity_id,omitempty" json:"entity_id,omitempty"`
	DurationSeconds int       `bson:"duration_seconds" json:"duration_seconds"`
	PageViews       int       `bson:"page_views" json:"page_views"`
	CreatedAt       time.Time `bson:"created_at" json:"createdAt"`
	UpdatedAt       time.Time `bson:"updated_at" json:"updatedAt"`
}

// VisitCollection is the MongoDB collection name.
const VisitCollection = "analytics_visits"
