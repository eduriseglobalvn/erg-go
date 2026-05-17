package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	authentities "erg.ninja/internal/modules/auth/domain/entity"
	authrepo "erg.ninja/internal/modules/auth/infrastructure/repository"
	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	defaultStudentImportPassword = "123456"
	studentAccountEmailDomain    = "student.erg.edu.vn"
)

var errStudentAccountProvisioningUnavailable = errors.New("STUDENT_ACCOUNT_PROVISIONING_UNAVAILABLE")

func (s *Service) BulkCreateStudentAccounts(ctx context.Context, tenantID string, actor Actor, req BulkCreateStudentAccountsRequestDTO) (BulkCreateStudentAccountsResponseDTO, error) {
	if s.authRepo == nil {
		return BulkCreateStudentAccountsResponseDTO{}, errStudentAccountProvisioningUnavailable
	}
	center, err := s.repo.GetCenter(ctx, tenantID, req.CenterID)
	if err != nil {
		return BulkCreateStudentAccountsResponseDTO{}, err
	}
	if center == nil {
		return BulkCreateStudentAccountsResponseDTO{}, errNotFound
	}
	if !actor.canAccessGlobal() && !s.canAccessCenter(ctx, tenantID, actor, req.CenterID) {
		return BulkCreateStudentAccountsResponseDTO{}, errScopeForbidden
	}

	result := BulkCreateStudentAccountsResponseDTO{
		Credentials: []ImportCredentialDTO{},
		Students:    []StudentListItemDTO{},
		FailedItems: []BulkActionFailedItemDTO{},
	}
	createdStudents := make([]Student, 0, len(req.Rows))

	for _, row := range req.Rows {
		rowID := firstNonEmpty(row.RowID, fmt.Sprintf("row-%d", row.RowNumber))
		if row.Included != nil && !*row.Included {
			result.Skipped++
			continue
		}
		fullName := strings.TrimSpace(row.FullName)
		username := normalizeStudentUsername(row.Username)
		password := strings.TrimSpace(row.Password)
		if password == "" {
			password = defaultStudentImportPassword
		}
		if fullName == "" || username == "" {
			result.Skipped++
			result.FailedItems = append(result.FailedItems, BulkActionFailedItemDTO{ID: rowID, Code: "invalid_row", Message: "missing fullName or username"})
			continue
		}

		class, err := s.resolveImportClass(ctx, tenantID, center.ID, firstNonEmpty(row.ClassID, req.ClassID), row.ClassName)
		if err != nil {
			result.Skipped++
			result.FailedItems = append(result.FailedItems, BulkActionFailedItemDTO{ID: rowID, Code: "class_not_found", Message: err.Error()})
			continue
		}

		exists, err := s.repo.UsernameExists(ctx, tenantID, username)
		if err != nil {
			return BulkCreateStudentAccountsResponseDTO{}, err
		}
		if exists {
			result.Duplicates++
			result.Skipped++
			result.FailedItems = append(result.FailedItems, BulkActionFailedItemDTO{ID: rowID, Code: "duplicate_username", Message: "student username already exists"})
			continue
		}

		authUser, err := s.createStudentAuthUser(ctx, tenantID, fullName, username, password, row)
		if err != nil {
			if errors.Is(err, authrepo.ErrDuplicateEmail) {
				result.Duplicates++
				result.Skipped++
				result.FailedItems = append(result.FailedItems, BulkActionFailedItemDTO{ID: rowID, Code: "duplicate_username", Message: "student account username already exists"})
				continue
			}
			return BulkCreateStudentAccountsResponseDTO{}, err
		}

		student := &Student{
			TenantID:   tenantID,
			CenterID:   class.CenterID,
			ClassID:    class.ID,
			FullName:   fullName,
			Username:   username,
			AuthUserID: authUser.ID.Hex(),
			Birthday:   row.Birthday,
			Phone:      row.Phone,
			Note:       row.Note,
			Status:     statusActive,
			CreatedAt:  time.Now().UTC(),
			UpdatedAt:  time.Now().UTC(),
		}
		if err := s.repo.CreateStudent(ctx, student); err != nil {
			result.Skipped++
			result.FailedItems = append(result.FailedItems, BulkActionFailedItemDTO{ID: rowID, Code: "student_create_failed", Message: err.Error()})
			continue
		}
		result.Created++
		createdStudents = append(createdStudents, *student)
		result.Credentials = append(result.Credentials, ImportCredentialDTO{
			RowID:     rowID,
			RowNumber: row.RowNumber,
			StudentID: student.ID.Hex(),
			Username:  username,
			Password:  password,
		})
	}

	if len(createdStudents) > 0 {
		result.Students = s.studentsToDTO(ctx, tenantID, createdStudents)
	}
	return result, nil
}

func (s *Service) createStudentAuthUser(ctx context.Context, tenantID, fullName, username, password string, row BulkCreateStudentAccountRowDTO) (*authentities.User, error) {
	passwordHash, err := authrepo.HashPassword(password)
	if err != nil {
		return nil, err
	}
	user := &authentities.User{
		ID:                 bson.NewObjectID(),
		Email:              studentAccountEmail(username),
		PasswordHash:       passwordHash,
		FullName:           fullName,
		Status:             authentities.UserStatusActive,
		Provider:           "local",
		AccountType:        "student",
		Roles:              []string{"lms.student", "student"},
		TenantID:           tenantID,
		Phone:              strings.TrimSpace(row.Phone),
		IsProfileCompleted: true,
	}
	if row.Birthday != nil {
		user.DateOfBirth = row.Birthday.Format("2006-01-02")
	}
	if err := s.authRepo.CreateUser(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func normalizeStudentUsername(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "")
	return value
}

func studentAccountEmail(username string) string {
	return normalizeStudentUsername(username) + "@" + studentAccountEmailDomain
}
