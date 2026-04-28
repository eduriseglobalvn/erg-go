// Package repository provides MongoDB data access for the recruitment module.
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"erg.ninja/internal/modules/recruitment/entities"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
)

var ErrJobNotFound = errors.New("recruitment: job not found")
var ErrCandidateNotFound = errors.New("recruitment: candidate not found")

// Repository provides MongoDB data access for recruitment.
type Repository struct {
	jobs       *mongo.Collection
	candidates *mongo.Collection
	log        *logger.Logger
}

// RepositoryOption configures the Repository.
type RepositoryOption func(*Repository)

// WithRecruitmentLogger sets the logger.
func WithRecruitmentLogger(log *logger.Logger) RepositoryOption {
	return func(r *Repository) { r.log = log }
}

// NewRepository creates a new recruitment repository.
func NewRepository(mongo *database.MongoClient, opts ...RepositoryOption) *Repository {
	r := &Repository{
		jobs:       mongo.Collection(entities.JobCollection),
		candidates: mongo.Collection(entities.CandidateCollection),
		log:        logger.NoOp(),
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

// ─── Job CRUD ─────────────────────────────────────────────────────────────────

// CreateJob inserts a new job.
func (r *Repository) CreateJob(ctx context.Context, job *entities.Job) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if job.ID.IsZero() {
		job.ID = bson.NewObjectID()
	}
	job.CreatedAt = time.Now().UTC()
	job.UpdatedAt = job.CreatedAt

	_, err := r.jobs.InsertOne(ctx, job)
	if err != nil {
		return fmt.Errorf("recruitment.CreateJob: %w", err)
	}
	return nil
}

// GetJobByID retrieves a job by its MongoDB ObjectID string.
func (r *Repository) GetJobByID(ctx context.Context, id string) (*entities.Job, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	objID, ok := database.ParseObjectID(id)
	if !ok {
		return nil, ErrJobNotFound
	}

	var job entities.Job
	err := r.jobs.FindOne(ctx, bson.M{"_id": objID}).Decode(&job)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrJobNotFound
		}
		return nil, fmt.Errorf("recruitment.GetJobByID: %w", err)
	}
	return &job, nil
}

// GetJobBySlug retrieves an active job by slug.
func (r *Repository) GetJobBySlug(ctx context.Context, slug string) (*entities.Job, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var job entities.Job
	err := r.jobs.FindOne(ctx, bson.M{"slug": slug, "is_active": true}).Decode(&job)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrJobNotFound
		}
		return nil, fmt.Errorf("recruitment.GetJobBySlug: %w", err)
	}
	return &job, nil
}

// ListJobsParams controls job listing.
type ListJobsParams struct {
	Search   string
	Salary   []string
	WorkType []string
	Location []string
	Sort     string // "newest" or "oldest"
	Page     int
	Limit    int
}

// ListJobs returns paginated active jobs with optional filters.
func (r *Repository) ListJobs(ctx context.Context, p ListJobsParams) ([]*entities.Job, int64, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if p.Limit <= 0 || p.Limit > 100 {
		p.Limit = 10
	}
	if p.Page < 1 {
		p.Page = 1
	}

	filter := bson.M{"is_active": true}

	if p.Search != "" {
		filter["$or"] = []bson.M{
			{"title": bson.M{"$regex": p.Search, "$options": "i"}},
			{"summary": bson.M{"$regex": p.Search, "$options": "i"}},
		}
	}
	if len(p.Salary) > 0 {
		filter["salary"] = bson.M{"$in": p.Salary}
	}
	if len(p.WorkType) > 0 {
		filter["work_type"] = bson.M{"$in": p.WorkType}
	}
	if len(p.Location) > 0 {
		filter["location"] = bson.M{"$in": p.Location}
	}

	order := -1 // newest first
	if p.Sort == "oldest" {
		order = 1
	}

	skip := int64((p.Page - 1) * p.Limit)
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: order}}).
		SetSkip(skip).
		SetLimit(int64(p.Limit))

	total, err := r.jobs.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("recruitment.ListJobs count: %w", err)
	}

	cur, err := r.jobs.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("recruitment.ListJobs: %w", err)
	}
	defer cur.Close(ctx)

	var jobs []*entities.Job
	if err := cur.All(ctx, &jobs); err != nil {
		return nil, 0, fmt.Errorf("recruitment.ListJobs decode: %w", err)
	}
	return jobs, total, nil
}

// UpdateJobFields updates only the non-nil fields of a job.
func (r *Repository) UpdateJobFields(ctx context.Context, id string, updates map[string]any) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	objID, ok := database.ParseObjectID(id)
	if !ok {
		return ErrJobNotFound
	}

	updates["updated_at"] = time.Now().UTC()
	result, err := r.jobs.UpdateOne(ctx, bson.M{"_id": objID}, bson.M{"$set": updates})
	if err != nil {
		return fmt.Errorf("recruitment.UpdateJobFields: %w", err)
	}
	if result.MatchedCount == 0 {
		return ErrJobNotFound
	}
	return nil
}

// SoftDeleteJob sets is_active=false.
func (r *Repository) SoftDeleteJob(ctx context.Context, id string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	objID, ok := database.ParseObjectID(id)
	if !ok {
		return ErrJobNotFound
	}

	result, err := r.jobs.UpdateOne(ctx, bson.M{"_id": objID}, bson.M{
		"$set": bson.M{
			"is_active":  false,
			"updated_at": time.Now().UTC(),
		},
	})
	if err != nil {
		return fmt.Errorf("recruitment.SoftDeleteJob: %w", err)
	}
	if result.MatchedCount == 0 {
		return ErrJobNotFound
	}
	return nil
}

// IncrementViewCount increments the view count for a job.
func (r *Repository) IncrementViewCount(ctx context.Context, id string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	objID, ok := database.ParseObjectID(id)
	if !ok {
		return nil // non-fatal
	}

	_, err := r.jobs.UpdateOne(ctx, bson.M{"_id": objID}, bson.M{
		"$inc": bson.M{"view_count": 1},
		"$set": bson.M{"updated_at": time.Now().UTC()},
	})
	if err != nil {
		r.log.Warn().Err(err).Str("id", id).Msg("recruitment: IncrementViewCount failed")
	}
	return nil
}

// ─── Candidate CRUD ───────────────────────────────────────────────────────────

// CreateCandidate inserts a new candidate.
func (r *Repository) CreateCandidate(ctx context.Context, c *entities.Candidate) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if c.ID.IsZero() {
		c.ID = bson.NewObjectID()
	}
	c.CreatedAt = time.Now().UTC()
	c.UpdatedAt = c.CreatedAt

	_, err := r.candidates.InsertOne(ctx, c)
	if err != nil {
		return fmt.Errorf("recruitment.CreateCandidate: %w", err)
	}
	return nil
}

// GetCandidateByID retrieves a candidate by ID.
func (r *Repository) GetCandidateByID(ctx context.Context, id string) (*entities.Candidate, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	objID, ok := database.ParseObjectID(id)
	if !ok {
		return nil, ErrCandidateNotFound
	}

	var c entities.Candidate
	err := r.candidates.FindOne(ctx, bson.M{"_id": objID}).Decode(&c)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrCandidateNotFound
		}
		return nil, fmt.Errorf("recruitment.GetCandidateByID: %w", err)
	}
	return &c, nil
}

// GetCandidateByTrackingCode retrieves a candidate by tracking code (UUID).
func (r *Repository) GetCandidateByTrackingCode(ctx context.Context, code string) (*entities.Candidate, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var c entities.Candidate
	err := r.candidates.FindOne(ctx, bson.M{"tracking_code": code}).Decode(&c)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrCandidateNotFound
		}
		return nil, fmt.Errorf("recruitment.GetCandidateByTrackingCode: %w", err)
	}
	return &c, nil
}

// ListCandidates returns all candidates, optionally filtered by jobId.
func (r *Repository) ListCandidates(ctx context.Context, jobID string) ([]*entities.Candidate, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	filter := bson.M{}
	if jobID != "" {
		filter["job_id"] = jobID
	}

	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	cur, err := r.candidates.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("recruitment.ListCandidates: %w", err)
	}
	defer cur.Close(ctx)

	var candidates []*entities.Candidate
	if err := cur.All(ctx, &candidates); err != nil {
		return nil, fmt.Errorf("recruitment.ListCandidates decode: %w", err)
	}
	return candidates, nil
}

// UpdateCandidateFields updates only the non-nil fields of a candidate.
func (r *Repository) UpdateCandidateFields(ctx context.Context, id string, updates map[string]any) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	objID, ok := database.ParseObjectID(id)
	if !ok {
		return ErrCandidateNotFound
	}

	updates["updated_at"] = time.Now().UTC()
	result, err := r.candidates.UpdateOne(ctx, bson.M{"_id": objID}, bson.M{"$set": updates})
	if err != nil {
		return fmt.Errorf("recruitment.UpdateCandidateFields: %w", err)
	}
	if result.MatchedCount == 0 {
		return ErrCandidateNotFound
	}
	return nil
}

// GetJobTitleByID retrieves the title of a job for denormalised tracking display.
func (r *Repository) GetJobTitleByID(ctx context.Context, id string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	objID, ok := database.ParseObjectID(id)
	if !ok {
		return "", nil
	}

	var job entities.Job
	err := r.jobs.FindOne(ctx, bson.M{"_id": objID}).Decode(&job)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return "", nil
		}
		return "", fmt.Errorf("recruitment.GetJobTitleByID: %w", err)
	}
	return job.Title, nil
}
