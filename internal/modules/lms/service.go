package lms

import (
	"context"
	crypto_rand "crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"erg.ninja/pkg/storage"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var (
	errScopeForbidden  = errors.New("SCOPE_FORBIDDEN")
	errNotFound        = errors.New("NOT_FOUND")
	errSheetAccess     = errors.New("SHEET_ACCESS_DENIED")
	errEmptyRecipient  = errors.New("ASSIGNMENT_RECIPIENT_EMPTY")
	errAttemptDone     = errors.New("ATTEMPT_ALREADY_SUBMITTED")
	errHashMismatch    = errors.New("ATTEMPT_PACKAGE_HASH_MISMATCH")
	errInvalidFieldKey = errors.New("INVALID_FIELD_KEY")
)

type Actor struct {
	UserID string
	Roles  []string
}

type Service struct {
	repo   *Repository
	sheets *storage.GoogleSheetsClient
}

func NewService(repo *Repository, sheets *storage.GoogleSheetsClient) *Service {
	return &Service{repo: repo, sheets: sheets}
}

func (s *Service) Scope(ctx context.Context, tenantID string, actor Actor) (ManagementScopeResponseDTO, error) {
	canGlobal := actor.canAccessGlobal()
	assignedCenters, _, err := s.repo.ListCenters(ctx, tenantID, CenterListRequestDTO{Page: 1, Limit: 100}, nonAdminUser(actor, canGlobal))
	if err != nil {
		return ManagementScopeResponseDTO{}, err
	}
	centerIDs := make([]bson.ObjectID, 0, len(assignedCenters))
	for _, center := range assignedCenters {
		centerIDs = append(centerIDs, center.ID)
	}
	assignedClasses, _, err := s.repo.ListClasses(ctx, tenantID, ClassListRequestDTO{Page: 1, Limit: 100}, centerIDs, nonAdminUser(actor, canGlobal))
	if err != nil {
		return ManagementScopeResponseDTO{}, err
	}
	current, err := s.currentOrDefault(ctx, tenantID, actor, canGlobal, assignedCenters, assignedClasses)
	if err != nil {
		return ManagementScopeResponseDTO{}, err
	}
	return ManagementScopeResponseDTO{
		CanAccessGlobalErg: canGlobal,
		AssignedCenters:    centersToDTO(assignedCenters),
		AssignedClasses:    classesToDTO(assignedClasses, nil),
		CurrentScope:       current,
	}, nil
}

func (s *Service) UpdateScope(ctx context.Context, tenantID string, actor Actor, req UpdateCurrentScopeRequestDTO) (CurrentScopeResponseDTO, error) {
	scope := ManagementScopeDTO{Level: req.Level, CenterID: req.CenterID, ClassID: req.ClassID}
	if err := s.validateScope(ctx, tenantID, actor, scope); err != nil {
		return CurrentScopeResponseDTO{}, err
	}
	err := s.repo.UpsertCurrentScope(ctx, &CurrentScope{
		TenantID: tenantID,
		UserID:   actor.UserID,
		Level:    req.Level,
		CenterID: req.CenterID,
		ClassID:  req.ClassID,
	})
	if err != nil {
		return CurrentScopeResponseDTO{}, err
	}
	return CurrentScopeResponseDTO{CurrentScope: scope}, nil
}

func (s *Service) ListCenters(ctx context.Context, tenantID string, actor Actor, req CenterListRequestDTO) (CenterListResponseDTO, error) {
	items, total, err := s.repo.ListCenters(ctx, tenantID, req, nonAdminUser(actor, actor.canAccessGlobal()))
	return CenterListResponseDTO{Items: centersToDTO(items), Total: total}, err
}

func (s *Service) CreateCenter(ctx context.Context, tenantID string, actor Actor, req CreateCenterRequestDTO) (CenterResponseDTO, error) {
	if !actor.canAccessGlobal() {
		return CenterResponseDTO{}, errScopeForbidden
	}
	center := &Center{TenantID: tenantID, Name: req.Name, Code: req.Code, Address: req.Address, ManagerUserID: req.ManagerUserID}
	if err := s.repo.CreateCenter(ctx, center); err != nil {
		return CenterResponseDTO{}, err
	}
	return centerToDTO(*center), nil
}

func (s *Service) UpdateCenter(ctx context.Context, tenantID string, actor Actor, id string, req UpdateCenterRequestDTO) (CenterResponseDTO, error) {
	if !actor.canAccessGlobal() {
		return CenterResponseDTO{}, errScopeForbidden
	}
	update := bson.M{}
	if req.Name != nil {
		update["name"] = *req.Name
	}
	if req.Address != nil {
		update["address"] = *req.Address
	}
	if req.Status != nil {
		update["status"] = *req.Status
	}
	if req.ManagerUserID != nil {
		update["manager_user_id"] = *req.ManagerUserID
	}
	center, err := s.repo.UpdateCenter(ctx, tenantID, id, update)
	if err != nil {
		return CenterResponseDTO{}, err
	}
	if center == nil {
		return CenterResponseDTO{}, errNotFound
	}
	return centerToDTO(*center), nil
}

func (s *Service) ListClasses(ctx context.Context, tenantID string, actor Actor, req ClassListRequestDTO) (ClassListResponseDTO, error) {
	centers, _, err := s.repo.ListCenters(ctx, tenantID, CenterListRequestDTO{Page: 1, Limit: 100}, nonAdminUser(actor, actor.canAccessGlobal()))
	if err != nil {
		return ClassListResponseDTO{}, err
	}
	centerIDs := make([]bson.ObjectID, 0, len(centers))
	centerNames := map[string]string{}
	for _, center := range centers {
		centerIDs = append(centerIDs, center.ID)
		centerNames[center.ID.Hex()] = center.Name
	}
	items, total, err := s.repo.ListClasses(ctx, tenantID, req, centerIDs, nonAdminUser(actor, actor.canAccessGlobal()))
	return ClassListResponseDTO{Items: classesToDTO(items, centerNames), Total: total}, err
}

func (s *Service) CreateClass(ctx context.Context, tenantID string, actor Actor, req CreateClassRequestDTO) (ClassResponseDTO, error) {
	if !actor.canAccessGlobal() && !s.canAccessCenter(ctx, tenantID, actor, req.CenterID) {
		return ClassResponseDTO{}, errScopeForbidden
	}
	center, err := s.repo.GetCenter(ctx, tenantID, req.CenterID)
	if err != nil {
		return ClassResponseDTO{}, err
	}
	if center == nil {
		return ClassResponseDTO{}, errNotFound
	}
	centerID, _ := objectID(req.CenterID)
	class := &Class{TenantID: tenantID, CenterID: centerID, Name: req.Name, Grade: req.Grade, HomeroomTeacherID: req.HomeroomTeacherID}
	if err := s.repo.CreateClass(ctx, class); err != nil {
		return ClassResponseDTO{}, err
	}
	return classToDTO(*class, center.Name), nil
}

func (s *Service) UpdateClass(ctx context.Context, tenantID string, actor Actor, id string, req UpdateClassRequestDTO) (ClassResponseDTO, error) {
	class, err := s.repo.GetClass(ctx, tenantID, id)
	if err != nil {
		return ClassResponseDTO{}, err
	}
	if class == nil {
		return ClassResponseDTO{}, errNotFound
	}
	if !actor.canAccessGlobal() && class.HomeroomTeacherID != actor.UserID && !s.canAccessCenter(ctx, tenantID, actor, class.CenterID.Hex()) {
		return ClassResponseDTO{}, errScopeForbidden
	}
	update := bson.M{}
	if req.Name != nil {
		update["name"] = *req.Name
	}
	if req.Grade != nil {
		update["grade"] = *req.Grade
	}
	if req.Status != nil {
		update["status"] = *req.Status
	}
	if req.HomeroomTeacherID != nil {
		update["homeroom_teacher_id"] = *req.HomeroomTeacherID
	}
	updated, err := s.repo.UpdateClass(ctx, tenantID, id, update)
	if err != nil {
		return ClassResponseDTO{}, err
	}
	return classToDTO(*updated, ""), nil
}

func (s *Service) ListStudents(ctx context.Context, tenantID string, actor Actor, req StudentListRequestDTO) (StudentListResponseDTO, error) {
	centerIDs, classIDs, err := s.allowedIDs(ctx, tenantID, actor)
	if err != nil {
		return StudentListResponseDTO{}, err
	}
	items, total, next, err := s.repo.ListStudents(ctx, tenantID, req, centerIDs, classIDs)
	if err != nil {
		return StudentListResponseDTO{}, err
	}
	return StudentListResponseDTO{Items: s.studentsToDTO(ctx, tenantID, items), Total: total, NextCursor: next}, nil
}

func (s *Service) GetStudent(ctx context.Context, tenantID string, actor Actor, id string) (StudentDetailResponseDTO, error) {
	student, err := s.repo.GetStudent(ctx, tenantID, id)
	if err != nil {
		return StudentDetailResponseDTO{}, err
	}
	if student == nil {
		return StudentDetailResponseDTO{}, errNotFound
	}
	if !s.canAccessStudent(ctx, tenantID, actor, *student) {
		return StudentDetailResponseDTO{}, errScopeForbidden
	}
	item := s.studentsToDTO(ctx, tenantID, []Student{*student})[0]
	class, _ := s.repo.GetClass(ctx, tenantID, student.ClassID.Hex())
	classes := []ClassResponseDTO{}
	if class != nil {
		classes = append(classes, classToDTO(*class, ""))
	}
	return StudentDetailResponseDTO{Profile: item, Classes: classes, Metrics: student.Metrics, Assignments: []any{}, Journey: []any{}}, nil
}

func (s *Service) CreateStudent(ctx context.Context, tenantID string, actor Actor, req CreateStudentRequestDTO) (StudentResponseDTO, error) {
	class, err := s.repo.GetClass(ctx, tenantID, req.ClassID)
	if err != nil {
		return StudentResponseDTO{}, err
	}
	if class == nil {
		return StudentResponseDTO{}, errNotFound
	}
	if !actor.canAccessGlobal() && class.HomeroomTeacherID != actor.UserID && !s.canAccessCenter(ctx, tenantID, actor, class.CenterID.Hex()) {
		return StudentResponseDTO{}, errScopeForbidden
	}
	tempPassword, err := secureTempPassword()
	if err != nil {
		return StudentResponseDTO{}, err
	}
	student := &Student{
		TenantID: tenantID,
		CenterID: class.CenterID,
		ClassID:  class.ID,
		FullName: req.FullName,
		Username: usernameFromName(req.FullName),
		Birthday: req.Birthday,
		Phone:    req.Phone,
		Note:     req.Note,
	}
	if err := s.repo.CreateStudent(ctx, student); err != nil {
		return StudentResponseDTO{}, err
	}
	return StudentResponseDTO{Student: s.studentsToDTO(ctx, tenantID, []Student{*student})[0], TempPassword: tempPassword}, nil
}

func (s *Service) UpdateStudent(ctx context.Context, tenantID string, actor Actor, id string, req UpdateStudentRequestDTO) (StudentResponseDTO, error) {
	student, err := s.repo.GetStudent(ctx, tenantID, id)
	if err != nil {
		return StudentResponseDTO{}, err
	}
	if student == nil {
		return StudentResponseDTO{}, errNotFound
	}
	if !s.canAccessStudent(ctx, tenantID, actor, *student) {
		return StudentResponseDTO{}, errScopeForbidden
	}
	update := bson.M{}
	if req.FullName != nil {
		update["full_name"] = *req.FullName
	}
	if req.Birthday != nil {
		update["birthday"] = *req.Birthday
	}
	if req.Phone != nil {
		update["phone"] = *req.Phone
	}
	if req.Note != nil {
		update["note"] = *req.Note
	}
	if req.Status != nil {
		update["status"] = *req.Status
	}
	if req.ClassID != nil {
		class, err := s.repo.GetClass(ctx, tenantID, *req.ClassID)
		if err != nil {
			return StudentResponseDTO{}, err
		}
		if class == nil {
			return StudentResponseDTO{}, errNotFound
		}
		if !actor.canAccessGlobal() && class.HomeroomTeacherID != actor.UserID && !s.canAccessCenter(ctx, tenantID, actor, class.CenterID.Hex()) {
			return StudentResponseDTO{}, errScopeForbidden
		}
		update["class_id"] = class.ID
		update["center_id"] = class.CenterID
	}
	updated, err := s.repo.UpdateStudent(ctx, tenantID, id, update)
	if err != nil {
		return StudentResponseDTO{}, err
	}
	return StudentResponseDTO{Student: s.studentsToDTO(ctx, tenantID, []Student{*updated})[0]}, nil
}

func (s *Service) BulkMoveStudents(ctx context.Context, tenantID string, actor Actor, classID string, req BulkMoveStudentsRequestDTO) (BulkActionResponseDTO, error) {
	source, err := s.repo.GetClass(ctx, tenantID, classID)
	if err != nil {
		return BulkActionResponseDTO{}, err
	}
	target, err := s.repo.GetClass(ctx, tenantID, req.TargetClassID)
	if err != nil {
		return BulkActionResponseDTO{}, err
	}
	if source == nil || target == nil {
		return BulkActionResponseDTO{}, errNotFound
	}
	if !actor.canAccessGlobal() && !s.canAccessCenter(ctx, tenantID, actor, source.CenterID.Hex()) && source.HomeroomTeacherID != actor.UserID {
		return BulkActionResponseDTO{}, errScopeForbidden
	}
	result := BulkActionResponseDTO{FailedItems: []BulkActionFailedItemDTO{}}
	for _, studentID := range req.StudentIDs {
		student, err := s.repo.GetStudent(ctx, tenantID, studentID)
		if err != nil || student == nil || student.ClassID != source.ID {
			result.FailedItems = append(result.FailedItems, BulkActionFailedItemDTO{ID: studentID, Code: "STUDENT_NOT_IN_CLASS", Message: "student not found in source class"})
			continue
		}
		if err := s.repo.SetStudentClass(ctx, tenantID, studentID, target); err != nil {
			result.FailedItems = append(result.FailedItems, BulkActionFailedItemDTO{ID: studentID, Code: "MOVE_FAILED", Message: err.Error()})
			continue
		}
		result.SuccessCount++
	}
	return result, nil
}

func (s *Service) currentOrDefault(ctx context.Context, tenantID string, actor Actor, canGlobal bool, centers []Center, classes []Class) (ManagementScopeDTO, error) {
	current, err := s.repo.GetCurrentScope(ctx, tenantID, actor.UserID)
	if err != nil {
		return ManagementScopeDTO{}, err
	}
	if current != nil {
		scope := ManagementScopeDTO{Level: current.Level, CenterID: current.CenterID, ClassID: current.ClassID}
		if s.validateScope(ctx, tenantID, actor, scope) == nil {
			return scope, nil
		}
	}
	if canGlobal {
		return ManagementScopeDTO{Level: scopeLevelGlobal}, nil
	}
	if len(centers) > 0 {
		return ManagementScopeDTO{Level: scopeLevelCenter, CenterID: centers[0].ID.Hex()}, nil
	}
	if len(classes) > 0 {
		return ManagementScopeDTO{Level: scopeLevelClass, CenterID: classes[0].CenterID.Hex(), ClassID: classes[0].ID.Hex()}, nil
	}
	return ManagementScopeDTO{Level: scopeLevelClass}, nil
}

func (s *Service) validateScope(ctx context.Context, tenantID string, actor Actor, scope ManagementScopeDTO) error {
	switch scope.Level {
	case scopeLevelGlobal:
		if actor.canAccessGlobal() {
			return nil
		}
	case scopeLevelCenter:
		if scope.CenterID != "" && s.canAccessCenter(ctx, tenantID, actor, scope.CenterID) {
			return nil
		}
	case scopeLevelClass:
		class, err := s.repo.GetClass(ctx, tenantID, scope.ClassID)
		if err == nil && class != nil && s.canAccessClass(ctx, tenantID, actor, *class) {
			return nil
		}
	}
	return errScopeForbidden
}

func (s *Service) allowedIDs(ctx context.Context, tenantID string, actor Actor) ([]bson.ObjectID, []bson.ObjectID, error) {
	if actor.canAccessGlobal() {
		return nil, nil, nil
	}
	centers, _, err := s.repo.ListCenters(ctx, tenantID, CenterListRequestDTO{Page: 1, Limit: 100}, actor.UserID)
	if err != nil {
		return nil, nil, err
	}
	centerIDs := make([]bson.ObjectID, 0, len(centers))
	for _, center := range centers {
		centerIDs = append(centerIDs, center.ID)
	}
	classes, _, err := s.repo.ListClasses(ctx, tenantID, ClassListRequestDTO{Page: 1, Limit: 100}, centerIDs, actor.UserID)
	if err != nil {
		return nil, nil, err
	}
	classIDs := make([]bson.ObjectID, 0, len(classes))
	for _, class := range classes {
		classIDs = append(classIDs, class.ID)
	}
	return centerIDs, classIDs, nil
}

func (s *Service) canAccessCenter(ctx context.Context, tenantID string, actor Actor, centerID string) bool {
	if actor.canAccessGlobal() {
		return true
	}
	center, err := s.repo.GetCenter(ctx, tenantID, centerID)
	return err == nil && center != nil && center.ManagerUserID == actor.UserID
}

func (s *Service) canAccessClass(ctx context.Context, tenantID string, actor Actor, class Class) bool {
	return actor.canAccessGlobal() || class.HomeroomTeacherID == actor.UserID || s.canAccessCenter(ctx, tenantID, actor, class.CenterID.Hex())
}

func (s *Service) canAccessStudent(ctx context.Context, tenantID string, actor Actor, student Student) bool {
	if actor.canAccessGlobal() {
		return true
	}
	class, err := s.repo.GetClass(ctx, tenantID, student.ClassID.Hex())
	return err == nil && class != nil && s.canAccessClass(ctx, tenantID, actor, *class)
}

func (s *Service) studentsToDTO(ctx context.Context, tenantID string, students []Student) []StudentListItemDTO {
	items := make([]StudentListItemDTO, 0, len(students))
	centerNames := map[string]string{}
	classNames := map[string]string{}
	for _, student := range students {
		centerID := student.CenterID.Hex()
		classID := student.ClassID.Hex()
		if _, ok := centerNames[centerID]; !ok {
			if center, _ := s.repo.GetCenter(ctx, tenantID, centerID); center != nil {
				centerNames[centerID] = center.Name
			}
		}
		if _, ok := classNames[classID]; !ok {
			if class, _ := s.repo.GetClass(ctx, tenantID, classID); class != nil {
				classNames[classID] = class.Name
			}
		}
		items = append(items, StudentListItemDTO{
			ID:                   student.ID.Hex(),
			FullName:             student.FullName,
			Username:             student.Username,
			CenterID:             centerID,
			CenterName:           centerNames[centerID],
			ClassID:              classID,
			ClassName:            classNames[classID],
			Status:               student.Status,
			AverageScore:         student.Metrics.AverageScore,
			CompletedAssignments: student.Metrics.CompletedAssignments,
			LastActivityAt:       student.Metrics.LastActivityAt,
		})
	}
	return items
}

func (a Actor) canAccessGlobal() bool {
	for _, role := range a.Roles {
		switch strings.ToLower(role) {
		case "admin", "erg_admin", "global_admin", "lms_admin":
			return true
		}
	}
	return false
}

func nonAdminUser(actor Actor, canGlobal bool) string {
	if canGlobal {
		return ""
	}
	return actor.UserID
}

func centersToDTO(items []Center) []CenterResponseDTO {
	out := make([]CenterResponseDTO, 0, len(items))
	for _, item := range items {
		out = append(out, centerToDTO(item))
	}
	return out
}

func centerToDTO(center Center) CenterResponseDTO {
	return CenterResponseDTO{ID: center.ID.Hex(), Name: center.Name, Code: center.Code, Address: center.Address, Status: center.Status, ManagerUserID: center.ManagerUserID, CreatedAt: center.CreatedAt, UpdatedAt: center.UpdatedAt}
}

func classesToDTO(items []Class, centerNames map[string]string) []ClassResponseDTO {
	out := make([]ClassResponseDTO, 0, len(items))
	for _, item := range items {
		out = append(out, classToDTO(item, centerNames[item.CenterID.Hex()]))
	}
	return out
}

func classToDTO(class Class, centerName string) ClassResponseDTO {
	return ClassResponseDTO{ID: class.ID.Hex(), CenterID: class.CenterID.Hex(), CenterName: centerName, Name: class.Name, Grade: class.Grade, Status: class.Status, HomeroomTeacherID: class.HomeroomTeacherID, CreatedAt: class.CreatedAt, UpdatedAt: class.UpdatedAt}
}

func usernameFromName(name string) string {
	username := strings.ToLower(strings.TrimSpace(name))
	username = strings.NewReplacer(" ", ".", "/", ".", "\\", ".", "@", "", "'", "").Replace(username)
	if username == "" {
		username = "student"
	}
	return fmt.Sprintf("%s.%d", username, time.Now().Unix()%100000)
}

func secureTempPassword() (string, error) {
	n, err := crypto_rand.Int(crypto_rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", fmt.Errorf("lms: generate temporary password: %w", err)
	}
	return fmt.Sprintf("ERG%06d", n.Int64()), nil
}

func writeServiceError(statusCode func(int, string, string), err error) {
	switch {
	case errors.Is(err, errScopeForbidden):
		statusCode(http.StatusForbidden, "SCOPE_FORBIDDEN", "scope forbidden")
	case errors.Is(err, errNotFound):
		statusCode(http.StatusNotFound, "NOT_FOUND", "resource not found")
	case errors.Is(err, errSheetAccess):
		statusCode(http.StatusForbidden, "SHEET_ACCESS_DENIED", "sheet access denied")
	case errors.Is(err, errEmptyRecipient):
		statusCode(http.StatusUnprocessableEntity, "ASSIGNMENT_RECIPIENT_EMPTY", "assignment recipient empty")
	case errors.Is(err, errAttemptDone):
		statusCode(http.StatusConflict, "ATTEMPT_ALREADY_SUBMITTED", "attempt already submitted")
	case errors.Is(err, errHashMismatch):
		statusCode(http.StatusConflict, "ATTEMPT_PACKAGE_HASH_MISMATCH", "attempt package hash mismatch")
	case errors.Is(err, errInvalidFieldKey):
		statusCode(http.StatusBadRequest, "INVALID_FIELD_KEY", "invalid field key")
	default:
		statusCode(http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}
}
