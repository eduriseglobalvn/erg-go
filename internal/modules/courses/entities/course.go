// Package entities provides the domain models for the courses module.
package entities

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Course status constants.
const (
	CourseStatusDraft     = "DRAFT"
	CourseStatusPublished = "PUBLISHED"
	CourseStatusArchived  = "ARCHIVED"
)

// Course represents a course document in MongoDB.
type Course struct {
	ID              bson.ObjectID `bson:"_id,omitempty" json:"id"`
	TenantID        string        `bson:"tenant_id" json:"tenant_id"`
	Title           string        `bson:"title" json:"title"`
	Slug            string        `bson:"slug" json:"slug"`
	Subdomain       string        `bson:"subdomain" json:"subdomain"`
	Description     string        `bson:"description" json:"description"`
	ThumbnailURL    string        `bson:"thumbnail_url" json:"thumbnailUrl"`
	InstructorID    string        `bson:"instructor_id" json:"instructor_id"`
	Status          string        `bson:"status" json:"status"`
	EnrollmentCount int           `bson:"enrollment_count" json:"enrollment_count"`
	RatingAvg       float64       `bson:"rating_avg" json:"rating_avg"`
	SchemaType      string        `bson:"schema_type" json:"schemaType"`
	SchemaDataJSON  string        `bson:"schema_data_json,omitempty" json:"schema_data_json,omitempty"`
	ThemeConfig     string        `bson:"theme_config,omitempty" json:"theme_config,omitempty"`
	CreatedAt       time.Time     `bson:"created_at" json:"createdAt"`
	UpdatedAt       time.Time     `bson:"updated_at" json:"updatedAt"`
}

// CourseCollection is the MongoDB collection name.
const CourseCollection = "courses"

// CourseSyllabus represents a syllabus section within a course.
type CourseSyllabus struct {
	ID       bson.ObjectID `bson:"_id,omitempty" json:"id"`
	CourseID bson.ObjectID `bson:"course_id" json:"course_id"`
	Order    int           `bson:"order" json:"order"`
	Title    string        `bson:"title" json:"title"`
	Content  string        `bson:"content" json:"content"`
}

// SyllabusCollection is the MongoDB collection name.
const SyllabusCollection = "course_syllabus"

// CourseInstructor represents an instructor profile linked to a course.
type CourseInstructor struct {
	ID        bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Name      string        `bson:"name" json:"name"`
	Bio       string        `bson:"bio" json:"bio"`
	AvatarURL string        `bson:"avatar_url" json:"avatarUrl"`
	Title     string        `bson:"title" json:"title"`
}

// InstructorCollection is the MongoDB collection name.
const InstructorCollection = "course_instructors"

// CourseLesson represents a lesson within a course.
type CourseLesson struct {
	ID              bson.ObjectID `bson:"_id,omitempty" json:"id"`
	CourseID        bson.ObjectID `bson:"course_id" json:"course_id"`
	Order           int           `bson:"order" json:"order"`
	Title           string        `bson:"title" json:"title"`
	Content         string        `bson:"content" json:"content"`
	DurationMinutes int           `bson:"duration_minutes" json:"duration_minutes"`
	VideoURL        string        `bson:"video_url" json:"video_url"`
}

// LessonCollection is the MongoDB collection name.
const LessonCollection = "course_lessons"

// CourseEnrollment represents a user enrollment in a course.
type CourseEnrollment struct {
	ID              bson.ObjectID `bson:"_id,omitempty" json:"id"`
	CourseID        bson.ObjectID `bson:"course_id" json:"course_id"`
	UserID          bson.ObjectID `bson:"user_id" json:"user_id"`
	EnrolledAt      time.Time     `bson:"enrolled_at" json:"enrolled_at"`
	CompletedAt     *time.Time    `bson:"completed_at,omitempty" json:"completed_at,omitempty"`
	ProgressPercent int           `bson:"progress_percent" json:"progress_percent"`
}

// EnrollmentCollection is the MongoDB collection name.
const EnrollmentCollection = "course_enrollments"
