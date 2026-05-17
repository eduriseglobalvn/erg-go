// Package repository provides MongoDB data access for the courses module.
package repository

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"erg.ninja/internal/modules/courses/domain/entity"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
)

// Repository provides data access for courses.
type Repository struct {
	client      *mongo.Client
	dbName      string
	courses     *mongo.Collection
	syllabus    *mongo.Collection
	instructors *mongo.Collection
	lessons     *mongo.Collection
	enrollments *mongo.Collection
	log         *logger.Logger
}

// RepositoryOption configures the Repository.
type RepositoryOption func(*Repository)

// WithRepositoryLogger sets the logger.
func WithRepositoryLogger(log *logger.Logger) RepositoryOption {
	return func(r *Repository) { r.log = log }
}

// NewRepository creates a new courses repository.
func NewRepository(mongo *database.MongoClient, opts ...RepositoryOption) *Repository {
	r := &Repository{
		client:      mongo.Client(),
		dbName:      mongo.DatabaseName(),
		courses:     mongo.Collection(entities.CourseCollection),
		syllabus:    mongo.Collection(entities.SyllabusCollection),
		instructors: mongo.Collection(entities.InstructorCollection),
		lessons:     mongo.Collection(entities.LessonCollection),
		enrollments: mongo.Collection(entities.EnrollmentCollection),
		log:         logger.NoOp(),
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

// CourseListParams controls course listing.
type CourseListParams struct {
	TenantID string
	Status   string
	Limit    int64
	Offset   int64
	Search   string
}

// ─── Course CRUD ───────────────────────────────────────────────────────────────

// Create inserts a new course.
func (r *Repository) Create(ctx context.Context, c *entities.Course) error {
	c.CreatedAt = time.Now().UTC()
	c.UpdatedAt = c.CreatedAt
	_, err := r.courses.InsertOne(ctx, c)
	if err != nil {
		return fmt.Errorf("courses.Create: %w", err)
	}
	return nil
}

// GetByID retrieves a course by ID.
func (r *Repository) GetByID(ctx context.Context, id string) (*entities.Course, error) {
	var c entities.Course
	err := r.courses.FindOne(ctx, bson.M{"_id": id}).Decode(&c)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("courses.GetByID: %w", err)
	}
	return &c, nil
}

// GetBySlug retrieves a course by tenant + slug.
func (r *Repository) GetBySlug(ctx context.Context, tenantID, slug string) (*entities.Course, error) {
	var c entities.Course
	err := r.courses.FindOne(ctx, bson.M{"tenant_id": tenantID, "slug": slug}).Decode(&c)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("courses.GetBySlug: %w", err)
	}
	return &c, nil
}

// GetBySubdomain retrieves a course by subdomain (public endpoint).
func (r *Repository) GetBySubdomain(ctx context.Context, subdomain string) (*entities.Course, error) {
	var c entities.Course
	err := r.courses.FindOne(ctx, bson.M{
		"subdomain": subdomain,
		"status":    entities.CourseStatusPublished,
	}).Decode(&c)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("courses.GetBySubdomain: %w", err)
	}
	return &c, nil
}

// Update updates a course.
func (r *Repository) Update(ctx context.Context, id string, update bson.M) error {
	update["updated_at"] = time.Now().UTC()
	_, err := r.courses.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": update})
	if err != nil {
		return fmt.Errorf("courses.Update: %w", err)
	}
	return nil
}

// UpdateThemeConfig updates only the theme config field.
func (r *Repository) UpdateThemeConfig(ctx context.Context, id, themeJSON string) error {
	return r.Update(ctx, id, bson.M{"theme_config": themeJSON})
}

// Delete removes a course.
func (r *Repository) Delete(ctx context.Context, id string) error {
	_, err := r.courses.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("courses.Delete: %w", err)
	}
	return nil
}

// List returns paginated courses.
func (r *Repository) List(ctx context.Context, p CourseListParams) ([]*entities.Course, int64, error) {
	filter := bson.M{"tenant_id": p.TenantID}
	if p.Status != "" {
		filter["status"] = p.Status
	}
	if p.Search != "" {
		filter["$or"] = []bson.M{
			{"title": bson.M{"$regex": p.Search, "$options": "i"}},
			{"description": bson.M{"$regex": p.Search, "$options": "i"}},
		}
	}

	if p.Limit <= 0 {
		p.Limit = 20
	}

	total, err := r.courses.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("courses.List count: %w", err)
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(p.Limit).
		SetSkip(p.Offset)

	cursor, err := r.courses.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("courses.List: %w", err)
	}
	defer cursor.Close(ctx)

	var results []*entities.Course
	if err := cursor.All(ctx, &results); err != nil {
		return nil, 0, fmt.Errorf("courses.List decode: %w", err)
	}
	return results, total, nil
}

// IncrementEnrollment increments the enrollment count for a course.
func (r *Repository) IncrementEnrollment(ctx context.Context, courseID string, delta int) error {
	_, err := r.courses.UpdateOne(ctx, bson.M{"_id": courseID}, bson.M{
		"$inc": bson.M{"enrollment_count": delta},
		"$set": bson.M{"updated_at": time.Now().UTC()},
	})
	if err != nil {
		return fmt.Errorf("courses.IncrementEnrollment: %w", err)
	}
	return nil
}

// UpdateRating updates the average rating for a course.
func (r *Repository) UpdateRating(ctx context.Context, courseID string, avg float64) error {
	return r.Update(ctx, courseID, bson.M{"rating_avg": avg})
}

// ─── Syllabus ─────────────────────────────────────────────────────────────────

// UpsertSyllabus creates or replaces syllabus sections for a course.
func (r *Repository) UpsertSyllabus(ctx context.Context, courseID string, sections []*entities.CourseSyllabus) error {
	// Delete existing sections.
	_, err := r.syllabus.DeleteMany(ctx, bson.M{"course_id": courseID})
	if err != nil {
		return fmt.Errorf("courses.UpsertSyllabus delete: %w", err)
	}
	if len(sections) == 0 {
		return nil
	}
	docs := make([]any, len(sections))
	for i, s := range sections {
		s.CourseID = bson.NewObjectID()
		docs[i] = s
	}
	_, err = r.syllabus.InsertMany(ctx, docs)
	if err != nil {
		return fmt.Errorf("courses.UpsertSyllabus insert: %w", err)
	}
	return nil
}

// ListSyllabus returns all syllabus sections for a course, ordered by order field.
func (r *Repository) ListSyllabus(ctx context.Context, courseID string) ([]*entities.CourseSyllabus, error) {
	opts := options.Find().SetSort(bson.D{{Key: "order", Value: 1}})
	cursor, err := r.syllabus.Find(ctx, bson.M{"course_id": courseID}, opts)
	if err != nil {
		return nil, fmt.Errorf("courses.ListSyllabus: %w", err)
	}
	defer cursor.Close(ctx)
	var results []*entities.CourseSyllabus
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("courses.ListSyllabus decode: %w", err)
	}
	return results, nil
}

// ─── Instructor ───────────────────────────────────────────────────────────────

// CreateInstructor inserts a new instructor profile.
func (r *Repository) CreateInstructor(ctx context.Context, i *entities.CourseInstructor) error {
	_, err := r.instructors.InsertOne(ctx, i)
	if err != nil {
		return fmt.Errorf("courses.CreateInstructor: %w", err)
	}
	return nil
}

// GetInstructor retrieves an instructor by ID.
func (r *Repository) GetInstructor(ctx context.Context, id string) (*entities.CourseInstructor, error) {
	var i entities.CourseInstructor
	err := r.instructors.FindOne(ctx, bson.M{"_id": id}).Decode(&i)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("courses.GetInstructor: %w", err)
	}
	return &i, nil
}

// ─── Lessons ─────────────────────────────────────────────────────────────────

// CreateLesson inserts a new lesson.
func (r *Repository) CreateLesson(ctx context.Context, l *entities.CourseLesson) error {
	_, err := r.lessons.InsertOne(ctx, l)
	if err != nil {
		return fmt.Errorf("courses.CreateLesson: %w", err)
	}
	return nil
}

// ListLessons returns all lessons for a course, ordered by order field.
func (r *Repository) ListLessons(ctx context.Context, courseID string) ([]*entities.CourseLesson, error) {
	opts := options.Find().SetSort(bson.D{{Key: "order", Value: 1}})
	cursor, err := r.lessons.Find(ctx, bson.M{"course_id": courseID}, opts)
	if err != nil {
		return nil, fmt.Errorf("courses.ListLessons: %w", err)
	}
	defer cursor.Close(ctx)
	var results []*entities.CourseLesson
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("courses.ListLessons decode: %w", err)
	}
	return results, nil
}

// ReorderLessons updates the order of lessons for a course.
func (r *Repository) ReorderLessons(ctx context.Context, courseID string, orderedIDs []string) error {
	for i, id := range orderedIDs {
		_, err := r.lessons.UpdateOne(ctx, bson.M{"_id": id, "course_id": courseID}, bson.M{
			"$set": bson.M{"order": i},
		})
		if err != nil {
			return fmt.Errorf("courses.ReorderLessons: %w", err)
		}
	}
	return nil
}

// ─── Enrollments ─────────────────────────────────────────────────────────────

// CreateEnrollment creates a new enrollment.
func (r *Repository) CreateEnrollment(ctx context.Context, e *entities.CourseEnrollment) error {
	_, err := r.enrollments.InsertOne(ctx, e)
	if err != nil {
		return fmt.Errorf("courses.CreateEnrollment: %w", err)
	}
	return nil
}

// GetEnrollment checks if a user is enrolled in a course.
func (r *Repository) GetEnrollment(ctx context.Context, courseID, userID string) (*entities.CourseEnrollment, error) {
	var e entities.CourseEnrollment
	err := r.enrollments.FindOne(ctx, bson.M{"course_id": courseID, "user_id": userID}).Decode(&e)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("courses.GetEnrollment: %w", err)
	}
	return &e, nil
}

// UpdateProgress updates the progress percentage for an enrollment.
func (r *Repository) UpdateProgress(ctx context.Context, enrollmentID string, percent int) error {
	update := bson.M{"progress_percent": percent}
	if percent >= 100 {
		now := time.Now().UTC()
		update["completed_at"] = now
	}
	_, err := r.enrollments.UpdateOne(ctx, bson.M{"_id": enrollmentID}, bson.M{"$set": update})
	if err != nil {
		return fmt.Errorf("courses.UpdateProgress: %w", err)
	}
	return nil
}
