package entity

import "time"

const TrendingSnapshotCollection = "trending_snapshots"

// TrendingSnapshot is a point-in-time snapshot used for historical charts.
type TrendingSnapshot struct {
	ID          string    `bson:"_id,omitempty" json:"id"`
	Topics      []string  `bson:"topics" json:"topics"`
	TopicCount  int       `bson:"topic_count" json:"topic_count"`
	GeneratedAt time.Time `bson:"generated_at" json:"generated_at"`
	CreatedAt   time.Time `bson:"created_at" json:"createdAt"`
}
