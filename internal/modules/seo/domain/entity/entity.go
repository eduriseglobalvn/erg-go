package entity

import (
	"encoding/json"
	"time"
)

type SeoKeyword struct {
	ID        string    `bson:"_id,omitempty"          json:"id"`
	Keyword   string    `bson:"keyword"               json:"keyword"`
	TargetURL string    `bson:"target_url"           json:"targetUrl"`
	LinkLimit int       `bson:"link_limit"           json:"linkLimit"`
	IsActive  bool      `bson:"is_active"             json:"isActive"`
	CreatedAt time.Time `bson:"created_at"            json:"createdAt"`
	UpdatedAt time.Time `bson:"updated_at,omitempty" json:"updatedAt,omitempty"`
}

type SeoRedirect struct {
	ID          string    `bson:"_id,omitempty"          json:"id"`
	FromPattern string    `bson:"from_pattern"           json:"fromPattern"`
	ToURL       string    `bson:"to_url"                json:"toUrl"`
	Type        string    `bson:"type"                  json:"type"`
	IsRegex     bool      `bson:"is_regex"              json:"isRegex"`
	IsActive    bool      `bson:"is_active"             json:"isActive"`
	HitCount    int       `bson:"hit_count"            json:"hitCount"`
	CreatedAt   time.Time `bson:"created_at"            json:"createdAt"`
	UpdatedAt   time.Time `bson:"updated_at,omitempty" json:"updatedAt,omitempty"`
}

type Seo404Log struct {
	ID        string    `bson:"_id,omitempty"     json:"id"`
	URL       string    `bson:"url"              json:"url"`
	Referrer  string    `bson:"referrer,omitempty" json:"referrer,omitempty"`
	UserAgent string    `bson:"useragent,omitempty" json:"userAgent,omitempty"`
	HitCount  int       `bson:"hit_count"        json:"hitCount"`
	LastSeen  time.Time `bson:"last_seen"         json:"lastSeen"`
	FirstSeen time.Time `bson:"first_seen"        json:"firstSeen"`
}

type SeoConfig struct {
	Key       string          `bson:"_id,omitempty" json:"key"`
	Value     json.RawMessage `bson:"value"          json:"value"`
	UpdatedBy string          `bson:"updated_by,omitempty" json:"updatedBy,omitempty"`
	UpdatedAt time.Time       `bson:"updated_at,omitempty"  json:"updatedAt,omitempty"`
}

type GSCData struct {
	ID          string    `bson:"_id,omitempty"   json:"id"`
	PostID      string    `bson:"post_id"        json:"postId"`
	URL         string    `bson:"url"            json:"url"`
	Clicks      int       `bson:"clicks"         json:"clicks"`
	Impressions int       `bson:"impressions"    json:"impressions"`
	CTR         float64   `bson:"ctr"            json:"ctr"`
	Position    float64   `bson:"position"       json:"position"`
	Date        time.Time `bson:"date"           json:"date"`
}
