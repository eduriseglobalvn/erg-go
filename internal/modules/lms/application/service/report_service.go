package service

import (
	"context"
	"encoding/csv"
	"fmt"
	"net/url"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func (s *Service) ClassroomReport(ctx context.Context, tenantID string, actor Actor, centerID, classID, subjectID string) (ClassroomReportResponseDTO, error) {
	if centerID != "" && !actor.canAccessGlobal() && !s.canAccessCenter(ctx, tenantID, actor, centerID) {
		return ClassroomReportResponseDTO{}, errScopeForbidden
	}
	classReq := ClassListRequestDTO{CenterID: centerID, Page: 1, Limit: 100}
	if classID != "" {
		classReq.Keyword = ""
	}
	classList, err := s.ListClasses(ctx, tenantID, actor, classReq)
	if err != nil {
		return ClassroomReportResponseDTO{}, err
	}
	assignments := []Assignment{}
	for _, classItem := range classList.Items {
		if classID != "" && classItem.ID != classID {
			continue
		}
		filter := bson.M{}
		oid, _ := objectID(classItem.ID)
		filter["class_id"] = oid
		if subjectID != "" {
			subjectOID, err := objectID(subjectID)
			if err != nil {
				return ClassroomReportResponseDTO{}, err
			}
			filter["subject_id"] = subjectOID
		}
		items, err := s.repo.ListAssignments(ctx, tenantID, filter)
		if err != nil {
			return ClassroomReportResponseDTO{}, err
		}
		assignments = append(assignments, items...)
	}
	summary := map[string]any{
		"classCount":      len(classList.Items),
		"assignmentCount": len(assignments),
	}
	return ClassroomReportResponseDTO{Summary: summary, Classes: classList.Items, Assignments: assignmentsToDTO(assignments), StudentsNeedSupport: []StudentListItemDTO{}}, nil
}

func (s *Service) StudentJourney(ctx context.Context, tenantID string, actor Actor, studentID string) (StudentJourneyResponseDTO, error) {
	student, err := s.repo.GetStudent(ctx, tenantID, studentID)
	if err != nil {
		return StudentJourneyResponseDTO{}, err
	}
	if student == nil {
		return StudentJourneyResponseDTO{}, errNotFound
	}
	if !s.canAccessStudent(ctx, tenantID, actor, *student) {
		return StudentJourneyResponseDTO{}, errScopeForbidden
	}
	attempts, err := s.repo.ListAttempts(ctx, tenantID, bson.M{"student_id": student.ID})
	if err != nil {
		return StudentJourneyResponseDTO{}, err
	}
	assignments := map[string]Assignment{}
	for _, attempt := range attempts {
		assignmentID := attempt.AssignmentID.Hex()
		if _, ok := assignments[assignmentID]; ok {
			continue
		}
		assignment, err := s.repo.GetAssignment(ctx, tenantID, assignmentID)
		if err != nil {
			return StudentJourneyResponseDTO{}, err
		}
		if assignment != nil {
			assignments[assignmentID] = *assignment
		}
	}
	return buildStudentJourney(*student, attempts, assignments), nil
}

func (s *Service) AssignmentReport(ctx context.Context, tenantID, assignmentID string) (AssignmentReportResponseDTO, error) {
	progress, err := s.AssignmentProgress(ctx, tenantID, assignmentID)
	if err != nil {
		return AssignmentReportResponseDTO{}, err
	}
	assignment, err := s.repo.GetAssignment(ctx, tenantID, assignmentID)
	if err != nil {
		return AssignmentReportResponseDTO{}, err
	}
	if assignment == nil {
		return AssignmentReportResponseDTO{}, errNotFound
	}
	attempts, err := s.repo.ListAttempts(ctx, tenantID, bson.M{"assignment_id": assignment.ID, "status": "submitted"})
	if err != nil {
		return AssignmentReportResponseDTO{}, err
	}
	dist := map[string]int{"0-49": 0, "50-79": 0, "80-100": 0}
	late := 0
	for _, attempt := range attempts {
		switch {
		case attempt.Percent < 50:
			dist["0-49"]++
		case attempt.Percent < 80:
			dist["50-79"]++
		default:
			dist["80-100"]++
		}
		if assignment.DueAt != nil && attempt.SubmittedAt != nil && attempt.SubmittedAt.After(*assignment.DueAt) {
			late++
		}
	}
	return AssignmentReportResponseDTO{Completion: progress, ScoreDistribution: dist, LateSubmissions: late, NeedsReview: progress.NeedsReview}, nil
}

func (s *Service) ExportReport(ctx context.Context, tenantID string, actor Actor, reportType, centerID, classID string) (ReportExportResponseDTO, error) {
	if normalizedReportType(reportType) == "roster" {
		return s.exportStudentRoster(ctx, tenantID, actor, centerID, classID)
	}
	report, err := s.ClassroomReport(ctx, tenantID, actor, centerID, classID, "")
	if err != nil {
		return ReportExportResponseDTO{}, err
	}
	var b strings.Builder
	w := csv.NewWriter(&b)
	_ = w.Write([]string{"type", "id", "title"})
	for _, a := range report.Assignments {
		_ = w.Write([]string{"assignment", a.ID, a.QuizID})
	}
	w.Flush()
	return ReportExportResponseDTO{DownloadURL: "data:text/csv;charset=utf-8," + url.QueryEscape(b.String())}, nil
}

func (s *Service) exportStudentRoster(ctx context.Context, tenantID string, actor Actor, centerID, classID string) (ReportExportResponseDTO, error) {
	var b strings.Builder
	w := csv.NewWriter(&b)
	_ = w.Write([]string{
		"studentId", "studentCode", "fullName", "username", "email", "gender", "birthday", "phone", "address",
		"parentName", "parentPhone", "parentEmail", "parentRelationship", "enrollmentDate",
		"schoolOrCenterId", "schoolOrCenterCode", "schoolOrCenterName", "schoolOrCenterType",
		"classId", "className", "grade", "academicYear", "status", "note",
	})
	cursor := ""
	for {
		page, err := s.ListStudents(ctx, tenantID, actor, StudentListRequestDTO{CenterID: centerID, ClassID: classID, Cursor: cursor, Limit: maxStudentBatchSize})
		if err != nil {
			return ReportExportResponseDTO{}, err
		}
		for _, student := range page.Items {
			_ = w.Write([]string{
				student.ID,
				student.StudentCode,
				student.FullName,
				student.Username,
				student.Email,
				student.Gender,
				formatDate(student.Birthday),
				student.Phone,
				student.Address,
				student.ParentName,
				student.ParentPhone,
				student.ParentEmail,
				student.ParentRelationship,
				formatDate(student.EnrollmentDate),
				student.CenterID,
				student.CenterCode,
				student.CenterName,
				student.CenterType,
				student.ClassID,
				student.ClassName,
				student.Grade,
				student.AcademicYear,
				student.Status,
				student.Note,
			})
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	w.Flush()
	return ReportExportResponseDTO{DownloadURL: "data:text/csv;charset=utf-8," + url.QueryEscape(b.String())}, nil
}

func normalizedReportType(reportType string) string {
	switch strings.ToLower(strings.TrimSpace(reportType)) {
	case "roster", "student_roster", "students", "student_profiles":
		return "roster"
	default:
		return strings.ToLower(strings.TrimSpace(reportType))
	}
}

func formatDate(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format("2006-01-02")
}

func (s *Service) ListInternalDocuments(ctx context.Context, tenantID string, actor Actor, typ, keyword, subjectID string) (InternalDocumentListResponseDTO, error) {
	if !actor.canAccessGlobal() {
		return InternalDocumentListResponseDTO{}, errScopeForbidden
	}
	filter := bson.M{}
	if typ != "" {
		filter["type"] = typ
	}
	if subjectID != "" {
		filter["subject_id"] = subjectID
	}
	if keyword != "" {
		filter["title"] = bson.M{"$regex": keyword, "$options": "i"}
	}
	items, total, err := s.repo.ListInternalDocuments(ctx, tenantID, filter)
	return InternalDocumentListResponseDTO{Items: internalDocumentsToDTO(items), Total: total}, err
}

func (s *Service) CreateInternalDocument(ctx context.Context, tenantID string, actor Actor, req CreateInternalDocumentRequestDTO) (InternalDocumentResponseDTO, error) {
	if !actor.canAccessGlobal() {
		return InternalDocumentResponseDTO{}, errScopeForbidden
	}
	doc := &InternalDocument{TenantID: tenantID, Type: req.Type, Title: req.Title, SubjectID: req.SubjectID, FileID: req.FileID, Content: req.Content, AuthorID: actor.UserID}
	if err := s.repo.CreateInternalDocument(ctx, doc); err != nil {
		return InternalDocumentResponseDTO{}, err
	}
	return internalDocumentToDTO(*doc), nil
}

func internalDocumentsToDTO(items []InternalDocument) []InternalDocumentResponseDTO {
	out := make([]InternalDocumentResponseDTO, 0, len(items))
	for _, item := range items {
		out = append(out, internalDocumentToDTO(item))
	}
	return out
}

func internalDocumentToDTO(d InternalDocument) InternalDocumentResponseDTO {
	return InternalDocumentResponseDTO{ID: d.ID.Hex(), Type: d.Type, Title: d.Title, SubjectID: d.SubjectID, FileID: d.FileID, Content: d.Content, AuthorID: d.AuthorID, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt}
}

func ensureReportUse(_ fmt.Stringer) {}
