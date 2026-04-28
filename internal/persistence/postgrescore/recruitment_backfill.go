package postgrescore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"gorm.io/gorm"

	"erg.ninja/pkg/database"
	"erg.ninja/pkg/logger"
)

// BackfillRecruitmentReport summarizes recruitment migration progress.
type BackfillRecruitmentReport struct {
	JobsSeen          int
	JobsCreated       int
	JobsUpdated       int
	CandidatesSeen    int
	CandidatesCreated int
	CandidatesUpdated int
}

type legacyRecruitmentJob struct {
	ID             bson.ObjectID `bson:"_id,omitempty"`
	TenantID       string        `bson:"tenant_id"`
	Slug           string        `bson:"slug"`
	Title          string        `bson:"title"`
	Status         string        `bson:"status"`
	IsHot          bool          `bson:"is_hot"`
	IsNew          bool          `bson:"is_new"`
	IsUrgent       bool          `bson:"is_urgent"`
	Salary         string        `bson:"salary"`
	SalaryMin      *float64      `bson:"salary_min,omitempty"`
	SalaryMax      *float64      `bson:"salary_max,omitempty"`
	SalaryCurrency string        `bson:"salary_currency"`
	Quantity       int           `bson:"quantity"`
	ViewCount      int           `bson:"view_count"`
	WorkType       string        `bson:"work_type"`
	WorkSchedule   string        `bson:"work_schedule,omitempty"`
	PostDate       string        `bson:"post_date,omitempty"`
	Deadline       string        `bson:"deadline"`
	DeadlineDate   *time.Time    `bson:"deadline_date,omitempty"`
	Location       string        `bson:"location"`
	StreetAddr     string        `bson:"street_address,omitempty"`
	City           string        `bson:"city,omitempty"`
	Country        string        `bson:"country"`
	EmpType        string        `bson:"employment_type"`
	Summary        string        `bson:"summary"`
	Description    []string      `bson:"description"`
	Requirements   []string      `bson:"requirements"`
	Benefits       []string      `bson:"benefits"`
	IsActive       bool          `bson:"is_active"`
	CreatedAt      time.Time     `bson:"created_at"`
	UpdatedAt      time.Time     `bson:"updated_at"`
	CreatedBy      string        `bson:"created_by,omitempty"`
}

type legacyRecruitmentCandidate struct {
	ID           bson.ObjectID `bson:"_id,omitempty"`
	JobID        string        `bson:"job_id,omitempty"`
	JobTitle     string        `bson:"job_title,omitempty"`
	TenantID     string        `bson:"tenant_id"`
	FullName     string        `bson:"full_name"`
	Email        string        `bson:"email"`
	Phone        string        `bson:"phone"`
	CVURL        string        `bson:"cv_url,omitempty"`
	CoverLetter  string        `bson:"cover_letter,omitempty"`
	Note         string        `bson:"note,omitempty"`
	PublicNote   string        `bson:"public_note,omitempty"`
	ApplyType    string        `bson:"apply_type"`
	Status       string        `bson:"status"`
	TrackingCode string        `bson:"tracking_code"`
	CreatedAt    time.Time     `bson:"created_at"`
	UpdatedAt    time.Time     `bson:"updated_at"`
}

// BackfillLegacyRecruitmentFromMongo copies legacy jobs/candidates from MongoDB
// into PostgreSQL so the recruitment module can switch storage without losing
// historical data.
func BackfillLegacyRecruitmentFromMongo(
	ctx context.Context,
	db *gorm.DB,
	mongoClient *database.MongoClient,
	log *logger.Logger,
) (*BackfillRecruitmentReport, error) {
	if db == nil || mongoClient == nil {
		return &BackfillRecruitmentReport{}, nil
	}
	if log == nil {
		log = logger.NoOp()
	}

	report := &BackfillRecruitmentReport{}
	if err := backfillRecruitmentJobs(ctx, db, mongoClient, report); err != nil {
		return nil, err
	}
	if err := backfillRecruitmentCandidates(ctx, db, mongoClient, report); err != nil {
		return nil, err
	}

	log.Info().
		Int("jobs_seen", report.JobsSeen).
		Int("jobs_created", report.JobsCreated).
		Int("jobs_updated", report.JobsUpdated).
		Int("candidates_seen", report.CandidatesSeen).
		Int("candidates_created", report.CandidatesCreated).
		Int("candidates_updated", report.CandidatesUpdated).
		Msg("postgrescore: legacy recruitment backfill complete")

	return report, nil
}

func backfillRecruitmentJobs(
	ctx context.Context,
	db *gorm.DB,
	mongoClient *database.MongoClient,
	report *BackfillRecruitmentReport,
) error {
	cursor, err := mongoClient.Collection("jobs").Find(
		ctx,
		bson.M{},
		options.Find().SetSort(bson.D{{Key: "updated_at", Value: 1}, {Key: "_id", Value: 1}}),
	)
	if err != nil {
		return fmt.Errorf("postgrescore.backfill.recruitment.jobs.find: %w", err)
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var legacy legacyRecruitmentJob
		if err := cursor.Decode(&legacy); err != nil {
			return fmt.Errorf("postgrescore.backfill.recruitment.jobs.decode: %w", err)
		}
		report.JobsSeen++
		if legacy.ID.IsZero() || strings.TrimSpace(legacy.Slug) == "" {
			continue
		}

		descJSON, err := json.Marshal(legacy.Description)
		if err != nil {
			return fmt.Errorf("postgrescore.backfill.recruitment.jobs.description[%s]: %w", legacy.Slug, err)
		}
		reqJSON, err := json.Marshal(legacy.Requirements)
		if err != nil {
			return fmt.Errorf("postgrescore.backfill.recruitment.jobs.requirements[%s]: %w", legacy.Slug, err)
		}
		benefitsJSON, err := json.Marshal(legacy.Benefits)
		if err != nil {
			return fmt.Errorf("postgrescore.backfill.recruitment.jobs.benefits[%s]: %w", legacy.Slug, err)
		}

		record := &RecruitmentJob{
			ID:               legacy.ID.Hex(),
			TenantID:         defaultString(strings.TrimSpace(legacy.TenantID), "default"),
			Slug:             legacy.Slug,
			Title:            legacy.Title,
			Status:           legacy.Status,
			IsHot:            legacy.IsHot,
			IsNew:            legacy.IsNew,
			IsUrgent:         legacy.IsUrgent,
			Salary:           legacy.Salary,
			SalaryMin:        legacy.SalaryMin,
			SalaryMax:        legacy.SalaryMax,
			SalaryCurrency:   legacy.SalaryCurrency,
			Quantity:         legacy.Quantity,
			ViewCount:        legacy.ViewCount,
			WorkType:         legacy.WorkType,
			WorkSchedule:     legacy.WorkSchedule,
			PostDate:         legacy.PostDate,
			Deadline:         legacy.Deadline,
			DeadlineDate:     legacy.DeadlineDate,
			Location:         legacy.Location,
			StreetAddr:       legacy.StreetAddr,
			City:             legacy.City,
			Country:          legacy.Country,
			EmploymentType:   legacy.EmpType,
			Summary:          legacy.Summary,
			DescriptionJSON:  string(descJSON),
			RequirementsJSON: string(reqJSON),
			BenefitsJSON:     string(benefitsJSON),
			IsActive:         legacy.IsActive,
			CreatedBy:        legacy.CreatedBy,
			CreatedAt:        zeroToRecruitment(legacy.CreatedAt, time.Now().UTC()),
			UpdatedAt:        zeroToRecruitment(legacy.UpdatedAt, zeroToRecruitment(legacy.CreatedAt, time.Now().UTC())),
		}

		var existing RecruitmentJob
		err = db.WithContext(ctx).Where("id = ?", record.ID).First(&existing).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return fmt.Errorf("postgrescore.backfill.recruitment.jobs.findExisting[%s]: %w", record.ID, err)
		}

		if err == nil {
			if err := db.WithContext(ctx).Model(&RecruitmentJob{}).Where("id = ?", record.ID).Updates(record).Error; err != nil {
				return fmt.Errorf("postgrescore.backfill.recruitment.jobs.update[%s]: %w", record.ID, err)
			}
			report.JobsUpdated++
			continue
		}

		if err := db.WithContext(ctx).Create(record).Error; err != nil {
			return fmt.Errorf("postgrescore.backfill.recruitment.jobs.create[%s]: %w", record.ID, err)
		}
		report.JobsCreated++
	}
	return cursor.Err()
}

func backfillRecruitmentCandidates(
	ctx context.Context,
	db *gorm.DB,
	mongoClient *database.MongoClient,
	report *BackfillRecruitmentReport,
) error {
	cursor, err := mongoClient.Collection("candidates").Find(
		ctx,
		bson.M{},
		options.Find().SetSort(bson.D{{Key: "updated_at", Value: 1}, {Key: "_id", Value: 1}}),
	)
	if err != nil {
		return fmt.Errorf("postgrescore.backfill.recruitment.candidates.find: %w", err)
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var legacy legacyRecruitmentCandidate
		if err := cursor.Decode(&legacy); err != nil {
			return fmt.Errorf("postgrescore.backfill.recruitment.candidates.decode: %w", err)
		}
		report.CandidatesSeen++
		if legacy.ID.IsZero() || strings.TrimSpace(legacy.TrackingCode) == "" {
			continue
		}

		record := &RecruitmentCandidate{
			ID:           legacy.ID.Hex(),
			JobID:        legacy.JobID,
			JobTitle:     legacy.JobTitle,
			TenantID:     defaultString(strings.TrimSpace(legacy.TenantID), "default"),
			FullName:     legacy.FullName,
			Email:        strings.TrimSpace(strings.ToLower(legacy.Email)),
			Phone:        legacy.Phone,
			CVURL:        legacy.CVURL,
			CoverLetter:  legacy.CoverLetter,
			Note:         legacy.Note,
			PublicNote:   legacy.PublicNote,
			ApplyType:    legacy.ApplyType,
			Status:       legacy.Status,
			TrackingCode: legacy.TrackingCode,
			CreatedAt:    zeroToRecruitment(legacy.CreatedAt, time.Now().UTC()),
			UpdatedAt:    zeroToRecruitment(legacy.UpdatedAt, zeroToRecruitment(legacy.CreatedAt, time.Now().UTC())),
		}

		var existing RecruitmentCandidate
		err = db.WithContext(ctx).Where("id = ?", record.ID).First(&existing).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return fmt.Errorf("postgrescore.backfill.recruitment.candidates.findExisting[%s]: %w", record.ID, err)
		}

		if err == nil {
			if err := db.WithContext(ctx).Model(&RecruitmentCandidate{}).Where("id = ?", record.ID).Updates(record).Error; err != nil {
				return fmt.Errorf("postgrescore.backfill.recruitment.candidates.update[%s]: %w", record.ID, err)
			}
			report.CandidatesUpdated++
			continue
		}

		if err := db.WithContext(ctx).Create(record).Error; err != nil {
			return fmt.Errorf("postgrescore.backfill.recruitment.candidates.create[%s]: %w", record.ID, err)
		}
		report.CandidatesCreated++
	}
	return cursor.Err()
}

func zeroToRecruitment(value, fallback time.Time) time.Time {
	if value.IsZero() {
		return fallback
	}
	return value.UTC()
}
