package entities

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// CandidateStatus mirrors the NestJS CandidateStatus enum.
const (
	CandidateStatusPending   = "PENDING"
	CandidateStatusReviewing = "REVIEWING"
	CandidateStatusInterview = "INTERVIEW"
	CandidateStatusOffer     = "OFFER"
	CandidateStatusHired     = "HIRED"
	CandidateStatusRejected  = "REJECTED"
)

// ApplyType mirrors the NestJS ApplyType enum.
const (
	ApplyTypeOnline = "ONLINE"
	ApplyTypeZalo   = "ZALO"
)

// Candidate represents an applicant document in MongoDB.
type Candidate struct {
	ID           bson.ObjectID `bson:"_id,omitempty" json:"id"`
	JobID        string        `bson:"job_id,omitempty" json:"job_id,omitempty"`
	JobTitle     string        `bson:"job_title,omitempty" json:"job_title,omitempty"` // denormalised for tracking page
	TenantID     string        `bson:"tenant_id" json:"tenant_id"`
	FullName     string        `bson:"full_name" json:"fullName"`
	Email        string        `bson:"email" json:"email"`
	Phone        string        `bson:"phone" json:"phone"`
	CVURL        string        `bson:"cv_url,omitempty" json:"cv_url,omitempty"`
	CoverLetter  string        `bson:"cover_letter,omitempty" json:"cover_letter,omitempty"`
	Note         string        `bson:"note,omitempty" json:"note,omitempty"`
	PublicNote   string        `bson:"public_note,omitempty" json:"public_note,omitempty"`
	ApplyType    string        `bson:"apply_type" json:"apply_type"` // ONLINE | ZALO
	Status       string        `bson:"status" json:"status"`         // PENDING | REVIEWING | INTERVIEW | OFFER | HIRED | REJECTED
	TrackingCode string        `bson:"tracking_code" json:"tracking_code"`
	CreatedAt    time.Time     `bson:"created_at" json:"createdAt"`
	UpdatedAt    time.Time     `bson:"updated_at" json:"updatedAt"`
}

// CandidateCollection is the MongoDB collection name.
const CandidateCollection = "candidates"
