package entities

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// JobStatus represents the display badge status of a job.
const (
	JobStatusHot    = "hot"
	JobStatusNew    = "new"
	JobStatusUrgent = "urgent"
	JobStatusNormal = "normal"
)

// EmploymentType mirrors the NestJS EmploymentType enum.
const (
	EmploymentTypeFullTime = "FULL_TIME"
	EmploymentTypePartTime = "PART_TIME"
	EmploymentTypeContract = "CONTRACT"
	EmploymentTypeIntern   = "INTERN"
)

// Job represents a job posting document in MongoDB.
type Job struct {
	ID             bson.ObjectID `bson:"_id,omitempty" json:"id"`
	TenantID       string        `bson:"tenant_id" json:"tenant_id"`
	Slug           string        `bson:"slug" json:"slug"`
	Title          string        `bson:"title" json:"title"`
	Status         string        `bson:"status" json:"status"` // hot|new|urgent|normal
	IsHot          bool          `bson:"is_hot" json:"is_hot"`
	IsNew          bool          `bson:"is_new" json:"is_new"`
	IsUrgent       bool          `bson:"is_urgent" json:"is_urgent"`
	Salary         string        `bson:"salary" json:"salary"`
	SalaryMin      *float64      `bson:"salary_min,omitempty" json:"salary_min,omitempty"`
	SalaryMax      *float64      `bson:"salary_max,omitempty" json:"salary_max,omitempty"`
	SalaryCurrency string        `bson:"salary_currency" json:"salary_currency"`
	Quantity       int           `bson:"quantity" json:"quantity"`
	ViewCount      int           `bson:"view_count" json:"viewCount"`
	WorkType       string        `bson:"work_type" json:"work_type"`
	WorkSchedule   string        `bson:"work_schedule,omitempty" json:"work_schedule,omitempty"`
	PostDate       string        `bson:"post_date,omitempty" json:"post_date,omitempty"`
	Deadline       string        `bson:"deadline" json:"deadline"`
	DeadlineDate   *time.Time    `bson:"deadline_date,omitempty" json:"deadline_date,omitempty"`
	Location       string        `bson:"location" json:"location"`
	StreetAddr     string        `bson:"street_address,omitempty" json:"street_address,omitempty"`
	City           string        `bson:"city,omitempty" json:"city,omitempty"`
	Country        string        `bson:"country" json:"country"`
	EmpType        string        `bson:"employment_type" json:"employment_type"` // FULL_TIME, PART_TIME, CONTRACT, INTERN
	Summary        string        `bson:"summary" json:"summary"`
	Description    []string      `bson:"description" json:"description"` // array of bullet strings
	Requirements   []string      `bson:"requirements" json:"requirements"`
	Benefits       []string      `bson:"benefits" json:"benefits"`
	IsActive       bool          `bson:"is_active" json:"is_active"`
	CreatedAt      time.Time     `bson:"created_at" json:"createdAt"`
	UpdatedAt      time.Time     `bson:"updated_at" json:"updatedAt"`
	CreatedBy      string        `bson:"created_by,omitempty" json:"created_by,omitempty"`
}

// JobCollection is the MongoDB collection name.
const JobCollection = "jobs"
