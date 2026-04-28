// Package entities provides domain models for the posts module.
package entities

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Category represents a post category stored in MongoDB.
type Category struct {
	ID          bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Name        string        `bson:"name" json:"name"`
	Slug        string        `bson:"slug" json:"slug"`
	Description string        `bson:"description,omitempty" json:"description,omitempty"`
	Icon        string        `bson:"icon,omitempty" json:"icon,omitempty"`
	IsHidden    bool          `bson:"is_hidden" json:"is_hidden"`
	CreatedAt   time.Time     `bson:"created_at" json:"createdAt"`
	UpdatedAt   time.Time     `bson:"updated_at" json:"updatedAt"`
}

// CategoryCollection is the MongoDB collection name.
const CategoryCollection = "post_categories"
