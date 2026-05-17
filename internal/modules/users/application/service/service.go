package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	entities "erg.ninja/internal/modules/auth/domain/entity"
	"erg.ninja/internal/modules/users/api/request"
	userrepo "erg.ninja/internal/modules/users/infrastructure/repository"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/security/password"
	"erg.ninja/pkg/storage"
)

// Service errors.
var (
	ErrInvalidCredentials = errors.New("users.service: invalid credentials")
	ErrWeakPassword       = errors.New("users.service: password too weak")
)

// Service handles business logic for the users module.
type Service struct {
	repo       *userrepo.Repository
	r2         *storage.R2Client
	log        *logger.Logger
	adminEmail string
	passParams password.Params
}

func New(repo *userrepo.Repository, r2 *storage.R2Client, log *logger.Logger, cfg *config.Config) *Service {
	adminEmail := defaultRootAdminEmail
	passParams := password.NormalizeParams(0, 0)
	if cfg != nil && strings.TrimSpace(cfg.Auth.AdminEmail) != "" {
		adminEmail = strings.TrimSpace(cfg.Auth.AdminEmail)
	}
	if cfg != nil {
		passParams = password.NormalizeParams(cfg.Auth.Argon2Memory, cfg.Auth.Argon2Iterations)
	}
	return &Service{repo: repo, r2: r2, log: log, adminEmail: strings.ToLower(adminEmail), passParams: passParams}
}

func (s *Service) GetProfile(ctx context.Context, userID string) (*entities.User, error) {
	id, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return nil, userrepo.ErrUserNotFound
	}
	user, err := s.repo.FindUserByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.repairRootAdmin(ctx, user); err != nil && s.log != nil {
		s.log.WarnContext(ctx).Err(err).Str("user_id", userID).Msg("users.service: failed to repair root admin profile")
	}
	return user, nil
}

const defaultRootAdminEmail = "admin@erg.edu.vn"

func (s *Service) isRootAdminEmail(email string) bool {
	email = strings.TrimSpace(email)
	if email == "" {
		return false
	}
	return strings.EqualFold(email, s.adminEmail) || strings.EqualFold(email, defaultRootAdminEmail)
}

func (s *Service) repairRootAdmin(ctx context.Context, user *entities.User) error {
	if user == nil || !s.isRootAdminEmail(user.Email) {
		return nil
	}

	updates := map[string]any{}
	if !strings.EqualFold(string(user.Status), string(entities.UserStatusActive)) {
		updates["status"] = string(entities.UserStatusActive)
		user.Status = entities.UserStatusActive
	}
	if strings.TrimSpace(user.Provider) == "" {
		updates["provider"] = "local"
		user.Provider = "local"
	}
	if strings.TrimSpace(user.AccountType) == "" {
		updates["account_type"] = "erg"
		user.AccountType = "erg"
	}
	if strings.TrimSpace(user.FullName) == "" {
		updates["full_name"] = "Super Administrator"
		user.FullName = "Super Administrator"
	}
	if !user.IsProfileCompleted {
		updates["is_profile_completed"] = true
		user.IsProfileCompleted = true
	}

	roles := ensureRootAdminRoles(user.Roles)
	if !sameStringSet(user.Roles, roles) {
		updates["roles"] = roles
		user.Roles = roles
	}
	if len(updates) == 0 {
		return nil
	}
	return s.repo.UpdateUserFields(ctx, user.ID, user.TenantID, updates)
}

func ensureRootAdminRoles(roles []string) []string {
	roles = uniqueRoleNames(roles)
	seen := make(map[string]bool, len(roles)+2)
	for _, role := range roles {
		seen[role] = true
	}
	if !seen["admin"] {
		roles = append(roles, "admin")
	}
	if !seen["SUPER_ADMIN"] {
		roles = append(roles, "SUPER_ADMIN")
	}
	return uniqueRoleNames(roles)
}

func sameStringSet(a, b []string) bool {
	a = uniqueRoleNames(a)
	b = uniqueRoleNames(b)
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]bool, len(a))
	for _, value := range a {
		seen[value] = true
	}
	for _, value := range b {
		if !seen[value] {
			return false
		}
	}
	return true
}

func uniqueRoleNames(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func (s *Service) UpdateProfile(ctx context.Context, userID, tenantID string, req *request.UpdateProfileRequest) error {
	id, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return userrepo.ErrUserNotFound
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
		return userrepo.ErrUserNotFound
	}
	user, err := s.repo.FindUserByIDAndTenant(ctx, id, tenantID)
	if err != nil {
		return err
	}
	passwordOK, _ := password.Verify(req.OldPassword, user.PasswordHash, s.passParams)
	if !passwordOK {
		return userrepo.ErrInvalidOldPassword
	}
	hash, err := password.Hash(req.NewPassword, s.passParams)
	if err != nil {
		return err
	}
	if err := s.repo.UpdatePasswordHash(ctx, id, tenantID, hash); err != nil {
		return err
	}
	return s.repo.RevokeAllUserSessionsWithReason(ctx, id, tenantID, "password_changed")
}

func (s *Service) Onboarding(ctx context.Context, userID, tenantID string, req *request.OnboardingRequest) error {
	id, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return userrepo.ErrUserNotFound
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
		return nil, userrepo.ErrUserNotFound
	}
	return s.repo.FindActiveSessions(ctx, id, tenantID)
}

func (s *Service) RevokeSession(ctx context.Context, userID, sessionID, tenantID string) error {
	id, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return userrepo.ErrUserNotFound
	}
	return s.repo.RevokeUserSessionWithReason(ctx, id, sessionID, tenantID, "logout")
}

func (s *Service) ValidateActiveSession(ctx context.Context, userID, sessionID, tenantID string) error {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(sessionID) == "" {
		return userrepo.ErrSessionNotFound
	}
	id, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return userrepo.ErrUserNotFound
	}
	session, err := s.repo.FindSessionByID(ctx, sessionID, tenantID)
	if err != nil {
		return err
	}
	if session.UserID != id || session.RevokedAt != nil || time.Now().UTC().After(session.ExpiresAt) {
		return userrepo.ErrSessionNotFound
	}
	return nil
}

func (s *Service) ListUsers(ctx context.Context, tenantID string, query *request.ListUsersQuery) ([]entities.User, int64, error) {
	params := userrepo.ListUsersParams{
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
		return nil, userrepo.ErrUserNotFound
	}
	return s.repo.FindUserByIDAndTenant(ctx, id, tenantID)
}

func (s *Service) UpdateUserStatus(ctx context.Context, userID, tenantID string, status string) error {
	id, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return userrepo.ErrUserNotFound
	}
	return s.repo.UpdateUserFields(ctx, id, tenantID, map[string]any{"status": status})
}

func (s *Service) AssignRoles(ctx context.Context, userID, tenantID string, roles []string) error {
	id, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return userrepo.ErrUserNotFound
	}
	return s.repo.UpdateUserFields(ctx, id, tenantID, map[string]any{"roles": roles})
}

func (s *Service) DeleteUser(ctx context.Context, userID, tenantID string) error {
	id, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return userrepo.ErrUserNotFound
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
