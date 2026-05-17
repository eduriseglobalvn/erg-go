package entities

import (
	"time"
)

type SystemConfig struct {
	ID          string    `bson:"_id,omitempty" json:"id"`
	Key         string    `bson:"key" json:"key"`
	Value       any       `bson:"value" json:"value"`
	Description string    `bson:"description,omitempty" json:"description"`
	UpdatedBy   string    `bson:"updated_by,omitempty" json:"updated_by"`
	UpdatedAt   time.Time `bson:"updated_at" json:"updatedAt"`
}
