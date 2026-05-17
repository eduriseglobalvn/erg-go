package entity

import "time"

const TrendingTopicCollection = "trending_topics"

// TrendingTopic is the normalized aggregated topic stored in MongoDB.
type TrendingTopic struct {
	ID              string    `bson:"_id,omitempty" json:"id"`
	Topic           string    `bson:"topic" json:"topic"`
	Slug            string    `bson:"slug" json:"slug"`
	Score           float64   `bson:"score" json:"score"`
	Volume          int       `bson:"volume" json:"volume"`
	Source          string    `bson:"source" json:"source"`
	Keywords        []string  `bson:"keywords,omitempty" json:"keywords,omitempty"`
	URLs            []string  `bson:"urls,omitempty" json:"urls,omitempty"`
	Timeline        []int     `bson:"timeline,omitempty" json:"timeline,omitempty"`
	LastRefreshedAt time.Time `bson:"last_refreshed_at" json:"last_refreshed_at"`
	CreatedAt       time.Time `bson:"created_at" json:"createdAt"`
	UpdatedAt       time.Time `bson:"updated_at" json:"updatedAt"`
}
