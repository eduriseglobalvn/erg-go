package entity

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// PinPurpose represents the purpose of a PIN code.
type PinPurpose string

const (
	PinPurposeRegister       PinPurpose = "register"
	PinPurposeForgotPassword PinPurpose = "forgot_password"
	PinPurposeVerify         PinPurpose = "verify"
)

// PinCode represents a one-time PIN code stored in MongoDB.
type PinCode struct {
	ID        bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Email     string        `bson:"email" json:"email"`
	Code      string        `bson:"code" json:"-"`
	Purpose   PinPurpose    `bson:"purpose" json:"purpose"`
	ExpiresAt time.Time     `bson:"expires_at" json:"expires_at"`
	UsedAt    *time.Time    `bson:"used_at,omitempty" json:"used_at,omitempty"`
	CreatedAt time.Time     `bson:"created_at" json:"createdAt"`
}

// IsExpired returns true if the PIN has passed its expiry time.
func (p *PinCode) IsExpired() bool {
	return time.Now().After(p.ExpiresAt)
}

// IsUsed returns true if the PIN has already been consumed.
func (p *PinCode) IsUsed() bool {
	return p.UsedAt != nil
}
