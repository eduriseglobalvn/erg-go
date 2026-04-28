package users

import (
	"context"
	"crypto/sha256"
	"errors"

	"go.mongodb.org/mongo-driver/v2/bson"

	"erg.ninja/internal/modules/auth/entities"
	"erg.ninja/internal/modules/users/dto/request"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/storage"
)

// Service errors.
var (
	ErrInvalidCredentials = errors.New("users.service: invalid credentials")
	ErrWeakPassword       = errors.New("users.service: password too weak")
)

// Service handles business logic for the users module.
type Service struct {
	repo *Repository
	r2   *storage.R2Client
	log  *logger.Logger
}

func newService(repo *Repository, r2 *storage.R2Client, log *logger.Logger) *Service {
	return &Service{repo: repo, r2: r2, log: log}
}

func (s *Service) GetProfile(ctx context.Context, userID string) (*entities.User, error) {
	id, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return nil, ErrUserNotFound
	}
	return s.repo.FindUserByID(ctx, id)
}

func (s *Service) UpdateProfile(ctx context.Context, userID, tenantID string, req *request.UpdateProfileRequest) error {
	id, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return ErrUserNotFound
	}
	updates := map[string]any{}
	if req.FullName != nil {
		updates["full_name"] = *req.FullName
	}
	if req.Phone != nil {
		updates["phone"] = *req.Phone
	}
	if req.Bio != nil {
		updates["bio"] = *req.Bio
	}
	if req.Gender != nil {
		updates["gender"] = *req.Gender
	}
	if req.DateOfBirth != nil {
		updates["date_of_birth"] = *req.DateOfBirth
	}
	if req.Address != nil {
		updates["address"] = *req.Address
	}
	if req.City != nil {
		updates["city"] = *req.City
	}
	if req.District != nil {
		updates["district"] = *req.District
	}
	if req.JobTitle != nil {
		updates["job_title"] = *req.JobTitle
	}
	if req.Region != nil {
		updates["region"] = *req.Region
	}
	if req.SocialLinks != nil {
		updates["social_links"] = *req.SocialLinks
	}
	if req.AvatarURL != nil {
		updates["avatar_url"] = *req.AvatarURL
	}
	return s.repo.UpdateUserFields(ctx, id, tenantID, updates)
}

func (s *Service) ChangePassword(ctx context.Context, userID, tenantID string, req *request.ChangePasswordRequest) error {
	id, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return ErrUserNotFound
	}
	user, err := s.repo.FindUserByIDAndTenant(ctx, id, tenantID)
	if err != nil {
		return err
	}
	if !verifyPasswordHash(user.PasswordHash, req.OldPassword) {
		return ErrInvalidOldPassword
	}
	hash := hashPassword(req.NewPassword)
	if err := s.repo.UpdatePasswordHash(ctx, id, tenantID, hash); err != nil {
		return err
	}
	return s.repo.RevokeAllUserSessions(ctx, id, tenantID)
}

func (s *Service) Onboarding(ctx context.Context, userID, tenantID string, req *request.OnboardingRequest) error {
	id, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return ErrUserNotFound
	}
	updates := map[string]any{
		"full_name":            req.FullName,
		"phone":                req.Phone,
		"bio":                  req.Bio,
		"gender":               req.Gender,
		"date_of_birth":        req.DateOfBirth,
		"address":              req.Address,
		"city":                 req.City,
		"district":             req.District,
		"job_title":            req.JobTitle,
		"region":               req.Region,
		"is_profile_completed": true,
	}
	return s.repo.UpdateUserFields(ctx, id, tenantID, updates)
}

func (s *Service) GetSessions(ctx context.Context, userID, tenantID string) ([]entities.UserSession, error) {
	id, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return nil, ErrUserNotFound
	}
	return s.repo.FindActiveSessions(ctx, id, tenantID)
}

func (s *Service) RevokeSession(ctx context.Context, sessionID, tenantID string) error {
	return s.repo.RevokeSession(ctx, sessionID, tenantID)
}

func (s *Service) ListUsers(ctx context.Context, tenantID string, query *request.ListUsersQuery) ([]entities.User, int64, error) {
	params := ListUsersParams{
		TenantID: tenantID,
		Status:   query.Status,
		Role:     query.Role,
		Search:   query.Search,
		Page:     query.Page,
		Limit:    query.Limit,
	}
	if params.Limit == 0 {
		params.Limit = 20
	}
	if params.Page == 0 {
		params.Page = 1
	}
	return s.repo.ListUsers(ctx, params)
}

func (s *Service) GetUserDetail(ctx context.Context, userID, tenantID string) (*entities.User, error) {
	id, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return nil, ErrUserNotFound
	}
	return s.repo.FindUserByIDAndTenant(ctx, id, tenantID)
}

func (s *Service) UpdateUserStatus(ctx context.Context, userID, tenantID string, status string) error {
	id, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return ErrUserNotFound
	}
	return s.repo.UpdateUserFields(ctx, id, tenantID, map[string]any{"status": status})
}

func (s *Service) AssignRoles(ctx context.Context, userID, tenantID string, roles []string) error {
	id, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return ErrUserNotFound
	}
	return s.repo.UpdateUserFields(ctx, id, tenantID, map[string]any{"roles": roles})
}

func (s *Service) DeleteUser(ctx context.Context, userID, tenantID string) error {
	id, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return ErrUserNotFound
	}
	return s.repo.DeleteUser(ctx, id, tenantID)
}

func (s *Service) BulkUpdateStatus(ctx context.Context, userIDs []string, tenantID string, status string) (int64, error) {
	var ids []bson.ObjectID
	for _, uid := range userIDs {
		if id, err := bson.ObjectIDFromHex(uid); err == nil {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return 0, nil
	}
	return s.repo.BulkUpdateStatus(ctx, ids, tenantID, entities.UserStatus(status))
}

func (s *Service) BulkDelete(ctx context.Context, userIDs []string, tenantID string) (int64, error) {
	var ids []bson.ObjectID
	for _, uid := range userIDs {
		if id, err := bson.ObjectIDFromHex(uid); err == nil {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return 0, nil
	}
	return s.repo.BulkDelete(ctx, ids, tenantID)
}

func hashPassword(password string) string {
	h := sha256.New()
	h.Write([]byte(password))
	return "sha256:" + string(h.Sum(nil))
}

func verifyPasswordHash(hash, password string) bool {
	return hash == hashPassword(password)
}
