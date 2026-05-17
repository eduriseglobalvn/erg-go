package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"gorm.io/gorm"

	entities "erg.ninja/internal/modules/auth/domain/entity"
	"erg.ninja/internal/persistence/postgrescore"
	"erg.ninja/pkg/database"
)

// Repository errors.
var (
	ErrUserNotFound       = errors.New("users.repository: user not found")
	ErrSessionNotFound    = errors.New("users.repository: session not found")
	ErrNoFieldsToUpdate   = errors.New("users.repository: no fields to update")
	ErrInvalidOldPassword = errors.New("users: invalid old password")
)

// Repository handles all user data access via PostgreSQL.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new users repository using PostgreSQL.
func NewRepository(gormClient *database.GORMPostgresClient) *Repository {
	var db *gorm.DB
	if gormClient != nil {
		db = gormClient.DB()
	}
	return &Repository{db: db}
}

func (r *Repository) ensureDB() error {
	if r.db == nil {
		return fmt.Errorf("users.repository: postgres client unavailable")
	}
	return nil
}

// FindUserByID returns a user by their ObjectID.
func (r *Repository) FindUserByID(ctx context.Context, id bson.ObjectID) (*entities.User, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var user postgrescore.AuthUser
	if err := r.db.WithContext(ctx).Where("id = ?", id.Hex()).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("users.repository.findUserByID: %w", err)
	}
	return r.mapUser(ctx, &user)
}

// FindUserByEmail returns a user by email (case-insensitive) and tenant.
func (r *Repository) FindUserByEmail(ctx context.Context, email string, tenantID string) (*entities.User, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var user postgrescore.AuthUser
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND email = ?", tenantID, normalizeEmail(email)).
		First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("users.repository.findUserByEmail: %w", err)
	}
	return r.mapUser(ctx, &user)
}

// UpdateUserFields updates only the specified fields on a user row.
func (r *Repository) UpdateUserFields(ctx context.Context, id bson.ObjectID, tenantID string, updates map[string]any) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if len(updates) == 0 {
		return ErrNoFieldsToUpdate
	}

	if roles, ok := updates["roles"]; ok {
		roleNames, _ := roles.([]string)
		delete(updates, "roles")
		if err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := r.ensureUserExistsTx(ctx, tx, id.Hex(), tenantID); err != nil {
				return err
			}
			if err := r.replaceUserRolesTx(ctx, tx, id.Hex(), roleNames); err != nil {
				return err
			}
			if len(updates) == 0 {
				return nil
			}
			updates["updated_at"] = time.Now().UTC()
			result := tx.WithContext(ctx).
				Model(&postgrescore.AuthUser{}).
				Where("id = ? AND tenant_id = ?", id.Hex(), tenantID).
				Updates(convertUserUpdates(updates))
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return ErrUserNotFound
			}
			return nil
		}); err != nil {
			if errors.Is(err, ErrUserNotFound) {
				return ErrUserNotFound
			}
			return fmt.Errorf("users.repository.updateUserFields: %w", err)
		}
		return nil
	}

	updates["updated_at"] = time.Now().UTC()
	result := r.db.WithContext(ctx).
		Model(&postgrescore.AuthUser{}).
		Where("id = ? AND tenant_id = ?", id.Hex(), tenantID).
		Updates(convertUserUpdates(updates))
	if result.Error != nil {
		return fmt.Errorf("users.repository.updateUserFields: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrUserNotFound
	}
	return nil
}

// UpdateUserStatus updates a user's status field by email.
func (r *Repository) UpdateUserStatusByEmail(ctx context.Context, email string, tenantID string, status entities.UserStatus) error {
	return r.UpdateUserStatus(ctx, email, tenantID, status)
}

// UpdateUserStatus updates a user's status field.
func (r *Repository) UpdateUserStatus(ctx context.Context, email string, tenantID string, status entities.UserStatus) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result := r.db.WithContext(ctx).
		Model(&postgrescore.AuthUser{}).
		Where("tenant_id = ? AND email = ?", tenantID, normalizeEmail(email)).
		Updates(map[string]any{
			"status":     string(status),
			"updated_at": time.Now().UTC(),
		})
	if result.Error != nil {
		return fmt.Errorf("users.repository.updateUserStatusByEmail: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrUserNotFound
	}
	return nil
}

// UpdatePasswordHash updates a user's password hash.
func (r *Repository) UpdatePasswordHash(ctx context.Context, id bson.ObjectID, tenantID string, hash string) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result := r.db.WithContext(ctx).
		Model(&postgrescore.AuthUser{}).
		Where("id = ? AND tenant_id = ?", id.Hex(), tenantID).
		Updates(map[string]any{
			"password_hash": hash,
			"updated_at":    time.Now().UTC(),
		})
	if result.Error != nil {
		return fmt.Errorf("users.repository.updatePasswordHash: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrUserNotFound
	}
	return nil
}

// DeleteUser soft-deletes a user by ID.
func (r *Repository) DeleteUser(ctx context.Context, id bson.ObjectID, tenantID string) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id.Hex(), tenantID).
		Delete(&postgrescore.AuthUser{})
	if result.Error != nil {
		return fmt.Errorf("users.repository.deleteUser: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrUserNotFound
	}
	return nil
}

// ListUsersParams holds query parameters for listing users.
type ListUsersParams struct {
	TenantID string
	Status   string
	Role     string
	Search   string
	Page     int
	Limit    int
}

// ListUsers returns paginated users with optional filters.
func (r *Repository) ListUsers(ctx context.Context, params ListUsersParams) ([]entities.User, int64, error) {
	if err := r.ensureDB(); err != nil {
		return nil, 0, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := r.db.WithContext(ctx).Model(&postgrescore.AuthUser{}).
		Where("tenant_id = ?", params.TenantID)

	if params.Status != "" {
		query = query.Where("status = ?", params.Status)
	}
	if params.Search != "" {
		search := "%" + strings.ToLower(params.Search) + "%"
		query = query.Where("LOWER(email) LIKE ? OR LOWER(full_name) LIKE ?", search, search)
	}
	if params.Role != "" {
		query = query.Joins("JOIN user_roles ON user_roles.user_id = users.id").
			Joins("JOIN roles ON roles.id = user_roles.role_id").
			Where("roles.name = ?", params.Role)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("users.repository.listUsers: count: %w", err)
	}

	var records []postgrescore.AuthUser
	skip := (params.Page - 1) * params.Limit
	if err := query.
		Order("created_at DESC").
		Offset(skip).
		Limit(params.Limit).
		Find(&records).Error; err != nil {
		return nil, 0, fmt.Errorf("users.repository.listUsers: find: %w", err)
	}

	roleMap, err := r.loadRoleMapForUsers(ctx, extractUserIDs(records))
	if err != nil {
		return nil, 0, fmt.Errorf("users.repository.listUsers.roles: %w", err)
	}

	users := make([]entities.User, 0, len(records))
	for i := range records {
		user, err := mapAuthUser(&records[i], roleMap[records[i].ID])
		if err != nil {
			return nil, 0, fmt.Errorf("users.repository.listUsers.map: %w", err)
		}
		users = append(users, *user)
	}
	return users, total, nil
}

// BulkUpdateStatus updates the status of multiple users.
func (r *Repository) BulkUpdateStatus(ctx context.Context, ids []bson.ObjectID, tenantID string, status entities.UserStatus) (int64, error) {
	if err := r.ensureDB(); err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result := r.db.WithContext(ctx).
		Model(&postgrescore.AuthUser{}).
		Where("id IN ? AND tenant_id = ?", objectIDHexes(ids), tenantID).
		Updates(map[string]any{
			"status":     string(status),
			"updated_at": time.Now().UTC(),
		})
	if result.Error != nil {
		return 0, fmt.Errorf("users.repository.bulkUpdateStatus: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// BulkDelete soft-deletes multiple users.
func (r *Repository) BulkDelete(ctx context.Context, ids []bson.ObjectID, tenantID string) (int64, error) {
	if err := r.ensureDB(); err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result := r.db.WithContext(ctx).
		Where("id IN ? AND tenant_id = ?", objectIDHexes(ids), tenantID).
		Delete(&postgrescore.AuthUser{})
	if result.Error != nil {
		return 0, fmt.Errorf("users.repository.bulkDelete: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// FindActiveSessions returns all active (non-revoked, non-expired) sessions for a user.
func (r *Repository) FindActiveSessions(ctx context.Context, userID bson.ObjectID, tenantID string) ([]entities.UserSession, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var records []postgrescore.AuthSession
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND tenant_id = ? AND revoked_at IS NULL AND expires_at > ?", userID.Hex(), tenantID, time.Now().UTC()).
		Order("created_at DESC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("users.repository.findActiveSessions: %w", err)
	}

	sessions := make([]entities.UserSession, 0, len(records))
	for i := range records {
		session, err := mapAuthSession(&records[i])
		if err != nil {
			return nil, fmt.Errorf("users.repository.findActiveSessions.map: %w", err)
		}
		sessions = append(sessions, *session)
	}
	return sessions, nil
}

// FindSessionByID returns a session by sessionID and tenant.
func (r *Repository) FindSessionByID(ctx context.Context, sessionID string, tenantID string) (*entities.UserSession, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var record postgrescore.AuthSession
	err := r.db.WithContext(ctx).
		Where("session_id = ? AND tenant_id = ?", sessionID, tenantID).
		First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("users.repository.findSessionByID: %w", err)
	}
	return mapAuthSession(&record)
}

// RevokeSession marks a session as revoked.
func (r *Repository) RevokeSession(ctx context.Context, sessionID string, tenantID string) error {
	return r.RevokeSessionWithReason(ctx, sessionID, tenantID, "logout")
}

// RevokeSessionWithReason marks a session as revoked and stores why it was revoked.
func (r *Repository) RevokeSessionWithReason(ctx context.Context, sessionID string, tenantID string, reason string) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	now := time.Now().UTC()
	result := r.db.WithContext(ctx).
		Model(&postgrescore.AuthSession{}).
		Where("session_id = ? AND tenant_id = ?", sessionID, tenantID).
		Updates(map[string]any{
			"revoked_at":     &now,
			"revoked_reason": strings.TrimSpace(reason),
			"updated_at":     now,
		})
	if result.Error != nil {
		return fmt.Errorf("users.repository.revokeSession: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrSessionNotFound
	}
	return nil
}

// RevokeUserSessionWithReason revokes a session only if it belongs to the user.
func (r *Repository) RevokeUserSessionWithReason(ctx context.Context, userID bson.ObjectID, sessionID string, tenantID string, reason string) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	now := time.Now().UTC()
	result := r.db.WithContext(ctx).
		Model(&postgrescore.AuthSession{}).
		Where("user_id = ? AND session_id = ? AND tenant_id = ?", userID.Hex(), sessionID, tenantID).
		Updates(map[string]any{
			"revoked_at":     &now,
			"revoked_reason": strings.TrimSpace(reason),
			"updated_at":     now,
		})
	if result.Error != nil {
		return fmt.Errorf("users.repository.revokeUserSession: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrSessionNotFound
	}
	return nil
}

// RevokeAllUserSessions revokes every active session for a user.
func (r *Repository) RevokeAllUserSessions(ctx context.Context, userID bson.ObjectID, tenantID string) error {
	return r.RevokeAllUserSessionsWithReason(ctx, userID, tenantID, "logout")
}

// RevokeAllUserSessionsWithReason revokes every active session for a user and stores why.
func (r *Repository) RevokeAllUserSessionsWithReason(ctx context.Context, userID bson.ObjectID, tenantID string, reason string) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	now := time.Now().UTC()
	if err := r.db.WithContext(ctx).
		Model(&postgrescore.AuthSession{}).
		Where("user_id = ? AND tenant_id = ? AND revoked_at IS NULL", userID.Hex(), tenantID).
		Updates(map[string]any{
			"revoked_at":     &now,
			"revoked_reason": strings.TrimSpace(reason),
			"updated_at":     now,
		}).Error; err != nil {
		return fmt.Errorf("users.repository.revokeAllUserSessions: %w", err)
	}
	return nil
}

// FindUserByIDAndTenant returns a user by ID + tenant.
func (r *Repository) FindUserByIDAndTenant(ctx context.Context, id bson.ObjectID, tenantID string) (*entities.User, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var record postgrescore.AuthUser
	if err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", id.Hex(), tenantID).
		First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("users.repository.findUserByIDAndTenant: %w", err)
	}
	return r.mapUser(ctx, &record)
}

func (r *Repository) mapUser(ctx context.Context, record *postgrescore.AuthUser) (*entities.User, error) {
	roles, err := r.loadRoleMapForUsers(ctx, []string{record.ID})
	if err != nil {
		return nil, err
	}
	return mapAuthUser(record, roles[record.ID])
}

func mapAuthUser(record *postgrescore.AuthUser, roles []string) (*entities.User, error) {
	id, err := bson.ObjectIDFromHex(record.ID)
	if err != nil {
		return nil, err
	}
	var socialLinks map[string]string
	if record.SocialLinksJSON != "" {
		if err := json.Unmarshal([]byte(record.SocialLinksJSON), &socialLinks); err != nil {
			socialLinks = nil
		}
	}
	return &entities.User{
		ID:                  id,
		Email:               record.Email,
		PasswordHash:        record.PasswordHash,
		FullName:            record.FullName,
		AvatarURL:           record.AvatarURL,
		Status:              entities.UserStatus(record.Status),
		Provider:            record.Provider,
		ProviderID:          record.ProviderID,
		AccountType:         record.AccountType,
		GoogleSub:           record.GoogleSub,
		GoogleEmail:         record.GoogleEmail,
		GoogleEmailVerified: record.GoogleEmailVerified,
		LastLoginProvider:   record.LastLoginProvider,
		Roles:               roles,
		TenantID:            record.TenantID,
		Phone:               record.Phone,
		Bio:                 record.Bio,
		Gender:              record.Gender,
		DateOfBirth:         record.DateOfBirth,
		Address:             record.Address,
		City:                record.City,
		District:            record.District,
		JobTitle:            record.JobTitle,
		Region:              record.Region,
		SocialLinks:         socialLinks,
		ExtendedProfile:     record.ExtendedProfile,
		IsProfileCompleted:  record.IsProfileCompleted,
		LastLoginAt:         record.LastLoginAt,
		LoginCount:          record.LoginCount,
		CreatedAt:           record.CreatedAt,
		UpdatedAt:           record.UpdatedAt,
	}, nil
}

func mapAuthSession(record *postgrescore.AuthSession) (*entities.UserSession, error) {
	id, err := bson.ObjectIDFromHex(record.ID)
	if err != nil {
		return nil, err
	}
	userID, err := bson.ObjectIDFromHex(record.UserID)
	if err != nil {
		return nil, err
	}
	return &entities.UserSession{
		ID:            id,
		UserID:        userID,
		SessionID:     record.SessionID,
		DeviceID:      record.DeviceID,
		DeviceName:    record.DeviceName,
		IPAddress:     record.IPAddress,
		UserAgent:     record.UserAgent,
		RefreshToken:  record.RefreshTokenHash,
		TenantID:      record.TenantID,
		ExpiresAt:     record.ExpiresAt,
		RevokedAt:     record.RevokedAt,
		RevokedReason: record.RevokedReason,
		LastActiveAt:  record.LastActiveAt,
		CreatedAt:     record.CreatedAt,
	}, nil
}

func (r *Repository) loadRoleMapForUsers(ctx context.Context, userIDs []string) (map[string][]string, error) {
	result := make(map[string][]string, len(userIDs))
	if len(userIDs) == 0 {
		return result, nil
	}
	type row struct {
		UserID string
		Name   string
	}
	var rows []row
	err := r.db.WithContext(ctx).
		Table("user_roles").
		Select("user_roles.user_id, roles.name").
		Joins("JOIN roles ON roles.id = user_roles.role_id").
		Where("user_roles.user_id IN ?", userIDs).
		Order("roles.name ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.Name == "" {
			continue
		}
		result[row.UserID] = append(result[row.UserID], row.Name)
	}
	return result, nil
}

func (r *Repository) replaceUserRolesTx(ctx context.Context, tx *gorm.DB, userID string, roleNames []string) error {
	if err := tx.WithContext(ctx).Where("user_id = ?", userID).Delete(&postgrescore.UserRole{}).Error; err != nil {
		return err
	}
	roleNames = uniqueStrings(roleNames)
	if len(roleNames) == 0 {
		return nil
	}
	var roles []postgrescore.ACRole
	if err := tx.WithContext(ctx).Where("name IN ?", roleNames).Find(&roles).Error; err != nil {
		return err
	}
	roleByName := make(map[string]string, len(roles))
	for _, role := range roles {
		roleByName[role.Name] = role.ID
	}
	now := time.Now().UTC()
	joins := make([]postgrescore.UserRole, 0, len(roleNames))
	for _, roleName := range roleNames {
		roleID := roleByName[roleName]
		if roleID == "" {
			continue
		}
		joins = append(joins, postgrescore.UserRole{
			UserID:    userID,
			RoleID:    roleID,
			CreatedAt: now,
		})
	}
	if len(joins) == 0 {
		return nil
	}
	return tx.WithContext(ctx).Create(&joins).Error
}

func (r *Repository) ensureUserExistsTx(ctx context.Context, tx *gorm.DB, userID, tenantID string) error {
	var count int64
	if err := tx.WithContext(ctx).Model(&postgrescore.AuthUser{}).
		Where("id = ? AND tenant_id = ?", userID, tenantID).
		Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return ErrUserNotFound
	}
	return nil
}

func convertUserUpdates(updates map[string]any) map[string]any {
	result := make(map[string]any, len(updates))
	for key, value := range updates {
		switch key {
		case "social_links":
			if raw, err := json.Marshal(value); err == nil {
				result["social_links_json"] = string(raw)
			}
		default:
			result[key] = value
		}
	}
	return result
}

func extractUserIDs(records []postgrescore.AuthUser) []string {
	ids := make([]string, 0, len(records))
	for _, record := range records {
		ids = append(ids, record.ID)
	}
	return ids
}

func objectIDHexes(ids []bson.ObjectID) []string {
	values := make([]string, 0, len(ids))
	for _, id := range ids {
		values = append(values, id.Hex())
	}
	return values
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func normalizeEmail(email string) string {
	return strings.TrimSpace(strings.ToLower(email))
}
