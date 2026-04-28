// Package service provides the business logic for the courses module.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"erg.ninja/internal/modules/courses/dto"
	"erg.ninja/internal/modules/courses/entities"
	"erg.ninja/internal/modules/courses/repository"
	"erg.ninja/pkg/logger"
)

// Service provides course business logic.
type Service struct {
	repo *repository.Repository
	log  *logger.Logger
}

// ServiceOption configures the Service.
type ServiceOption func(*Service)

// WithCourseLogger sets the logger.
func WithCourseLogger(log *logger.Logger) ServiceOption {
	return func(s *Service) { s.log = log }
}

// NewService creates a new courses service.
func NewService(repo *repository.Repository, opts ...ServiceOption) *Service {
	s := &Service{repo: repo, log: logger.NoOp()}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Create creates a new course.
func (s *Service) Create(ctx context.Context, tenantID string, req dto.CreateCourseRequest) (*entities.Course, error) {
	// Check slug uniqueness within tenant.
	existing, err := s.repo.GetBySlug(ctx, tenantID, req.Slug)
	if err != nil {
		return nil, fmt.Errorf("courses.Create: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("course with slug %q already exists", req.Slug)
	}

	c := &entities.Course{
		ID:             bson.NewObjectID(),
		TenantID:       tenantID,
		Title:          req.Title,
		Slug:           req.Slug,
		Subdomain:      req.Subdomain,
		Description:    req.Description,
		ThumbnailURL:   req.ThumbnailURL,
		InstructorID:   req.InstructorID,
		Status:         entities.CourseStatusDraft,
		SchemaType:     req.SchemaType,
		SchemaDataJSON: req.SchemaDataJSON,
		ThemeConfig:    req.ThemeConfig,
	}
	if c.SchemaType == "" {
		c.SchemaType = "Course"
	}

	if err := s.repo.Create(ctx, c); err != nil {
		return nil, fmt.Errorf("courses.Create: %w", err)
	}
	s.log.InfoContext(ctx).Str("id", c.ID.Hex()).Str("slug", c.Slug).Msg("courses: created")
	return c, nil
}

// GetByID returns a course by ID.
func (s *Service) GetByID(ctx context.Context, id string) (*entities.Course, error) {
	c, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("courses.GetByID: %w", err)
	}
	return c, nil
}

// GetBySubdomain returns a published course by subdomain (public).
func (s *Service) GetBySubdomain(ctx context.Context, subdomain string) (*entities.Course, error) {
	c, err := s.repo.GetBySubdomain(ctx, subdomain)
	if err != nil {
		return nil, fmt.Errorf("courses.GetBySubdomain: %w", err)
	}
	return c, nil
}

// List returns paginated courses for a tenant.
func (s *Service) List(ctx context.Context, tenantID string, params repository.CourseListParams) ([]*entities.Course, int64, error) {
	params.TenantID = tenantID
	return s.repo.List(ctx, params)
}

// Update updates a course.
func (s *Service) Update(ctx context.Context, id string, req dto.UpdateCourseRequest) (*entities.Course, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("courses.Update: %w", err)
	}
	if existing == nil {
		return nil, fmt.Errorf("course not found")
	}

	update := map[string]interface{}{}
	if req.Title != nil {
		update["title"] = *req.Title
	}
	if req.Slug != nil {
		// Check uniqueness.
		existing, err := s.repo.GetBySlug(ctx, existing.TenantID, *req.Slug)
		if err != nil {
			return nil, fmt.Errorf("courses.Update: %w", err)
		}
		if existing != nil && existing.ID.Hex() != id {
			return nil, fmt.Errorf("course with slug %q already exists", *req.Slug)
		}
		update["slug"] = *req.Slug
	}
	if req.Subdomain != nil {
		update["subdomain"] = *req.Subdomain
	}
	if req.Description != nil {
		update["description"] = *req.Description
	}
	if req.ThumbnailURL != nil {
		update["thumbnail_url"] = *req.ThumbnailURL
	}
	if req.InstructorID != nil {
		update["instructor_id"] = *req.InstructorID
	}
	if req.Status != nil {
		update["status"] = *req.Status
	}
	if req.SchemaType != nil {
		update["schema_type"] = *req.SchemaType
	}
	if req.SchemaDataJSON != nil {
		update["schema_data_json"] = *req.SchemaDataJSON
	}

	if len(update) == 0 {
		return existing, nil
	}

	if err := s.repo.Update(ctx, id, update); err != nil {
		return nil, fmt.Errorf("courses.Update: %w", err)
	}
	return s.repo.GetByID(ctx, id)
}

// UpdateTheme updates only the theme config.
func (s *Service) UpdateTheme(ctx context.Context, id, themeJSON string) error {
	if err := s.repo.UpdateThemeConfig(ctx, id, themeJSON); err != nil {
		return fmt.Errorf("courses.UpdateTheme: %w", err)
	}
	s.log.InfoContext(ctx).Str("id", id).Msg("courses: theme updated")
	return nil
}

// Delete removes a course and all related data.
func (s *Service) Delete(ctx context.Context, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("courses.Delete: %w", err)
	}
	s.log.InfoContext(ctx).Str("id", id).Msg("courses: deleted")
	return nil
}

// GetDetail returns full course detail including lessons and syllabus.
func (s *Service) GetDetail(ctx context.Context, id string) (*dto.CourseDetailResponse, error) {
	c, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("courses.GetDetail: %w", err)
	}
	if c == nil {
		return nil, nil
	}

	lessons, _ := s.repo.ListLessons(ctx, id)
	syllabus, _ := s.repo.ListSyllabus(ctx, id)

	var instructor *entities.CourseInstructor
	if c.InstructorID != "" {
		instructor, _ = s.repo.GetInstructor(ctx, c.InstructorID)
	}

	lessonDTOs := make([]dto.LessonResponse, len(lessons))
	for i, l := range lessons {
		lessonDTOs[i] = dto.ToLessonResponse(l)
	}
	syllabusDTOs := make([]dto.SyllabusResponse, len(syllabus))
	for i, sec := range syllabus {
		syllabusDTOs[i] = dto.ToSyllabusResponse(sec)
	}

	return &dto.CourseDetailResponse{
		CourseResponse: dto.ToResponse(c),
		Instructor:     instructor,
		Lessons:        lessonDTOs,
		Syllabus:       syllabusDTOs,
	}, nil
}

// GetSchemaMarkup generates JSON-LD schema.org markup for a course.
func (s *Service) GetSchemaMarkup(ctx context.Context, id string) (*dto.CourseSchemaResponse, error) {
	c, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("courses.GetSchemaMarkup: %w", err)
	}
	if c == nil {
		return nil, nil
	}

	schemaType := c.SchemaType
	if schemaType == "" {
		schemaType = "Course"
	}

	schema := map[string]interface{}{
		"@context":    "https://schema.org",
		"@type":       schemaType,
		"name":        c.Title,
		"description": c.Description,
	}
	if c.ThumbnailURL != "" {
		schema["image"] = c.ThumbnailURL
	}
	if c.SchemaDataJSON != "" {
		// Merge any extra schema data.
		var extra map[string]interface{}
		if err := json.Unmarshal([]byte(c.SchemaDataJSON), &extra); err == nil {
			for k, v := range extra {
				schema[k] = v
			}
		}
	}

	schemaJSON, _ := json.Marshal(schema)
	return &dto.CourseSchemaResponse{
		Course:     dto.ToResponse(c),
		SchemaJSON: string(schemaJSON),
	}, nil
}

// ReorderLessons updates lesson order.
func (s *Service) ReorderLessons(ctx context.Context, courseID string, orderedIDs []string) error {
	// Verify course exists.
	c, err := s.repo.GetByID(ctx, courseID)
	if err != nil {
		return fmt.Errorf("courses.ReorderLessons: %w", err)
	}
	if c == nil {
		return fmt.Errorf("course not found")
	}
	if err := s.repo.ReorderLessons(ctx, courseID, orderedIDs); err != nil {
		return fmt.Errorf("courses.ReorderLessons: %w", err)
	}
	s.log.InfoContext(ctx).Str("course_id", courseID).Msg("courses: lessons reordered")
	return nil
}

// CreateLesson adds a lesson to a course.
func (s *Service) CreateLesson(ctx context.Context, courseID string, title, content, videoURL string, duration int) (*entities.CourseLesson, error) {
	existing, err := s.repo.GetByID(ctx, courseID)
	if err != nil {
		return nil, fmt.Errorf("courses.CreateLesson: %w", err)
	}
	if existing == nil {
		return nil, fmt.Errorf("course not found")
	}

	// Get current max order.
	lessons, _ := s.repo.ListLessons(ctx, courseID)
	order := len(lessons)

	l := &entities.CourseLesson{
		ID:              bson.NewObjectID(),
		CourseID:        bson.NewObjectID(),
		Order:           order,
		Title:           title,
		Content:         content,
		DurationMinutes: duration,
		VideoURL:        videoURL,
	}
	if err := s.repo.CreateLesson(ctx, l); err != nil {
		return nil, fmt.Errorf("courses.CreateLesson: %w", err)
	}
	return l, nil
}

// Enroll enrolls a user in a course.
func (s *Service) Enroll(ctx context.Context, courseID, userID string) error {
	existing, err := s.repo.GetEnrollment(ctx, courseID, userID)
	if err != nil {
		return fmt.Errorf("courses.Enroll: %w", err)
	}
	if existing != nil {
		return fmt.Errorf("already enrolled")
	}

	enrollment := &entities.CourseEnrollment{
		ID:              bson.NewObjectID(),
		CourseID:        bson.NewObjectID(),
		UserID:          bson.NewObjectID(),
		EnrolledAt:      time.Now().UTC(),
		ProgressPercent: 0,
	}
	if err := s.repo.CreateEnrollment(ctx, enrollment); err != nil {
		return fmt.Errorf("courses.Enroll: %w", err)
	}
	if err := s.repo.IncrementEnrollment(ctx, courseID, 1); err != nil {
		s.log.WarnContext(ctx).Err(err).Str("course_id", courseID).Msg("courses: enrollment count update failed")
	}
	s.log.InfoContext(ctx).Str("course_id", courseID).Str("user_id", userID).Msg("courses: user enrolled")
	return nil
}

// NormalizeSlug converts a title to a URL-safe slug.
func NormalizeSlug(title string) string {
	slug := strings.ToLower(title)
	replacer := strings.NewReplacer(" ", "-", "_", "-")
	slug = replacer.Replace(slug)
	slug = strings.Trim(slug, "-")
	return slug
}
