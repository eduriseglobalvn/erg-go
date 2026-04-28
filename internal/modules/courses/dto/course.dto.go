// Package dto provides request/response types for the courses module.
package dto

import (
	"time"

	"github.com/go-playground/validator/v10"

	"erg.ninja/internal/modules/courses/entities"
)

// CreateCourseRequest is the payload for POST /courses.
type CreateCourseRequest struct {
	Title          string `json:"title" validate:"required,min=3,max=200"`
	Slug           string `json:"slug" validate:"required,max=100"`
	Subdomain      string `json:"subdomain" validate:"max=100"`
	Description    string `json:"description" validate:"max=5000"`
	ThumbnailURL   string `json:"thumbnailUrl" validate:"omitempty,url"`
	InstructorID   string `json:"instructorId"`
	SchemaType     string `json:"schemaType"` // Course, OnlineCourse
	SchemaDataJSON string `json:"schemaDataJson,omitempty"`
	ThemeConfig    string `json:"themeConfig,omitempty"`
}

// UpdateCourseRequest is the payload for PATCH /courses/:id.
type UpdateCourseRequest struct {
	Title          *string `json:"title,omitempty" validate:"omitempty,min=3,max=200"`
	Slug           *string `json:"slug,omitempty" validate:"omitempty,max=100"`
	Subdomain      *string `json:"subdomain,omitempty" validate:"omitempty,max=100"`
	Description    *string `json:"description,omitempty" validate:"omitempty,max=5000"`
	ThumbnailURL   *string `json:"thumbnailUrl,omitempty" validate:"omitempty,url"`
	InstructorID   *string `json:"instructorId,omitempty"`
	Status         *string `json:"status,omitempty"`
	SchemaType     *string `json:"schemaType,omitempty"`
	SchemaDataJSON *string `json:"schemaDataJson,omitempty"`
}

// UpdateThemeRequest is the payload for PATCH /courses/:id/theme.
type UpdateThemeRequest struct {
	ThemeConfig string `json:"themeConfig" validate:"required"`
}

// LessonReorderRequest is the payload for POST /courses/:id/lessons/reorder.
type LessonReorderRequest struct {
	OrderedLessonIDs []string `json:"orderedLessonIds" validate:"required,min=1"`
}

// CourseListResponse is the paginated list response.
type CourseListResponse struct {
	Items []CourseResponse `json:"items"`
	Total int64            `json:"total"`
	Page  int              `json:"page"`
	Limit int              `json:"limit"`
}

// CourseResponse is the standard course representation.
type CourseResponse struct {
	ID              string    `json:"id"`
	TenantID        string    `json:"tenantId"`
	Title           string    `json:"title"`
	Slug            string    `json:"slug"`
	Subdomain       string    `json:"subdomain"`
	Description     string    `json:"description"`
	ThumbnailURL    string    `json:"thumbnailUrl"`
	InstructorID    string    `json:"instructorId"`
	Status          string    `json:"status"`
	EnrollmentCount int       `json:"enrollmentCount"`
	RatingAvg       float64   `json:"ratingAvg"`
	SchemaType      string    `json:"schemaType"`
	SchemaDataJSON  string    `json:"schemaDataJson,omitempty"`
	ThemeConfig     string    `json:"themeConfig,omitempty"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

// CourseDetailResponse includes lessons and syllabus for GET /courses/:id.
type CourseDetailResponse struct {
	CourseResponse
	Instructor *entities.CourseInstructor `json:"instructor,omitempty"`
	Lessons    []LessonResponse           `json:"lessons,omitempty"`
	Syllabus   []SyllabusResponse         `json:"syllabus,omitempty"`
}

// CourseSchemaResponse wraps the JSON-LD schema markup for SEO.
type CourseSchemaResponse struct {
	Course     CourseResponse `json:"course"`
	SchemaJSON string         `json:"schemaJson"`
}

// LessonResponse is the lesson representation.
type LessonResponse struct {
	ID              string `json:"id"`
	CourseID        string `json:"courseId"`
	Order           int    `json:"order"`
	Title           string `json:"title"`
	Content         string `json:"content"`
	DurationMinutes int    `json:"durationMinutes"`
	VideoURL        string `json:"videoUrl"`
}

// SyllabusResponse is the syllabus section representation.
type SyllabusResponse struct {
	ID       string `json:"id"`
	CourseID string `json:"courseId"`
	Order    int    `json:"order"`
	Title    string `json:"title"`
	Content  string `json:"content"`
}

// ToResponse converts an entity to a response DTO.
func ToResponse(c *entities.Course) CourseResponse {
	return CourseResponse{
		ID:              c.ID.Hex(),
		TenantID:        c.TenantID,
		Title:           c.Title,
		Slug:            c.Slug,
		Subdomain:       c.Subdomain,
		Description:     c.Description,
		ThumbnailURL:    c.ThumbnailURL,
		InstructorID:    c.InstructorID,
		Status:          c.Status,
		EnrollmentCount: c.EnrollmentCount,
		RatingAvg:       c.RatingAvg,
		SchemaType:      c.SchemaType,
		SchemaDataJSON:  c.SchemaDataJSON,
		ThemeConfig:     c.ThemeConfig,
		CreatedAt:       c.CreatedAt,
		UpdatedAt:       c.UpdatedAt,
	}
}

// ToResponses converts a slice of entities.
func ToResponses(courses []*entities.Course) []CourseResponse {
	items := make([]CourseResponse, len(courses))
	for i, c := range courses {
		items[i] = ToResponse(c)
	}
	return items
}

// ToLessonResponse converts a lesson entity.
func ToLessonResponse(l *entities.CourseLesson) LessonResponse {
	return LessonResponse{
		ID:              l.ID.Hex(),
		CourseID:        l.CourseID.Hex(),
		Order:           l.Order,
		Title:           l.Title,
		Content:         l.Content,
		DurationMinutes: l.DurationMinutes,
		VideoURL:        l.VideoURL,
	}
}

// ToSyllabusResponse converts a syllabus entity.
func ToSyllabusResponse(s *entities.CourseSyllabus) SyllabusResponse {
	return SyllabusResponse{
		ID:       s.ID.Hex(),
		CourseID: s.CourseID.Hex(),
		Order:    s.Order,
		Title:    s.Title,
		Content:  s.Content,
	}
}

// Validate validates a struct using go-playground/validator.
func Validate(s interface{}) error {
	v := validator.New()
	return v.Struct(s)
}
