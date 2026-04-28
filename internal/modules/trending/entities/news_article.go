package entities

import "time"

const NewsArticleCollection = "news_articles"

// NewsArticle is a supporting document used to explain why a topic is trending.
type NewsArticle struct {
	ID             string    `bson:"_id,omitempty" json:"id"`
	Topic          string    `bson:"topic" json:"topic"`
	Headline       string    `bson:"headline" json:"headline"`
	Source         string    `bson:"source" json:"source"`
	URL            string    `bson:"url" json:"url"`
	PublishedAt    time.Time `bson:"published_at" json:"published_at"`
	RelevanceScore float64   `bson:"relevance_score" json:"relevance_score"`
	CreatedAt      time.Time `bson:"created_at" json:"createdAt"`
	UpdatedAt      time.Time `bson:"updated_at" json:"updatedAt"`
}
