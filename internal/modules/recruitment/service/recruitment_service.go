// Package service provides business logic for the recruitment module.
package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"erg.ninja/internal/modules/recruitment/dto"
	"erg.ninja/internal/modules/recruitment/entities"
	"erg.ninja/internal/modules/recruitment/repository"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/storage"
)

const (
	// Auto-flag thresholds (mirrors erg-backend processWithFlags).
	autoFlagNewDays    = 7  // isNew if created within 7 days
	autoFlagUrgentDays = 5  // isUrgent if deadline within 5 days
	autoFlagHotViews   = 20 // isHot if viewCount > 20
)

// Service provides recruitment business logic.
type Service struct {
	repo *repository.Repository
	r2   *storage.R2Client
	log  *logger.Logger
}

// ServiceOption configures the Service.
type ServiceOption func(*Service)

// WithRecruitmentLogger sets the logger.
func WithRecruitmentLogger(log *logger.Logger) ServiceOption {
	return func(s *Service) { s.log = log }
}

// WithR2 sets the R2 storage client.
func WithR2(r2 *storage.R2Client) ServiceOption {
	return func(s *Service) { s.r2 = r2 }
}

// NewService creates a new recruitment service.
func NewService(repo *repository.Repository, log *logger.Logger, opts ...ServiceOption) *Service {
	s := &Service{repo: repo, log: log}
	for _, o := range opts {
		o(s)
	}
	return s
}

// ─── Job Management ────────────────────────────────────────────────────────────

// CreateJob creates a new job posting.
func (s *Service) CreateJob(ctx context.Context, req *dto.CreateJobRequest) (*dto.JobItemResponse, error) {
	if req.SalaryCurrency == "" {
		req.SalaryCurrency = "VND"
	}
	if req.Country == "" {
		req.Country = "VN"
	}

	job := &entities.Job{
		TenantID:       req.TenantID,
		Slug:           req.Slug,
		Title:          req.Title,
		Status:         req.Status,
		IsHot:          req.IsHot,
		IsNew:          req.IsNew,
		IsUrgent:       req.IsUrgent,
		Salary:         req.Salary,
		SalaryMin:      req.SalaryMin,
		SalaryMax:      req.SalaryMax,
		SalaryCurrency: req.SalaryCurrency,
		Quantity:       req.Quantity,
		WorkType:       req.WorkType,
		WorkSchedule:   req.WorkSchedule,
		PostDate:       req.PostDate,
		Deadline:       req.Deadline,
		Location:       req.Location,
		StreetAddr:     req.StreetAddr,
		City:           req.City,
		Country:        req.Country,
		EmpType:        req.EmpType,
		Summary:        req.Summary,
		Description:    req.Description,
		Requirements:   req.Requirements,
		Benefits:       req.Benefits,
		IsActive:       req.IsActive,
		CreatedBy:      req.CreatedBy,
	}
	if job.Status == "" {
		job.Status = entities.JobStatusNormal
	}

	if err := s.repo.CreateJob(ctx, job); err != nil {
		return nil, fmt.Errorf("recruitment.CreateJob: %w", err)
	}

	s.log.InfoContext(ctx).Str("id", job.ID.Hex()).Str("slug", job.Slug).Msg("recruitment: job created")
	return dto.ToJobItemResponse(job), nil
}

// GetJobBySlug returns an active job by slug and increments its view count.
func (s *Service) GetJobBySlug(ctx context.Context, slug string) (*dto.JobDetailResponse, error) {
	job, err := s.repo.GetJobBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	// Increment view count (fire-and-forget).
	_ = s.repo.IncrementViewCount(ctx, job.ID.Hex())

	// Re-compute auto-flags before returning.
	job = computeAutoFlags(job)

	schema := buildJobSchema(job)

	return &dto.JobDetailResponse{
		Job:    dto.ToJobItemResponse(job),
		Schema: schema,
	}, nil
}

// ListJobs returns paginated active jobs with filters.
func (s *Service) ListJobs(ctx context.Context, params dto.JobQueryParams) (*dto.JobListResponse, error) {
	if params.Limit <= 0 {
		params.Limit = 10
	}
	if params.Limit > 100 {
		params.Limit = 100
	}
	if params.Page < 1 {
		params.Page = 1
	}

	jobs, total, err := s.repo.ListJobs(ctx, repository.ListJobsParams{
		Search:   params.Search,
		Salary:   params.Salary,
		WorkType: params.WorkType,
		Location: params.Location,
		Sort:     params.Sort,
		Page:     params.Page,
		Limit:    params.Limit,
	})
	if err != nil {
		return nil, fmt.Errorf("recruitment.ListJobs: %w", err)
	}

	// Apply auto-flags to each job.
	for _, job := range jobs {
		job = computeAutoFlags(job)
	}

	items := make([]*dto.JobItemResponse, len(jobs))
	for i, job := range jobs {
		items[i] = dto.ToJobItemResponse(job)
	}

	totalPages := int(total) / params.Limit
	if int(total)%params.Limit != 0 {
		totalPages++
	}

	return &dto.JobListResponse{
		Items: items,
		Meta: &dto.ListMeta{
			Total:      int(total),
			Page:       params.Page,
			Limit:      params.Limit,
			TotalPages: totalPages,
		},
	}, nil
}

// UpdateJob updates a job's fields.
func (s *Service) UpdateJob(ctx context.Context, id string, req *dto.UpdateJobRequest) (*dto.JobItemResponse, error) {
	updates := map[string]any{}

	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Slug != nil {
		updates["slug"] = *req.Slug
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if req.IsHot != nil {
		updates["is_hot"] = *req.IsHot
	}
	if req.IsNew != nil {
		updates["is_new"] = *req.IsNew
	}
	if req.IsUrgent != nil {
		updates["is_urgent"] = *req.IsUrgent
	}
	if req.Salary != nil {
		updates["salary"] = *req.Salary
	}
	if req.SalaryMin != nil {
		updates["salary_min"] = *req.SalaryMin
	}
	if req.SalaryMax != nil {
		updates["salary_max"] = *req.SalaryMax
	}
	if req.SalaryCurrency != nil {
		updates["salary_currency"] = *req.SalaryCurrency
	}
	if req.Quantity != nil {
		updates["quantity"] = *req.Quantity
	}
	if req.WorkType != nil {
		updates["work_type"] = *req.WorkType
	}
	if req.WorkSchedule != nil {
		updates["work_schedule"] = *req.WorkSchedule
	}
	if req.PostDate != nil {
		updates["post_date"] = *req.PostDate
	}
	if req.Deadline != nil {
		updates["deadline"] = *req.Deadline
	}
	if req.Location != nil {
		updates["location"] = *req.Location
	}
	if req.StreetAddr != nil {
		updates["street_address"] = *req.StreetAddr
	}
	if req.City != nil {
		updates["city"] = *req.City
	}
	if req.Country != nil {
		updates["country"] = *req.Country
	}
	if req.EmpType != nil {
		updates["employment_type"] = *req.EmpType
	}
	if req.Summary != nil {
		updates["summary"] = *req.Summary
	}
	if len(req.Description) > 0 {
		updates["description"] = req.Description
	}
	if len(req.Requirements) > 0 {
		updates["requirements"] = req.Requirements
	}
	if len(req.Benefits) > 0 {
		updates["benefits"] = req.Benefits
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	if len(updates) == 0 {
		return nil, fmt.Errorf("no fields to update")
	}

	if err := s.repo.UpdateJobFields(ctx, id, updates); err != nil {
		return nil, fmt.Errorf("recruitment.UpdateJob: %w", err)
	}

	job, err := s.repo.GetJobByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("recruitment.UpdateJob fetch: %w", err)
	}

	s.log.InfoContext(ctx).Str("id", id).Msg("recruitment: job updated")
	return dto.ToJobItemResponse(job), nil
}

// DeleteJob soft-deletes a job.
func (s *Service) DeleteJob(ctx context.Context, id string) error {
	if err := s.repo.SoftDeleteJob(ctx, id); err != nil {
		return fmt.Errorf("recruitment.DeleteJob: %w", err)
	}
	s.log.InfoContext(ctx).Str("id", id).Msg("recruitment: job deleted (soft)")
	return nil
}

// ToggleJobFlag toggles a boolean flag on a job.
func (s *Service) ToggleJobFlag(ctx context.Context, id, flag string) (*dto.JobItemResponse, error) {
	job, err := s.repo.GetJobByID(ctx, id)
	if err != nil {
		return nil, err
	}

	updates := map[string]any{}
	switch flag {
	case "isHot":
		job.IsHot = !job.IsHot
		updates["is_hot"] = job.IsHot
	case "isUrgent":
		job.IsUrgent = !job.IsUrgent
		updates["is_urgent"] = job.IsUrgent
	case "isNew":
		job.IsNew = !job.IsNew
		updates["is_new"] = job.IsNew
	default:
		return nil, fmt.Errorf("recruitment: unknown flag %q", flag)
	}

	if err := s.repo.UpdateJobFields(ctx, id, updates); err != nil {
		return nil, fmt.Errorf("recruitment.ToggleJobFlag: %w", err)
	}

	job, err = s.repo.GetJobByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return dto.ToJobItemResponse(job), nil
}

// UpdateJobStatus sets the isActive field.
func (s *Service) UpdateJobStatus(ctx context.Context, id string, isActive bool) (*dto.JobItemResponse, error) {
	if err := s.repo.UpdateJobFields(ctx, id, map[string]any{"is_active": isActive}); err != nil {
		return nil, fmt.Errorf("recruitment.UpdateJobStatus: %w", err)
	}

	job, err := s.repo.GetJobByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return dto.ToJobItemResponse(job), nil
}

// ─── Candidate / Apply ────────────────────────────────────────────────────────

// CV MIME types accepted (mirrors erg-backend).
var allowedCVMIMEs = map[string]struct{}{
	"application/pdf":    {},
	"application/msword": {},
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": {},
}

const maxCVSize = 2 << 20 // 2 MB

// ApplyResult holds the result of an apply operation.
type ApplyResult struct {
	TrackingCode string
	CVURL        string
}

// Apply handles a job application: CV upload to R2 → DB record → email confirmation.
func (s *Service) Apply(ctx context.Context, tenantID string, req *dto.ApplyRequest, cvBuf []byte, cvFilename, cvMime string) (*dto.ApplyResponse, error) {
	// ── 1. Validate CV ──────────────────────────────────────────────────────
	if len(cvBuf) == 0 {
		return nil, fmt.Errorf("CV file is required")
	}
	if int64(len(cvBuf)) > maxCVSize {
		return nil, fmt.Errorf("CV file exceeds 2MB limit")
	}
	if _, ok := allowedCVMIMEs[cvMime]; !ok {
		return nil, fmt.Errorf("only PDF or Word files are accepted (.pdf, .doc, .docx)")
	}

	// ── 2. Resolve job title if jobId provided ───────────────────────────────
	jobTitle := "Ứng viên chung"
	if req.JobID != "" {
		job, err := s.repo.GetJobByID(ctx, req.JobID)
		if err == nil && job != nil {
			jobTitle = job.Title
		}
	}

	// ── 3. Upload CV to R2 ──────────────────────────────────────────────────
	cvURL, err := s.uploadCV(ctx, req.FullName, cvBuf, cvFilename, cvMime)
	if err != nil {
		return nil, fmt.Errorf("recruitment.Apply upload: %w", err)
	}

	// ── 4. Create candidate record ───────────────────────────────────────────
	candidate := &entities.Candidate{
		JobID:        req.JobID,
		JobTitle:     jobTitle,
		TenantID:     tenantID,
		FullName:     req.FullName,
		Email:        req.Email,
		Phone:        req.Phone,
		CVURL:        cvURL,
		CoverLetter:  req.CoverLetter,
		ApplyType:    entities.ApplyTypeOnline,
		Status:       entities.CandidateStatusPending,
		TrackingCode: uuid.New().String(),
	}

	if err := s.repo.CreateCandidate(ctx, candidate); err != nil {
		// Rollback: delete uploaded CV.
		if cvURL != "" && s.r2 != nil {
			_ = s.r2.Delete(ctx, cvURL)
		}
		return nil, fmt.Errorf("recruitment.Apply create: %w", err)
	}

	// ── 5. Send confirmation email (async, non-blocking) ───────────────────
	s.sendApplicationEmail(context.Background(), candidate, jobTitle, req.TrackingURL)

	s.log.InfoContext(ctx).
		Str("candidate_id", candidate.ID.Hex()).
		Str("tracking_code", candidate.TrackingCode).
		Str("full_name", candidate.FullName).
		Str("job", jobTitle).
		Msg("recruitment: application received")

	return &dto.ApplyResponse{
		TrackingCode: candidate.TrackingCode,
		Message:      "Hồ sơ của bạn đã được gửi thành công. Vui lòng kiểm tra email để theo dõi.",
	}, nil
}

// uploadCV renames the CV file by applicant name and uploads to R2 under cv/.
func (s *Service) uploadCV(ctx context.Context, fullName string, buf []byte, originalFilename, mime string) (string, error) {
	if s.r2 == nil {
		return "", fmt.Errorf("storage not configured")
	}

	ext := strings.TrimPrefix(strings.ToLower(getExt(originalFilename)), ".")
	if ext == "" {
		ext = "pdf"
	}
	safeName := slugifyName(fullName)
	filename := fmt.Sprintf("%s-%d.%s", safeName, time.Now().UnixMilli(), ext)

	return s.r2.UploadRaw(ctx, buf, "cv", filename, mime)
}

// slugifyName converts a person's name to a URL-safe filename component.
func slugifyName(name string) string {
	name = strings.ReplaceAll(name, "đ", "d")
	name = strings.ReplaceAll(name, "Đ", "D")
	name = strings.NewReplacer(
		"À", "A", "Á", "A", "Ạ", "A",
		"Ằ", "A", "Ắ", "A", "Ặ", "A",
		"Ầ", "A", "Ấ", "A", "Ậ", "A",
		"È", "E", "É", "E", "Ẹ", "E",
		"Ề", "E", "Ế", "E", "Ệ", "E",
		"Ì", "I", "Í", "I", "Ị", "I",
		"Ò", "O", "Ó", "O", "Ọ", "O",
		"Ờ", "O", "Ớ", "O", "Ợ", "O",
		"Ủ", "U", "Ú", "U", "Ụ", "U",
		"Ỳ", "Y", "Ý", "Y", "Ỵ", "Y",
		" ", "-",
	).Replace(name)

	// Remove remaining non-alphanumeric chars.
	var clean strings.Builder
	for _, ch := range name {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' {
			clean.WriteRune(ch)
		}
	}
	return strings.ToLower(strings.Trim(clean.String(), "-"))
}

func getExt(filename string) string {
	if i := strings.LastIndexByte(filename, '.'); i >= 0 {
		return filename[i:]
	}
	return ""
}

// TrackApplication returns a public tracking view of an application.
func (s *Service) TrackApplication(ctx context.Context, code string) (*dto.TrackingResponse, error) {
	c, err := s.repo.GetCandidateByTrackingCode(ctx, code)
	if err != nil {
		return nil, err
	}

	return &dto.TrackingResponse{
		FullName:    c.FullName,
		JobTitle:    c.JobTitle,
		ApplyType:   c.ApplyType,
		Status:      c.Status,
		PublicNote:  c.PublicNote,
		SubmittedAt: c.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}

// ListCandidates returns all candidates (admin).
func (s *Service) ListCandidates(ctx context.Context, jobID string) (*dto.CandidateListResponse, error) {
	candidates, err := s.repo.ListCandidates(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("recruitment.ListCandidates: %w", err)
	}

	items := make([]*dto.CandidateItemResponse, len(candidates))
	for i, c := range candidates {
		items[i] = dto.ToCandidateItemResponse(c)
	}

	return &dto.CandidateListResponse{
		Items: items,
		Meta: &dto.ListMeta{
			Total:      len(items),
			Page:       1,
			Limit:      len(items),
			TotalPages: 1,
		},
	}, nil
}

// UpdateCandidateStatus updates candidate status and public note.
func (s *Service) UpdateCandidateStatus(ctx context.Context, id string, req *dto.UpdateCandidateStatusRequest) (*dto.CandidateItemResponse, error) {
	updates := map[string]any{"status": req.Status}
	if req.PublicNote != "" {
		updates["public_note"] = req.PublicNote
	}

	if err := s.repo.UpdateCandidateFields(ctx, id, updates); err != nil {
		return nil, fmt.Errorf("recruitment.UpdateCandidateStatus: %w", err)
	}

	c, err := s.repo.GetCandidateByID(ctx, id)
	if err != nil {
		return nil, err
	}

	s.log.InfoContext(ctx).Str("id", id).Str("status", req.Status).Msg("recruitment: candidate status updated")
	return dto.ToCandidateItemResponse(c), nil
}

// ─── Auto-flags (mirrors erg-backend processWithFlags) ───────────────────────

// computeAutoFlags recomputes isNew, isUrgent, isHot from timestamps and view count.
func computeAutoFlags(job *entities.Job) *entities.Job {
	now := time.Now()
	createdDate := job.CreatedAt

	// isNew: created within 7 days.
	if !job.IsNew && createdDate.After(now.AddDate(0, 0, -autoFlagNewDays)) {
		job.IsNew = true
	}

	// isUrgent: deadline within 5 days.
	if !job.IsUrgent && job.Deadline != "" {
		if deadline, err := parseDeadline(job.Deadline); err == nil {
			daysTo := int(deadline.Sub(now).Hours() / 24)
			if daysTo > 0 && daysTo <= autoFlagUrgentDays {
				job.IsUrgent = true
			}
		}
	}

	// isHot: viewCount > 20 AND isActive AND (deadline not passed OR no deadline).
	if !job.IsHot && job.ViewCount > autoFlagHotViews {
		if job.Deadline == "" {
			job.IsHot = true
		} else if deadline, err := parseDeadline(job.Deadline); err == nil && deadline.After(now) {
			job.IsHot = true
		}
	}

	return job
}

func parseDeadline(s string) (time.Time, error) {
	// Try multiple common formats.
	formats := []string{
		time.RFC3339,
		"2006-01-02",
		"2006/01/02",
		"02/01/2006",
		"01/02/2006",
	}
	s = strings.TrimSpace(s)
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("parse deadline: unknown format %q", s)
}

// ─── Schema.org JSON-LD ──────────────────────────────────────────────────────

// buildJobSchema generates a schema.org/JobPosting JSON-LD object.
func buildJobSchema(job *entities.Job) *dto.SchemaJobPosting {
	hiringOrg := map[string]string{
		"@type":  "Organization",
		"name":   "Trung tâm Tin học ERG",
		"sameAs": "https://erg.edu.vn",
	}

	jobLocation := map[string]string{
		"@type": "Place",
		"name":  job.Location,
	}
	if job.City != "" {
		jobLocation["address"] = job.City
		if job.StreetAddr != "" {
			jobLocation["address"] = job.StreetAddr + ", " + job.City
		}
	}

	var baseSalary *map[string]interface{}
	if job.SalaryMin != nil || job.SalaryMax != nil {
		bs := map[string]interface{}{
			"@type":    "MonetaryAmount",
			"currency": job.SalaryCurrency,
		}
		if job.SalaryMin != nil {
			bs["value"] = map[string]interface{}{
				"@type":    "QuantitativeValue",
				"minValue": *job.SalaryMin,
				"maxValue": *job.SalaryMax,
				"unitText": "MONTH",
			}
		}
		baseSalary = &bs
	}

	s := &dto.SchemaJobPosting{
		Context:            "https://schema.org",
		Type:               "JobPosting",
		Title:              job.Title,
		Description:        strings.Join(job.Description, "\n"),
		DatePosted:         job.CreatedAt.Format("2006-01-02"),
		HiringOrganization: hiringOrg["name"],
		JobLocation:        jobLocation,
		EmploymentType:     mapEmpType(job.EmpType),
	}
	if job.Deadline != "" {
		s.ValidThrough = job.Deadline
	}
	if baseSalary != nil {
		s.BaseSalary = baseSalary
	}
	return s
}

func mapEmpType(emp string) string {
	switch emp {
	case "FULL_TIME":
		return "FULL_TIME"
	case "PART_TIME":
		return "PART_TIME"
	case "CONTRACT":
		return "CONTRACT"
	case "INTERN":
		return "INTERN"
	default:
		return "FULL_TIME"
	}
}

// ─── Email confirmation ───────────────────────────────────────────────────────

// sendApplicationEmail sends a confirmation email to the applicant.
// Errors are logged but not propagated (fire-and-forget).
func (s *Service) sendApplicationEmail(ctx context.Context, c *entities.Candidate, jobTitle, trackingURL string) {
	if c.Email == "" {
		return
	}

	baseURL := trackingURL
	if baseURL == "" {
		baseURL = "https://erg.edu.vn/tuyen-dung/track"
	}
	trackingLink := baseURL + "/" + c.TrackingCode

	body := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: Arial, sans-serif; max-width: 600px; margin: auto; border: 1px solid #e0e0e0; border-radius: 8px; overflow: hidden;">
  <div style="background: #003087; padding: 20px; text-align: center;">
    <h2 style="color: white; margin: 0;">Trung tâm Tin học ERG</h2>
    <p style="color: #90caf9; margin: 4px 0 0;">Xác nhận ứng tuyển thành công</p>
  </div>
  <div style="padding: 24px;">
    <p>Chào <strong>%s</strong>,</p>
    <p>Cảm ơn bạn đã quan tâm đến cơ hội nghề nghiệp tại <strong>Trung tâm Tin học ERG</strong>.</p>
    <p>Chúng tôi xác nhận đã nhận được hồ sơ của bạn cho vị trí:</p>
    <div style="background: #f5f5f5; border-left: 4px solid #003087; padding: 12px 16px; margin: 12px 0; border-radius: 4px;">
      <strong style="font-size: 16px;">📋 %s</strong>
    </div>
    <p>Theo dõi trạng thái hồ sơ:</p>
    <div style="text-align: center; margin: 24px 0;">
      <a href="%s" style="background: #003087; color: white; padding: 12px 24px; border-radius: 6px; text-decoration: none; font-weight: bold; display: inline-block;">🔍 Xem trạng thái hồ sơ</a>
    </div>
    <p style="color: #666; font-size: 13px;">Hoặc copy link: <a href="%s">%s</a></p>
    <hr style="border: none; border-top: 1px solid #e0e0e0; margin: 20px 0;">
    <p style="color: #666; font-size: 13px;">
      📞 HR: <strong>0909 xxx xxx</strong><br>
      📧 Email: <strong>tuyendung@erg.edu.vn</strong>
    </p>
  </div>
  <div style="background: #f9f9f9; padding: 12px; text-align: center; color: #999; font-size: 12px;">
    © %d Trung tâm Tin học ERG — EDURISE GLOBAL CO., LTD
  </div>
</body>
</html>`,
		c.FullName, jobTitle,
		trackingLink, trackingLink, trackingLink,
		time.Now().Year(),
	)

	s.log.InfoContext(ctx).
		Str("to", c.Email).
		Str("tracking_code", c.TrackingCode).
		Msg("recruitment: application email queued (dev mode — SMTP not wired; log only)")

	// TODO: Wire up erg.ninja/pkg/mail or notifications.EmailProvider.
	// For now, email is logged only. Once pkg/mail is available:
	// _ = mail.Send(ctx, c.Email, "[ERG] Xác nhận ứng tuyển - "+jobTitle, body)
	_ = body // suppress unused in dev
}

// IntPtr is a helper to convert int to *int.
func IntPtr(i int) *int { return &i }

// StringPtr is a helper to convert string to *string.
func StringPtr(s string) *string { return &s }

// ParseInt parses a string to int, returning 0 on error.
func ParseInt(s string) int {
	if i, err := strconv.Atoi(s); err == nil {
		return i
	}
	return 0
}
