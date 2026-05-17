package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	authentities "erg.ninja/internal/modules/auth/domain/entity"
	authrepo "erg.ninja/internal/modules/auth/infrastructure/repository"
	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	maxBulkStudentAccountRows = 500
	studentAccountEmailDomain = "student.erg.edu.vn"
	studentPasswordMinLength  = 8
	studentPasswordMaxLength  = 128
)

var (
	errStudentAccountProvisioningUnavailable = errors.New("STUDENT_ACCOUNT_PROVISIONING_UNAVAILABLE")
	errInvalidStudentAccountPayload          = errors.New("INVALID_STUDENT_ACCOUNT_PAYLOAD")
	studentUsernamePattern                   = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._-]{1,62}[a-z0-9])?$`)
)

func (s *Service) BulkCreateStudentAccounts(ctx context.Context, tenantID string, actor Actor, req BulkCreateStudentAccountsRequestDTO) (BulkCreateStudentAccountsResponseDTO, error) {
	if s.authRepo == nil {
		return BulkCreateStudentAccountsResponseDTO{}, errStudentAccountProvisioningUnavailable
	}
	if len(req.Rows) == 0 || len(req.Rows) > maxBulkStudentAccountRows {
		return BulkCreateStudentAccountsResponseDTO{}, errInvalidStudentAccountPayload
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
	usernamesInRequest := map[string]string{}

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
			password, err = secureTempPassword()
			if err != nil {
				return BulkCreateStudentAccountsResponseDTO{}, err
			}
		}
		row.FullName = fullName
		profile, err := normalizeBulkStudentProfile(row)
		if err != nil {
			result.Skipped++
			result.FailedItems = append(result.FailedItems, BulkActionFailedItemDTO{ID: rowID, Code: "invalid_profile", Message: "invalid student profile fields"})
			continue
		}
		if profile.FullName == "" || username == "" {
			result.Skipped++
			result.FailedItems = append(result.FailedItems, BulkActionFailedItemDTO{ID: rowID, Code: "invalid_row", Message: "missing fullName or username"})
			continue
		}
		if err := validateStudentUsername(username); err != nil {
			result.Skipped++
			result.FailedItems = append(result.FailedItems, BulkActionFailedItemDTO{ID: rowID, Code: "invalid_username", Message: "username must be 3-64 characters and contain only lowercase letters, numbers, dot, underscore, or hyphen"})
			continue
		}
		if previousRowID, ok := usernamesInRequest[username]; ok {
			result.Duplicates++
			result.Skipped++
			result.FailedItems = append(result.FailedItems, BulkActionFailedItemDTO{ID: rowID, Code: "duplicate_username", Message: "student username duplicates row " + previousRowID})
			continue
		}
		usernamesInRequest[username] = rowID
		if err := validateStudentPassword(password, username, profile.FullName); err != nil {
			result.Skipped++
			result.FailedItems = append(result.FailedItems, BulkActionFailedItemDTO{ID: rowID, Code: "weak_password", Message: "password must be 8-128 characters and cannot contain username or student name"})
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

		row.Phone = profile.Phone
		authUser, err := s.createStudentAuthUser(ctx, tenantID, profile.FullName, username, password, row)
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
			TenantID:           tenantID,
			CenterID:           class.CenterID,
			ClassID:            class.ID,
			StudentCode:        profile.StudentCode,
			FullName:           profile.FullName,
			Username:           username,
			AuthUserID:         authUser.ID.Hex(),
			Email:              profile.Email,
			Gender:             profile.Gender,
			Birthday:           row.Birthday,
			Phone:              profile.Phone,
			Address:            profile.Address,
			ParentName:         profile.ParentName,
			ParentPhone:        profile.ParentPhone,
			ParentEmail:        profile.ParentEmail,
			ParentRelationship: profile.ParentRelationship,
			EnrollmentDate:     row.EnrollmentDate,
			Note:               profile.Note,
			Status:             statusActive,
			CreatedAt:          time.Now().UTC(),
			UpdatedAt:          time.Now().UTC(),
		}
		if err := s.repo.CreateStudent(ctx, student); err != nil {
			_ = s.authRepo.DeleteUserByID(ctx, authUser.ID)
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
	return value
}

func studentAccountEmail(username string) string {
	return normalizeStudentUsername(username) + "@" + studentAccountEmailDomain
}

func validateStudentUsername(username string) error {
	if !studentUsernamePattern.MatchString(username) {
		return errInvalidStudentAccountPayload
	}
	if strings.Contains(username, "..") || strings.Contains(username, "__") || strings.Contains(username, "--") {
		return errInvalidStudentAccountPayload
	}
	return nil
}

func validateStudentPassword(password, username, fullName string) error {
	password = strings.TrimSpace(password)
	if len(password) < studentPasswordMinLength || len(password) > studentPasswordMaxLength {
		return errInvalidStudentAccountPayload
	}
	lowerPassword := strings.ToLower(password)
	if username != "" && strings.Contains(lowerPassword, strings.ToLower(username)) {
		return errInvalidStudentAccountPayload
	}
	nameToken := compactCredentialToken(fullName)
	if len(nameToken) >= 4 && strings.Contains(lowerPassword, nameToken) {
		return errInvalidStudentAccountPayload
	}
	hasLetter := false
	hasDigit := false
	for _, r := range password {
		if unicode.IsLetter(r) {
			hasLetter = true
		}
		if unicode.IsDigit(r) {
			hasDigit = true
		}
		if unicode.IsControl(r) {
			return errInvalidStudentAccountPayload
		}
	}
	if !hasLetter || !hasDigit {
		return errInvalidStudentAccountPayload
	}
	return nil
}

func compactCredentialToken(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
