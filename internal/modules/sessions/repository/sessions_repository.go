// Package repository provides data access for the sessions module.
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"gorm.io/gorm"

	"erg.ninja/internal/modules/sessions/entities"
	"erg.ninja/internal/persistence/postgrescore"
	"erg.ninja/pkg/database"
)

// Repository errors.
var (
	ErrUserNotFound    = errors.New("sessions.repository: user not found")
	ErrSessionNotFound = errors.New("sessions.repository: session not found")
)

// Repository provides PostgreSQL data access for sessions.
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new sessions repository.
func NewRepository(gormClient *database.GORMPostgresClient) *Repository {
	var db *gorm.DB
	if gormClient != nil {
		db = gormClient.DB()
	}
	return &Repository{db: db}
}

func (r *Repository) ensureDB() error {
	if r.db == nil {
		return fmt.Errorf("sessions.repository: postgres client unavailable")
	}
	return nil
}

// GetUserByID retrieves a user row and its role names.
func (r *Repository) GetUserByID(ctx context.Context, tenantID, userID string) (*entities.User, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var record postgrescore.AuthUser
	err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", userID, tenantID).
		First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("sessions.repository.GetUserByID: %w", err)
	}

	roles, err := r.loadUserRoleNames(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("sessions.repository.GetUserByID.roles: %w", err)
	}
	return mapSessionUser(&record, roles)
}

// GetSessionByID retrieves an active session row.
func (r *Repository) GetSessionByID(ctx context.Context, tenantID, sessionID string) (*entities.Session, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var record postgrescore.AuthSession
	err := r.db.WithContext(ctx).
		Where("session_id = ? AND tenant_id = ? AND revoked_at IS NULL AND expires_at > ?", sessionID, tenantID, time.Now().UTC()).
		First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("sessions.repository.GetSessionByID: %w", err)
	}

	return mapSessionRecord(&record)
}

// UpdateSessionLastActive updates the last_active timestamp for a session.
func (r *Repository) UpdateSessionLastActive(ctx context.Context, sessionID string) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	now := time.Now().UTC()
	if err := r.db.WithContext(ctx).
		Model(&postgrescore.AuthSession{}).
		Where("session_id = ?", sessionID).
		Updates(map[string]any{
			"last_active_at": now,
			"updated_at":     now,
		}).Error; err != nil {
		return fmt.Errorf("sessions.repository.UpdateSessionLastActive: %w", err)
	}
	return nil
}

func (r *Repository) loadUserRoleNames(ctx context.Context, userID string) ([]string, error) {
	type row struct {
		Name string
	}
	var rows []row
	if err := r.db.WithContext(ctx).
		Table("roles").
		Select("roles.name").
		Joins("JOIN user_roles ON user_roles.role_id = roles.id").
		Where("user_roles.user_id = ?", userID).
		Order("roles.name ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	roles := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.Name != "" {
			roles = append(roles, row.Name)
		}
	}
	return roles, nil
}

func mapSessionUser(record *postgrescore.AuthUser, roles []string) (*entities.User, error) {
	id, err := bson.ObjectIDFromHex(record.ID)
	if err != nil {
		return nil, err
	}
	return &entities.User{
		ID:                 id,
		TenantID:           record.TenantID,
		Email:              record.Email,
		FullName:           record.FullName,
		AvatarURL:          record.AvatarURL,
		Provider:           record.Provider,
		AccountType:        record.AccountType,
		LastLoginProvider:  record.LastLoginProvider,
		Status:             record.Status,
		Roles:              roles,
		IsProfileCompleted: record.IsProfileCompleted,
	}, nil
}

func mapSessionRecord(record *postgrescore.AuthSession) (*entities.Session, error) {
	id, err := bson.ObjectIDFromHex(record.ID)
	if err != nil {
		return nil, err
	}
	userID, err := bson.ObjectIDFromHex(record.UserID)
	if err != nil {
		return nil, err
	}
	return &entities.Session{
		ID:         id,
		TenantID:   record.TenantID,
		UserID:     userID,
		SessionID:  record.SessionID,
		IPAddress:  record.IPAddress,
		UserAgent:  record.UserAgent,
		CreatedAt:  record.CreatedAt,
		LastActive: record.LastActiveAt,
		ExpiresAt:  record.ExpiresAt,
		IsActive:   record.RevokedAt == nil && record.ExpiresAt.After(time.Now().UTC()),
		RevokedAt:  record.RevokedAt,
	}, nil
}
