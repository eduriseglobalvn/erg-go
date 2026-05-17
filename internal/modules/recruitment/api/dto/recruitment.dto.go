// Package dto defines request/response types for the recruitment module.
package dto

import (
	"github.com/go-playground/validator/v10"

	entities "erg.ninja/internal/modules/recruitment/domain/entity"
)

// ─── Job DTOs ─────────────────────────────────────────────────────────────────

// CreateJobRequest mirrors CreateJobDto from erg-backend.
type CreateJobRequest struct {
	TenantID       string   `json:"tenant_id,omitempty"`
	CreatedBy      string   `json:"created_by,omitempty"`
	Title          string   `json:"title" validate:"required"`
	Slug           string   `json:"slug" validate:"required"`
	Status         string   `json:"status,omitempty"`
	IsHot          bool     `json:"is_hot,omitempty"`
	IsNew          bool     `json:"is_new,omitempty"`
	IsUrgent       bool     `json:"is_urgent,omitempty"`
	Salary         string   `json:"salary" validate:"required"`
	SalaryMin      *float64 `json:"salary_min,omitempty"`
	SalaryMax      *float64 `json:"salary_max,omitempty"`
	SalaryCurrency string   `json:"salary_currency,omitempty"`
	Quantity       int      `json:"quantity" validate:"required,min=1"`
	WorkType       string   `json:"work_type" validate:"required"`
	WorkSchedule   string   `json:"work_schedule,omitempty"`
	PostDate       string   `json:"post_date,omitempty"`
	Deadline       string   `json:"deadline" validate:"required"`
	Location       string   `json:"location" validate:"required"`
	StreetAddr     string   `json:"street_address,omitempty"`
	City           string   `json:"city,omitempty"`
	Country        string   `json:"country,omitempty"`
	EmpType        string   `json:"employment_type,omitempty"` // FULL_TIME, PART_TIME, CONTRACT, INTERN
	Summary        string   `json:"summary,omitempty"`
	Description    []string `json:"description" validate:"required"`
	Requirements   []string `json:"requirements" validate:"required"`
	Benefits       []string `json:"benefits" validate:"required"`
	IsActive       bool     `json:"is_active,omitempty"`
}

// UpdateJobRequest mirrors UpdateJobDto (all fields optional).
type UpdateJobRequest struct {
	Title          *string  `json:"title,omitempty"`
	Slug           *string  `json:"slug,omitempty"`
	Status         *string  `json:"status,omitempty"`
	IsHot          *bool    `json:"is_hot,omitempty"`
	IsNew          *bool    `json:"is_new,omitempty"`
	IsUrgent       *bool    `json:"is_urgent,omitempty"`
	Salary         *string  `json:"salary,omitempty"`
	SalaryMin      *float64 `json:"salary_min,omitempty"`
	SalaryMax      *float64 `json:"salary_max,omitempty"`
	SalaryCurrency *string  `json:"salary_currency,omitempty"`
	Quantity       *int     `json:"quantity,omitempty"`
	WorkType       *string  `json:"work_type,omitempty"`
	WorkSchedule   *string  `json:"work_schedule,omitempty"`
	PostDate       *string  `json:"post_date,omitempty"`
	Deadline       *string  `json:"deadline,omitempty"`
	Location       *string  `json:"location,omitempty"`
	StreetAddr     *string  `json:"street_address,omitempty"`
	City           *string  `json:"city,omitempty"`
	Country        *string  `json:"country,omitempty"`
	EmpType        *string  `json:"employment_type,omitempty"`
	Summary        *string  `json:"summary,omitempty"`
	Description    []string `json:"description,omitempty"`
	Requirements   []string `json:"requirements,omitempty"`
	Benefits       []string `json:"benefits,omitempty"`
	IsActive       *bool    `json:"is_active,omitempty"`
}

// JobQueryParams mirrors JobQueryDto from erg-backend.
type JobQueryParams struct {
	Page     int      `query:"page"`
	Limit    int      `query:"limit"`
	Search   string   `query:"search"`
	Salary   []string `query:"salary"`
	WorkType []string `query:"work_type"`
	Location []string `query:"location"`
	Sort     string   `query:"sort"` // newest | oldest
}

// JobListResponse mirrors NestJS findAllJobs response.
type JobListResponse struct {
	Items []*JobItemResponse `json:"items"`
	Meta  *ListMeta          `json:"meta"`
}

type ListMeta struct {
	Total      int `json:"total"`
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	TotalPages int `json:"totalPages"`
}

// JobItemResponse is a public job listing item.
type JobItemResponse struct {
	ID             string   `json:"id"`
	Slug           string   `json:"slug"`
	Title          string   `json:"title"`
	Status         string   `json:"status"`
	IsHot          bool     `json:"is_hot"`
	IsNew          bool     `json:"is_new"`
	IsUrgent       bool     `json:"is_urgent"`
	Salary         string   `json:"salary"`
	SalaryMin      *float64 `json:"salary_min,omitempty"`
	SalaryMax      *float64 `json:"salary_max,omitempty"`
	SalaryCurrency string   `json:"salary_currency"`
	Quantity       int      `json:"quantity"`
	ViewCount      int      `json:"viewCount"`
	WorkType       string   `json:"work_type"`
	WorkSchedule   string   `json:"work_schedule,omitempty"`
	PostDate       string   `json:"post_date,omitempty"`
	Deadline       string   `json:"deadline"`
	Location       string   `json:"location"`
	Summary        string   `json:"summary"`
	Description    []string `json:"description"`
	Requirements   []string `json:"requirements"`
	Benefits       []string `json:"benefits"`
	EmpType        string   `json:"employment_type"`
	City           string   `json:"city,omitempty"`
	Country        string   `json:"country"`
	IsActive       bool     `json:"is_active"`
	CreatedAt      string   `json:"createdAt"`
}

// JobDetailResponse includes job + Schema.org JSON-LD for SEO.
type JobDetailResponse struct {
	Job    *JobItemResponse  `json:"job"`
	Schema *SchemaJobPosting `json:"schema,omitempty"`
}

// SchemaJobPosting represents a schema.org/JobPosting JSON-LD object.
type SchemaJobPosting struct {
	Context            string      `json:"@context"`
	Type               string      `json:"@type"`
	Title              string      `json:"title"`
	Description        string      `json:"description"`
	DatePosted         string      `json:"datePosted"`
	ValidThrough       string      `json:"validThrough,omitempty"`
	HiringOrganization string      `json:"hiringOrganization"`
	JobLocation        interface{} `json:"jobLocation"`
	EmploymentType     string      `json:"employmentType"`
	BaseSalary         interface{} `json:"baseSalary,omitempty"`
}

// ─── Apply DTOs ────────────────────────────────────────────────────────────────

// ApplyRequest mirrors ApplyJobDto from erg-backend.
type ApplyRequest struct {
	JobID       string `json:"job_id,omitempty"`
	FullName    string `json:"fullName" validate:"required"`
	Email       string `json:"email" validate:"required,email"`
	Phone       string `json:"phone" validate:"required"`
	CoverLetter string `json:"cover_letter,omitempty"`
	TrackingURL string `json:"tracking_url,omitempty"`
}

// ApplyResponse mirrors NestJS apply response.
type ApplyResponse struct {
	TrackingCode string `json:"tracking_code"`
	Message      string `json:"message"`
}

// ─── Candidate DTOs ───────────────────────────────────────────────────────────

// CandidateListResponse mirrors NestJS findAllCandidates.
type CandidateListResponse struct {
	Items []*CandidateItemResponse `json:"items"`
	Meta  *ListMeta                `json:"meta"`
}

// CandidateItemResponse is an admin candidate item.
type CandidateItemResponse struct {
	ID           string `json:"id"`
	JobID        string `json:"job_id,omitempty"`
	JobTitle     string `json:"job_title,omitempty"`
	FullName     string `json:"fullName"`
	Email        string `json:"email"`
	Phone        string `json:"phone"`
	CVURL        string `json:"cv_url,omitempty"`
	CoverLetter  string `json:"cover_letter,omitempty"`
	Note         string `json:"note,omitempty"`
	PublicNote   string `json:"public_note,omitempty"`
	ApplyType    string `json:"apply_type"`
	Status       string `json:"status"`
	TrackingCode string `json:"tracking_code"`
	CreatedAt    string `json:"createdAt"`
}

// UpdateCandidateStatusRequest mirrors UpdateCandidateStatusDto.
type UpdateCandidateStatusRequest struct {
	Status     string `json:"status" validate:"required"`
	PublicNote string `json:"public_note,omitempty"`
}

// TrackingResponse mirrors NestJS trackApplication response.
type TrackingResponse struct {
	FullName    string `json:"fullName"`
	JobTitle    string `json:"job_title"`
	ApplyType   string `json:"apply_type"`
	Status      string `json:"status"`
	PublicNote  string `json:"public_note"`
	SubmittedAt string `json:"submitted_at"`
}

// ─── Validation ────────────────────────────────────────────────────────────────

// Validate validates a request struct using go-playground/validator.
func Validate(v interface{}) error {
	return validator.New().Struct(v)
}

// ToJobItemResponse converts a job entity to a public listing response.
func ToJobItemResponse(j *entities.Job) *JobItemResponse {
	r := &JobItemResponse{
		ID:             j.ID.Hex(),
		Slug:           j.Slug,
		Title:          j.Title,
		Status:         j.Status,
		IsHot:          j.IsHot,
		IsNew:          j.IsNew,
		IsUrgent:       j.IsUrgent,
		Salary:         j.Salary,
		SalaryMin:      j.SalaryMin,
		SalaryMax:      j.SalaryMax,
		SalaryCurrency: j.SalaryCurrency,
		Quantity:       j.Quantity,
		ViewCount:      j.ViewCount,
		WorkType:       j.WorkType,
		WorkSchedule:   j.WorkSchedule,
		PostDate:       j.PostDate,
		Deadline:       j.Deadline,
		Location:       j.Location,
		Summary:        j.Summary,
		Description:    j.Description,
		Requirements:   j.Requirements,
		Benefits:       j.Benefits,
		EmpType:        j.EmpType,
		City:           j.City,
		Country:        j.Country,
		IsActive:       j.IsActive,
		CreatedAt:      j.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	return r
}

// ToCandidateItemResponse converts a candidate entity to an admin response.
func ToCandidateItemResponse(c *entities.Candidate) *CandidateItemResponse {
	return &CandidateItemResponse{
		ID:           c.ID.Hex(),
		JobID:        c.JobID,
		JobTitle:     c.JobTitle,
		FullName:     c.FullName,
		Email:        c.Email,
		Phone:        c.Phone,
		CVURL:        c.CVURL,
		CoverLetter:  c.CoverLetter,
		Note:         c.Note,
		PublicNote:   c.PublicNote,
		ApplyType:    c.ApplyType,
		Status:       c.Status,
		TrackingCode: c.TrackingCode,
		CreatedAt:    c.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
