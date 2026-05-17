package service

import (
	"context"
	"errors"
	"strings"

	authrepo "erg.ninja/internal/modules/auth/infrastructure/repository"
	"go.mongodb.org/mongo-driver/v2/bson"
)

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
	profile, err := normalizeCreateStudentProfile(req)
	if err != nil {
		return StudentResponseDTO{}, err
	}
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
	username := normalizeStudentUsername(req.Username)
	if username == "" {
		duplicate := false
		username, duplicate, err = s.uniqueStudentUsername(ctx, tenantID, profile.FullName, class.Name)
		if err != nil {
			return StudentResponseDTO{}, err
		}
		_ = duplicate
	} else {
		if err := validateStudentUsername(username); err != nil {
			return StudentResponseDTO{}, err
		}
		exists, err := s.repo.UsernameExists(ctx, tenantID, username)
		if err != nil {
			return StudentResponseDTO{}, err
		}
		if exists {
			return StudentResponseDTO{}, errInvalidStudentAccountPayload
		}
	}
	if strings.TrimSpace(req.Password) != "" {
		tempPassword = strings.TrimSpace(req.Password)
	}
	if err := validateStudentPassword(tempPassword, username, profile.FullName); err != nil {
		return StudentResponseDTO{}, err
	}
	authUserID := ""
	if s.authRepo != nil {
		authUser, err := s.createStudentAuthUser(ctx, tenantID, profile.FullName, username, tempPassword, BulkCreateStudentAccountRowDTO{
			FullName:  profile.FullName,
			Username:  username,
			Password:  tempPassword,
			Birthday:  req.Birthday,
			Phone:     profile.Phone,
			ClassID:   class.ID.Hex(),
			ClassName: class.Name,
		})
		if err != nil {
			if errors.Is(err, authrepo.ErrDuplicateEmail) {
				return StudentResponseDTO{}, errInvalidStudentAccountPayload
			}
			return StudentResponseDTO{}, err
		}
		authUserID = authUser.ID.Hex()
	}
	student := &Student{
		TenantID:           tenantID,
		CenterID:           class.CenterID,
		ClassID:            class.ID,
		StudentCode:        profile.StudentCode,
		FullName:           profile.FullName,
		Username:           username,
		AuthUserID:         authUserID,
		Email:              profile.Email,
		Gender:             profile.Gender,
		Birthday:           req.Birthday,
		Phone:              profile.Phone,
		Address:            profile.Address,
		ParentName:         profile.ParentName,
		ParentPhone:        profile.ParentPhone,
		ParentEmail:        profile.ParentEmail,
		ParentRelationship: profile.ParentRelationship,
		EnrollmentDate:     req.EnrollmentDate,
		Note:               profile.Note,
	}
	if err := s.repo.CreateStudent(ctx, student); err != nil {
		if s.authRepo != nil && authUserID != "" {
			if oid, oidErr := objectID(authUserID); oidErr == nil {
				_ = s.authRepo.DeleteUserByID(ctx, oid)
			}
		}
		return StudentResponseDTO{}, err
	}
	resp := StudentResponseDTO{Student: s.studentsToDTO(ctx, tenantID, []Student{*student})[0]}
	if s.authRepo != nil {
		resp.TempPassword = tempPassword
	}
	return resp, nil
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
	profile := normalizedStudentProfile{
		StudentCode:        student.StudentCode,
		FullName:           student.FullName,
		Email:              student.Email,
		Gender:             student.Gender,
		Phone:              student.Phone,
		Address:            student.Address,
		ParentName:         student.ParentName,
		ParentPhone:        student.ParentPhone,
		ParentEmail:        student.ParentEmail,
		ParentRelationship: student.ParentRelationship,
		Note:               student.Note,
	}
	if req.StudentCode != nil {
		profile.StudentCode = *req.StudentCode
	}
	update := bson.M{}
	if req.FullName != nil {
		profile.FullName = *req.FullName
	}
	if req.Email != nil {
		profile.Email = *req.Email
	}
	if req.Gender != nil {
		profile.Gender = *req.Gender
	}
	if req.Phone != nil {
		profile.Phone = *req.Phone
	}
	if req.Address != nil {
		profile.Address = *req.Address
	}
	if req.ParentName != nil {
		profile.ParentName = *req.ParentName
	}
	if req.ParentPhone != nil {
		profile.ParentPhone = *req.ParentPhone
	}
	if req.ParentEmail != nil {
		profile.ParentEmail = *req.ParentEmail
	}
	if req.ParentRelationship != nil {
		profile.ParentRelationship = *req.ParentRelationship
	}
	if req.Note != nil {
		profile.Note = *req.Note
	}
	normalizedProfile, err := normalizeStudentProfile(profile)
	if err != nil {
		return StudentResponseDTO{}, err
	}
	update["student_code"] = normalizedProfile.StudentCode
	update["full_name"] = normalizedProfile.FullName
	update["email"] = normalizedProfile.Email
	update["gender"] = normalizedProfile.Gender
	update["phone"] = normalizedProfile.Phone
	update["address"] = normalizedProfile.Address
	update["parent_name"] = normalizedProfile.ParentName
	update["parent_phone"] = normalizedProfile.ParentPhone
	update["parent_email"] = normalizedProfile.ParentEmail
	update["parent_relationship"] = normalizedProfile.ParentRelationship
	update["note"] = normalizedProfile.Note
	if req.Birthday != nil {
		update["birthday"] = *req.Birthday
	}
	if req.EnrollmentDate != nil {
		update["enrollment_date"] = *req.EnrollmentDate
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

func (s *Service) studentsToDTO(ctx context.Context, tenantID string, students []Student) []StudentListItemDTO {
	items := make([]StudentListItemDTO, 0, len(students))
	centers := map[string]Center{}
	classes := map[string]Class{}
	for _, student := range students {
		centerID := student.CenterID.Hex()
		classID := student.ClassID.Hex()
		if _, ok := centers[centerID]; !ok {
			if center, _ := s.repo.GetCenter(ctx, tenantID, centerID); center != nil {
				centers[centerID] = *center
			}
		}
		if _, ok := classes[classID]; !ok {
			if class, _ := s.repo.GetClass(ctx, tenantID, classID); class != nil {
				classes[classID] = *class
			}
		}
		center := centers[centerID]
		class := classes[classID]
		items = append(items, StudentListItemDTO{
			ID:                   student.ID.Hex(),
			StudentCode:          student.StudentCode,
			FullName:             student.FullName,
			Username:             student.Username,
			Email:                student.Email,
			Gender:               student.Gender,
			Birthday:             student.Birthday,
			Phone:                student.Phone,
			Address:              student.Address,
			ParentName:           student.ParentName,
			ParentPhone:          student.ParentPhone,
			ParentEmail:          student.ParentEmail,
			ParentRelationship:   student.ParentRelationship,
			EnrollmentDate:       student.EnrollmentDate,
			Note:                 student.Note,
			CenterID:             centerID,
			CenterName:           center.Name,
			CenterCode:           center.Code,
			CenterType:           centerType(center),
			ClassID:              classID,
			ClassName:            class.Name,
			Grade:                class.Grade,
			AcademicYear:         class.AcademicYear,
			Status:               student.Status,
			AverageScore:         student.Metrics.AverageScore,
			CompletedAssignments: student.Metrics.CompletedAssignments,
			LastActivityAt:       student.Metrics.LastActivityAt,
		})
	}
	return items
}
