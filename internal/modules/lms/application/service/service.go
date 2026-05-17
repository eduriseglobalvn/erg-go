package service

import (
	"context"
	crypto_rand "crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	auditservice "erg.ninja/internal/modules/audit/application/service"
	authrepo "erg.ninja/internal/modules/auth/infrastructure/repository"
	"erg.ninja/pkg/storage"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var (
	errScopeForbidden           = errors.New("SCOPE_FORBIDDEN")
	errNotFound                 = errors.New("NOT_FOUND")
	errSheetAccess              = errors.New("SHEET_ACCESS_DENIED")
	errEmptyRecipient           = errors.New("ASSIGNMENT_RECIPIENT_EMPTY")
	errAttemptDone              = errors.New("ATTEMPT_ALREADY_SUBMITTED")
	errHashMismatch             = errors.New("ATTEMPT_PACKAGE_HASH_MISMATCH")
	errInvalidFieldKey          = errors.New("INVALID_FIELD_KEY")
	errInvalidQuestionKind      = errors.New("INVALID_QUESTION_KIND")
	errInvalidEducationUnitType = errors.New("INVALID_EDUCATION_UNIT_TYPE")
)

type Actor struct {
	UserID string
	Roles  []string
}

type Service struct {
	repo       *Repository
	sheets     *storage.GoogleSheetsClient
	audit      auditservice.Publisher
	authRepo   *authrepo.Repo
	accessRepo *accessManagementRepository
}

type ServiceOption func(*Service)

func WithAuthRepository(repo *authrepo.Repo) ServiceOption {
	return func(s *Service) { s.authRepo = repo }
}

func WithAccessManagementRepository(repo *accessManagementRepository) ServiceOption {
	return func(s *Service) { s.accessRepo = repo }
}

func NewService(repo *Repository, sheets *storage.GoogleSheetsClient, opts ...ServiceOption) *Service {
	svc := &Service{repo: repo, sheets: sheets, audit: auditservice.NoopPublisher{}}
	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}
	return svc
}

func (s *Service) Scope(ctx context.Context, tenantID string, actor Actor) (ManagementScopeResponseDTO, error) {
	canGlobal := actor.canAccessGlobal()
	assignedCenters, _, err := s.repo.ListCenters(ctx, tenantID, CenterListRequestDTO{Page: 1, Limit: 100}, nonAdminUser(actor, canGlobal))
	if err != nil {
		return ManagementScopeResponseDTO{}, err
	}
	centerIDs := make([]bson.ObjectID, 0, len(assignedCenters))
	for _, center := range assignedCenters {
		if centerType(center) == educationUnitTypeSystem {
			continue
		}
		centerIDs = append(centerIDs, center.ID)
	}
	assignedClasses, err := s.scopeClasses(ctx, tenantID, actor, canGlobal, centerIDs)
	if err != nil {
		return ManagementScopeResponseDTO{}, err
	}
	centerNames, centerTypes := s.centerInfoForClasses(ctx, tenantID, assignedCenters, assignedClasses)
	current, err := s.currentOrDefault(ctx, tenantID, actor, canGlobal, assignedCenters, assignedClasses)
	if err != nil {
		return ManagementScopeResponseDTO{}, err
	}
	assignedCenterDTOs := centersToDTO(assignedCenters)
	assignedClassDTOs := classesToDTO(assignedClasses, centerNames)
	current = s.decorateScope(ctx, tenantID, current)
	return ManagementScopeResponseDTO{
		CanAccessGlobalErg: canGlobal,
		AssignedCenters:    assignedCenterDTOs,
		AssignedClasses:    assignedClassDTOs,
		CurrentScope:       current,
		AvailableScopes:    availableScopes(canGlobal, assignedCenterDTOs, assignedClassDTOs, centerTypes),
	}, nil
}

func (s *Service) UpdateScope(ctx context.Context, tenantID string, actor Actor, req UpdateCurrentScopeRequestDTO) (CurrentScopeResponseDTO, error) {
	scope := ManagementScopeDTO{Level: normalizeScopeLevel(req.Level), CenterID: req.CenterID, ClassID: req.ClassID}
	if err := s.validateScope(ctx, tenantID, actor, scope); err != nil {
		return CurrentScopeResponseDTO{}, err
	}
	err := s.repo.UpsertCurrentScope(ctx, &CurrentScope{
		TenantID: tenantID,
		UserID:   actor.UserID,
		Level:    scope.Level,
		CenterID: req.CenterID,
		ClassID:  req.ClassID,
	})
	if err != nil {
		return CurrentScopeResponseDTO{}, err
	}
	return CurrentScopeResponseDTO{CurrentScope: s.decorateScope(ctx, tenantID, scope)}, nil
}

func (s *Service) ListCenters(ctx context.Context, tenantID string, actor Actor, req CenterListRequestDTO) (CenterListResponseDTO, error) {
	if req.Type != "" {
		req.Type = normalizeEducationUnitType(req.Type)
		if !isEducationUnitType(req.Type) {
			return CenterListResponseDTO{}, errInvalidEducationUnitType
		}
	}
	items, total, err := s.repo.ListCenters(ctx, tenantID, req, nonAdminUser(actor, actor.canAccessGlobal()))
	return CenterListResponseDTO{Items: centersToDTO(items), Total: total}, err
}

func (s *Service) CreateCenter(ctx context.Context, tenantID string, actor Actor, req CreateCenterRequestDTO) (CenterResponseDTO, error) {
	unitType := normalizeEducationUnitType(req.Type)
	if unitType == "" {
		unitType = educationUnitTypeCenter
	}
	return s.createEducationUnit(ctx, tenantID, actor, CreateEducationUnitRequestDTO{
		Type:          unitType,
		Name:          req.Name,
		Code:          req.Code,
		Address:       req.Address,
		ManagerUserID: req.ManagerUserID,
	})
}

func (s *Service) ListEducationUnits(ctx context.Context, tenantID string, actor Actor, req CenterListRequestDTO) (EducationUnitListResponseDTO, error) {
	return s.ListCenters(ctx, tenantID, actor, req)
}

func (s *Service) CreateEducationUnit(ctx context.Context, tenantID string, actor Actor, req CreateEducationUnitRequestDTO) (EducationUnitResponseDTO, error) {
	req.Type = normalizeEducationUnitType(req.Type)
	return s.createEducationUnit(ctx, tenantID, actor, req)
}

func (s *Service) createEducationUnit(ctx context.Context, tenantID string, actor Actor, req CreateEducationUnitRequestDTO) (CenterResponseDTO, error) {
	if !actor.canAccessGlobal() {
		return CenterResponseDTO{}, errScopeForbidden
	}
	if !isEducationUnitType(req.Type) {
		return CenterResponseDTO{}, errInvalidEducationUnitType
	}
	parentID, err := optionalObjectID(req.ParentID)
	if err != nil {
		return CenterResponseDTO{}, errInvalidAccessPolicy
	}
	center := &Center{
		TenantID:      tenantID,
		Type:          req.Type,
		Name:          req.Name,
		Code:          req.Code,
		ParentID:      parentID,
		AvatarURL:     req.AvatarURL,
		Address:       req.Address,
		Description:   req.Description,
		Phone:         req.Phone,
		Email:         req.Email,
		Website:       req.Website,
		ManagerUserID: req.ManagerUserID,
	}
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
	if req.Type != nil {
		unitType := normalizeEducationUnitType(*req.Type)
		if !isEducationUnitType(unitType) {
			return CenterResponseDTO{}, errInvalidEducationUnitType
		}
		update["type"] = unitType
	}
	if req.Name != nil {
		update["name"] = *req.Name
	}
	if req.ParentID != nil {
		parentID, err := optionalObjectID(*req.ParentID)
		if err != nil {
			return CenterResponseDTO{}, errInvalidAccessPolicy
		}
		update["parent_id"] = parentID
	}
	if req.AvatarURL != nil {
		update["avatar_url"] = *req.AvatarURL
	}
	if req.Address != nil {
		update["address"] = *req.Address
	}
	if req.Description != nil {
		update["description"] = *req.Description
	}
	if req.Phone != nil {
		update["phone"] = *req.Phone
	}
	if req.Email != nil {
		update["email"] = *req.Email
	}
	if req.Website != nil {
		update["website"] = *req.Website
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

func (s *Service) GetEducationUnit(ctx context.Context, tenantID string, actor Actor, id string) (EducationUnitResponseDTO, error) {
	center, err := s.repo.GetCenter(ctx, tenantID, id)
	if err != nil {
		return EducationUnitResponseDTO{}, err
	}
	if center == nil {
		return EducationUnitResponseDTO{}, errNotFound
	}
	if !actor.canAccessGlobal() && !s.canAccessCenter(ctx, tenantID, actor, id) && !s.hasClassInCenter(ctx, tenantID, actor, id) {
		return EducationUnitResponseDTO{}, errScopeForbidden
	}
	return centerToDTO(*center), nil
}

func (s *Service) UpdateEducationUnit(ctx context.Context, tenantID string, actor Actor, id string, req UpdateEducationUnitRequestDTO) (EducationUnitResponseDTO, error) {
	return s.UpdateCenter(ctx, tenantID, actor, id, req)
}

func (s *Service) ListEducationUnitClasses(ctx context.Context, tenantID string, actor Actor, id string, req ClassListRequestDTO) (ClassListResponseDTO, error) {
	center, err := s.repo.GetCenter(ctx, tenantID, id)
	if err != nil {
		return ClassListResponseDTO{}, err
	}
	if center == nil {
		return ClassListResponseDTO{}, errNotFound
	}
	req.CenterID = id
	req.UnitID = id
	var homeroomTeacherID string
	if !actor.canAccessGlobal() && !s.canAccessCenter(ctx, tenantID, actor, id) {
		if actor.UserID == "" || !s.hasClassInCenter(ctx, tenantID, actor, id) {
			return ClassListResponseDTO{}, errScopeForbidden
		}
		homeroomTeacherID = actor.UserID
	}
	items, total, err := s.repo.ListClasses(ctx, tenantID, req, nil, homeroomTeacherID)
	if err != nil {
		return ClassListResponseDTO{}, err
	}
	return ClassListResponseDTO{Items: classesToDTO(items, map[string]string{id: center.Name}), Total: total}, nil
}

func (s *Service) ListClasses(ctx context.Context, tenantID string, actor Actor, req ClassListRequestDTO) (ClassListResponseDTO, error) {
	if req.CenterID == "" {
		req.CenterID = req.UnitID
	}
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
	class := &Class{TenantID: tenantID, CenterID: centerID, Name: req.Name, Grade: req.Grade, AcademicYear: req.AcademicYear, HomeroomTeacherID: req.HomeroomTeacherID}
	if err := s.repo.CreateClass(ctx, class); err != nil {
		return ClassResponseDTO{}, err
	}
	return classToDTO(*class, center.Name), nil
}

func (s *Service) GetClass(ctx context.Context, tenantID string, actor Actor, id string) (ClassResponseDTO, error) {
	class, err := s.repo.GetClass(ctx, tenantID, id)
	if err != nil {
		return ClassResponseDTO{}, err
	}
	if class == nil {
		return ClassResponseDTO{}, errNotFound
	}
	if !s.canAccessClass(ctx, tenantID, actor, *class) {
		return ClassResponseDTO{}, errScopeForbidden
	}
	centerName := ""
	if center, _ := s.repo.GetCenter(ctx, tenantID, class.CenterID.Hex()); center != nil {
		centerName = center.Name
	}
	return classToDTO(*class, centerName), nil
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
	if req.AcademicYear != nil {
		update["academic_year"] = *req.AcademicYear
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

func (s *Service) ListClassStudents(ctx context.Context, tenantID string, actor Actor, classID string, req StudentListRequestDTO) (StudentListResponseDTO, error) {
	class, err := s.repo.GetClass(ctx, tenantID, classID)
	if err != nil {
		return StudentListResponseDTO{}, err
	}
	if class == nil {
		return StudentListResponseDTO{}, errNotFound
	}
	if !s.canAccessClass(ctx, tenantID, actor, *class) {
		return StudentListResponseDTO{}, errScopeForbidden
	}
	req.ClassID = classID
	req.CenterID = class.CenterID.Hex()
	items, total, next, err := s.repo.ListStudents(ctx, tenantID, req, nil, nil)
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
	switch normalizeScopeLevel(scope.Level) {
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

func (s *Service) scopeClasses(ctx context.Context, tenantID string, actor Actor, canGlobal bool, managedCenterIDs []bson.ObjectID) ([]Class, error) {
	if canGlobal {
		classes, _, err := s.repo.ListClasses(ctx, tenantID, ClassListRequestDTO{Page: 1, Limit: 100}, nil, "")
		return classes, err
	}
	seen := map[bson.ObjectID]bool{}
	items := []Class{}
	if len(managedCenterIDs) > 0 {
		classes, _, err := s.repo.ListClasses(ctx, tenantID, ClassListRequestDTO{Page: 1, Limit: 100}, managedCenterIDs, "")
		if err != nil {
			return nil, err
		}
		for _, class := range classes {
			if !seen[class.ID] {
				seen[class.ID] = true
				items = append(items, class)
			}
		}
	}
	if actor.UserID != "" {
		classes, _, err := s.repo.ListClasses(ctx, tenantID, ClassListRequestDTO{Page: 1, Limit: 100}, nil, actor.UserID)
		if err != nil {
			return nil, err
		}
		for _, class := range classes {
			if !seen[class.ID] {
				seen[class.ID] = true
				items = append(items, class)
			}
		}
	}
	return items, nil
}

func (s *Service) canAccessCenter(ctx context.Context, tenantID string, actor Actor, centerID string) bool {
	if actor.canAccessGlobal() {
		return true
	}
	center, err := s.repo.GetCenter(ctx, tenantID, centerID)
	if err != nil || center == nil {
		return false
	}
	if center.ManagerUserID == actor.UserID {
		return true
	}
	if centerType(*center) == educationUnitTypeSchool && !center.ParentID.IsZero() {
		parent, err := s.repo.GetCenter(ctx, tenantID, center.ParentID.Hex())
		return err == nil && parent != nil && parent.ManagerUserID == actor.UserID
	}
	return false
}

func (s *Service) hasClassInCenter(ctx context.Context, tenantID string, actor Actor, centerID string) bool {
	if actor.UserID == "" {
		return false
	}
	items, total, err := s.repo.ListClasses(ctx, tenantID, ClassListRequestDTO{CenterID: centerID, Page: 1, Limit: 1}, nil, actor.UserID)
	return err == nil && total > 0 && len(items) > 0
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

func normalizeScopeLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case scopeLevelSystem:
		return scopeLevelGlobal
	default:
		return strings.ToLower(strings.TrimSpace(level))
	}
}

func normalizeEducationUnitType(unitType string) string {
	return strings.ToLower(strings.TrimSpace(unitType))
}

func isEducationUnitType(unitType string) bool {
	return unitType == educationUnitTypeSchool || unitType == educationUnitTypeCenter
}

func centerType(center Center) string {
	unitType := normalizeEducationUnitType(center.Type)
	if unitType == "" {
		return educationUnitTypeCenter
	}
	return unitType
}

func scopeBadge(unitType string) string {
	switch unitType {
	case educationUnitTypeSystem:
		return "System"
	case educationUnitTypeSchool:
		return "School"
	case educationUnitTypeCenter:
		return "Center"
	default:
		return "Class"
	}
}

func scopeIcon(unitType string) string {
	switch unitType {
	case educationUnitTypeSystem:
		return "shield"
	case educationUnitTypeSchool:
		return "school"
	case educationUnitTypeCenter:
		return "building-2"
	default:
		return "users"
	}
}

func availableScopes(canGlobal bool, centers []CenterResponseDTO, classes []ClassResponseDTO, centerTypes map[string]string) []ManagementScopeDTO {
	out := []ManagementScopeDTO{}
	if canGlobal {
		out = append(out, systemScopeDTO())
	}
	for _, center := range centers {
		unitType := normalizeEducationUnitType(center.Type)
		if unitType == educationUnitTypeSystem {
			continue
		}
		out = append(out, ManagementScopeDTO{
			Level:       scopeLevelCenter,
			Type:        unitType,
			UnitType:    unitType,
			CenterID:    center.ID,
			CenterName:  center.Name,
			Badge:       scopeBadge(unitType),
			Icon:        scopeIcon(unitType),
			Description: educationUnitScopeDescription(unitType, center.Name),
		})
	}
	for _, class := range classes {
		unitType := centerTypes[class.CenterID]
		if unitType == "" {
			unitType = educationUnitTypeCenter
		}
		out = append(out, ManagementScopeDTO{
			Level:       scopeLevelClass,
			Type:        unitType,
			UnitType:    unitType,
			CenterID:    class.CenterID,
			CenterName:  class.CenterName,
			ClassID:     class.ID,
			ClassName:   class.Name,
			Badge:       "Class",
			Icon:        "users",
			Description: classScopeDescription(unitType, class.CenterName, class.Name),
		})
	}
	return out
}

func systemScopeDTO() ManagementScopeDTO {
	return ManagementScopeDTO{
		Level:       scopeLevelGlobal,
		Type:        scopeLevelSystem,
		CenterName:  "Hệ thống ERG",
		Badge:       scopeBadge(scopeLevelSystem),
		Icon:        scopeIcon(scopeLevelSystem),
		Description: "Phạm vi toàn hệ thống ERG LMS",
	}
}

func educationUnitScopeDescription(unitType, name string) string {
	if unitType == educationUnitTypeSchool {
		return "School scope: " + name
	}
	return "Center scope: " + name
}

func classScopeDescription(unitType, centerName, className string) string {
	prefix := "Center"
	if unitType == educationUnitTypeSchool {
		prefix = "School"
	}
	if centerName == "" {
		return prefix + " class scope: " + className
	}
	return prefix + " class scope: " + centerName + " / " + className
}

func (s *Service) decorateScope(ctx context.Context, tenantID string, scope ManagementScopeDTO) ManagementScopeDTO {
	scope.Level = normalizeScopeLevel(scope.Level)
	switch scope.Level {
	case scopeLevelGlobal:
		return systemScopeDTO()
	case scopeLevelCenter:
		if center, _ := s.repo.GetCenter(ctx, tenantID, scope.CenterID); center != nil {
			unitType := centerType(*center)
			scope.Type = unitType
			scope.UnitType = unitType
			scope.CenterName = center.Name
			scope.Badge = scopeBadge(unitType)
			scope.Icon = scopeIcon(unitType)
			scope.Description = educationUnitScopeDescription(unitType, center.Name)
		}
	case scopeLevelClass:
		if class, _ := s.repo.GetClass(ctx, tenantID, scope.ClassID); class != nil {
			scope.CenterID = class.CenterID.Hex()
			scope.ClassName = class.Name
			unitType := educationUnitTypeCenter
			centerName := ""
			if center, _ := s.repo.GetCenter(ctx, tenantID, class.CenterID.Hex()); center != nil {
				unitType = centerType(*center)
				centerName = center.Name
			}
			scope.Type = unitType
			scope.UnitType = unitType
			scope.CenterName = centerName
			scope.Badge = "Class"
			scope.Icon = "users"
			scope.Description = classScopeDescription(unitType, centerName, class.Name)
		}
	}
	return scope
}

func (s *Service) centerInfoForClasses(ctx context.Context, tenantID string, centers []Center, classes []Class) (map[string]string, map[string]string) {
	names := map[string]string{}
	types := map[string]string{}
	for _, center := range centers {
		names[center.ID.Hex()] = center.Name
		types[center.ID.Hex()] = centerType(center)
	}
	for _, class := range classes {
		centerID := class.CenterID.Hex()
		if _, ok := names[centerID]; ok {
			continue
		}
		if center, _ := s.repo.GetCenter(ctx, tenantID, centerID); center != nil {
			names[centerID] = center.Name
			types[centerID] = centerType(*center)
		}
	}
	return names, types
}

func centersToDTO(items []Center) []CenterResponseDTO {
	out := make([]CenterResponseDTO, 0, len(items))
	for _, item := range items {
		out = append(out, centerToDTO(item))
	}
	return out
}

func centerToDTO(center Center) CenterResponseDTO {
	parentID := ""
	if !center.ParentID.IsZero() {
		parentID = center.ParentID.Hex()
	}
	return CenterResponseDTO{
		ID:            center.ID.Hex(),
		Type:          centerType(center),
		Name:          center.Name,
		Code:          center.Code,
		ParentID:      parentID,
		AvatarURL:     center.AvatarURL,
		Address:       center.Address,
		Description:   center.Description,
		Phone:         center.Phone,
		Email:         center.Email,
		Website:       center.Website,
		Status:        center.Status,
		ManagerUserID: center.ManagerUserID,
		CreatedAt:     center.CreatedAt,
		UpdatedAt:     center.UpdatedAt,
	}
}

func optionalObjectID(id string) (bson.ObjectID, error) {
	if strings.TrimSpace(id) == "" {
		return bson.NilObjectID, nil
	}
	return objectID(id)
}

func classesToDTO(items []Class, centerNames map[string]string) []ClassResponseDTO {
	out := make([]ClassResponseDTO, 0, len(items))
	for _, item := range items {
		out = append(out, classToDTO(item, centerNames[item.CenterID.Hex()]))
	}
	return out
}

func classToDTO(class Class, centerName string) ClassResponseDTO {
	centerID := class.CenterID.Hex()
	return ClassResponseDTO{ID: class.ID.Hex(), CenterID: centerID, UnitID: centerID, CenterName: centerName, Name: class.Name, Grade: class.Grade, AcademicYear: class.AcademicYear, Status: class.Status, HomeroomTeacherID: class.HomeroomTeacherID, CreatedAt: class.CreatedAt, UpdatedAt: class.UpdatedAt}
}

func usernameFromName(name string) string {
	username := normalizeUsername(name)
	if username == "" {
		username = "student"
	}
	if len(username) > 56 {
		username = strings.Trim(username[:56], "._-")
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
	case errors.Is(err, errInvalidQuestionKind):
		statusCode(http.StatusBadRequest, "INVALID_QUESTION_KIND", "invalid question kind")
	case errors.Is(err, errInvalidEducationUnitType):
		statusCode(http.StatusBadRequest, "INVALID_EDUCATION_UNIT_TYPE", "education unit type must be school or center")
	case errors.Is(err, errInvalidStudentAccountPayload):
		statusCode(http.StatusBadRequest, "INVALID_STUDENT_ACCOUNT_PAYLOAD", "invalid student account payload")
	case errors.Is(err, errInvalidAccessPolicy):
		statusCode(http.StatusBadRequest, "INVALID_ACCESS_POLICY", err.Error())
	case errors.Is(err, errAccessManagementUnavailable):
		statusCode(http.StatusServiceUnavailable, "ACCESS_MANAGEMENT_UNAVAILABLE", "access management storage is unavailable")
	default:
		statusCode(http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}
}
