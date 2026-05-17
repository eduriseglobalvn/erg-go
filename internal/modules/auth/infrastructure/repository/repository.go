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
	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/database"
	passwordpkg "erg.ninja/pkg/security/password"
)

// Repository errors.
var (
	ErrUserNotFound    = errors.New("auth.repository: user not found")
	ErrSessionNotFound = errors.New("auth.repository: session not found")
	ErrPinNotFound     = errors.New("auth.repository: pin not found")
	ErrPinExpired      = errors.New("auth.repository: pin expired")
	ErrPinAlreadyUsed  = errors.New("auth.repository: pin already used")
	ErrInvalidPassword = errors.New("auth.repository: invalid password")
	ErrDuplicateEmail  = errors.New("auth.repository: duplicate email")
)

// Repo holds all auth data access dependencies.
type Repo struct {
	db    *gorm.DB
	redis *cache.RedisClient
}

// RepoDeps holds dependencies for NewRepo.
type RepoDeps struct {
	GORM  *database.GORMPostgresClient
	Redis *cache.RedisClient
}

// NewRepo creates a new auth repository backed by PostgreSQL.
func NewRepo(deps RepoDeps) *Repo {
	var db *gorm.DB
	if deps.GORM != nil {
		db = deps.GORM.DB()
	}
	return &Repo{db: db, redis: deps.Redis}
}

func (r *Repo) ensureDB() error {
	if r.db == nil {
		return fmt.Errorf("auth.repository: postgres client unavailable")
	}
	return nil
}

// ─── User ───────────────────────────────────────────────────────────────────

// FindUserByEmail returns a user by email (case-insensitive) and tenant.
func (r *Repo) FindUserByEmail(ctx context.Context, email string, tenantID string) (*entities.User, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var record postgrescore.AuthUser
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND email = ?", tenantID, normalizeEmail(email)).
		First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("auth.repository.findUserByEmail: %w", err)
	}
	roles, err := r.loadUserRoleNames(ctx, record.ID)
	if err != nil {
		return nil, fmt.Errorf("auth.repository.findUserByEmail.roles: %w", err)
	}
	return mapAuthUserRecord(&record, roles)
}

// FindUserByGoogleSub returns a user by Google subject and tenant.
func (r *Repo) FindUserByGoogleSub(ctx context.Context, googleSub string, tenantID string) (*entities.User, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var record postgrescore.AuthUser
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND google_sub = ?", tenantID, strings.TrimSpace(googleSub)).
		First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("auth.repository.findUserByGoogleSub: %w", err)
	}
	roles, err := r.loadUserRoleNames(ctx, record.ID)
	if err != nil {
		return nil, fmt.Errorf("auth.repository.findUserByGoogleSub.roles: %w", err)
	}
	return mapAuthUserRecord(&record, roles)
}

// FindUserByID returns a user by their ObjectID.
func (r *Repo) FindUserByID(ctx context.Context, id bson.ObjectID) (*entities.User, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var record postgrescore.AuthUser
	err := r.db.WithContext(ctx).Where("id = ?", id.Hex()).First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("auth.repository.findUserByID: %w", err)
	}
	roles, err := r.loadUserRoleNames(ctx, record.ID)
	if err != nil {
		return nil, fmt.Errorf("auth.repository.findUserByID.roles: %w", err)
	}
	return mapAuthUserRecord(&record, roles)
}

// CreateUser inserts a new user. Returns ErrDuplicateEmail if the email already exists.
func (r *Repo) CreateUser(ctx context.Context, user *entities.User) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	now := time.Now().UTC()
	if user.ID.IsZero() {
		user.ID = bson.NewObjectID()
	}
	user.CreatedAt = now
	user.UpdatedAt = now

	record, err := newAuthUserRecord(user)
	if err != nil {
		return fmt.Errorf("auth.repository.createUser.record: %w", err)
	}

	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(record).Error; err != nil {
			if isDuplicateKey(err) {
				return ErrDuplicateEmail
			}
			return err
		}
		if err := r.replaceUserRolesTx(ctx, tx, record.ID, user.Roles); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrDuplicateEmail) {
			return ErrDuplicateEmail
		}
		return fmt.Errorf("auth.repository.createUser: %w", err)
	}
	return nil
}

// DeleteUserByID permanently removes a user created during a failed provisioning flow.
func (r *Repo) DeleteUserByID(ctx context.Context, id bson.ObjectID) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	if id.IsZero() {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		userID := id.Hex()
		if err := tx.Where("user_id = ?", userID).Delete(&postgrescore.UserRole{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("id = ?", userID).Delete(&postgrescore.AuthUser{}).Error; err != nil {
			return err
		}
		return nil
	})
}

// UpdateUserStatus updates a user's status field.
func (r *Repo) UpdateUserStatus(ctx context.Context, email string, tenantID string, status entities.UserStatus) error {
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
		return fmt.Errorf("auth.repository.updateUserStatus: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrUserNotFound
	}
	return nil
}

// UpdatePasswordHash updates a user's password hash.
func (r *Repo) UpdatePasswordHash(ctx context.Context, email string, tenantID string, hash string) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result := r.db.WithContext(ctx).
		Model(&postgrescore.AuthUser{}).
		Where("tenant_id = ? AND email = ?", tenantID, normalizeEmail(email)).
		Updates(map[string]any{
			"password_hash": hash,
			"updated_at":    time.Now().UTC(),
		})
	if result.Error != nil {
		return fmt.Errorf("auth.repository.updatePasswordHash: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrUserNotFound
	}
	return nil
}

// UpdateUserRoles replaces the stored role names for a user.
func (r *Repo) UpdateUserRoles(ctx context.Context, id bson.ObjectID, roles []string) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&postgrescore.AuthUser{}).Where("id = ?", id.Hex()).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return ErrUserNotFound
		}
		return r.replaceUserRolesTx(ctx, tx, id.Hex(), roles)
	})
}

// UpdateUserIdentityFields updates identity-related columns for an existing user.
func (r *Repo) UpdateUserIdentityFields(ctx context.Context, id bson.ObjectID, updates map[string]any) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	if len(updates) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	updates["updated_at"] = time.Now().UTC()
	result := r.db.WithContext(ctx).
		Model(&postgrescore.AuthUser{}).
		Where("id = ?", id.Hex()).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("auth.repository.updateUserIdentityFields: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrUserNotFound
	}
	return nil
}

// TouchSuccessfulLogin stores last login metadata without mutating existing auth fields.
func (r *Repo) TouchSuccessfulLogin(ctx context.Context, id bson.ObjectID, provider string) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	now := time.Now().UTC()
	result := r.db.WithContext(ctx).
		Model(&postgrescore.AuthUser{}).
		Where("id = ?", id.Hex()).
		Updates(map[string]any{
			"last_login_at":       now,
			"last_login_provider": strings.TrimSpace(provider),
			"login_count":         gorm.Expr("login_count + 1"),
			"updated_at":          now,
		})
	if result.Error != nil {
		return fmt.Errorf("auth.repository.touchSuccessfulLogin: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrUserNotFound
	}
	return nil
}

// HashPassword hashes a password using Argon2id with default server parameters.
func HashPassword(password string) (string, error) {
	return passwordpkg.Hash(password, passwordpkg.NormalizeParams(0, 0))
}

// VerifyPassword compares a password against a stored hash.
func VerifyPassword(password, hash string) bool {
	ok, _ := passwordpkg.Verify(password, hash, passwordpkg.NormalizeParams(0, 0))
	return ok
}

// ─── Session ─────────────────────────────────────────────────────────────────

// sessionCacheKey returns the Redis cache key for a lightweight active-session marker.
func sessionCacheKey(userID, sessionID string) string {
	return fmt.Sprintf("auth_session_active:%s:%s", userID, sessionID)
}

// sessionContextCacheKey matches the sessions module cache key. Auth must clear
// it when revoking sessions so stale session contexts cannot survive logout.
func sessionContextCacheKey(userID, sessionID string) string {
	return fmt.Sprintf("session_ctx:%s:%s", userID, sessionID)
}

// cacheSession stores a session in Redis with 15-min TTL.
func (r *Repo) cacheSession(ctx context.Context, session *entities.UserSession) error {
	if r.redis == nil {
		return nil
	}
	key := sessionCacheKey(session.UserID.Hex(), session.SessionID)
	ttl := time.Until(session.ExpiresAt)
	if ttl <= 0 {
		return nil
	}
	return r.redis.Set(ctx, key, "active", ttl)
}

// InvalidateSessionCache removes a session from Redis.
func (r *Repo) InvalidateSessionCache(ctx context.Context, userID, sessionID string) error {
	if r.redis == nil {
		return nil
	}
	return r.redis.Del(ctx, sessionCacheKey(userID, sessionID), sessionContextCacheKey(userID, sessionID))
}

// InvalidateSessionCaches removes multiple session cache entries in one Redis round trip.
func (r *Repo) InvalidateSessionCaches(ctx context.Context, userID string, sessionIDs []string) error {
	if r.redis == nil || len(sessionIDs) == 0 {
		return nil
	}
	keys := make([]string, 0, len(sessionIDs))
	for _, sessionID := range sessionIDs {
		sessionID = strings.TrimSpace(sessionID)
		if sessionID == "" {
			continue
		}
		keys = append(keys, sessionCacheKey(userID, sessionID), sessionContextCacheKey(userID, sessionID))
	}
	if len(keys) == 0 {
		return nil
	}
	return r.redis.Del(ctx, keys...)
}

// CreateSession inserts a new user session and caches it in Redis.
func (r *Repo) CreateSession(ctx context.Context, session *entities.UserSession) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	now := time.Now().UTC()
	session.ID = bson.NewObjectID()
	session.CreatedAt = now
	session.LastActiveAt = now

	record := &postgrescore.AuthSession{
		ID:               session.ID.Hex(),
		UserID:           session.UserID.Hex(),
		SessionID:        session.SessionID,
		DeviceID:         session.DeviceID,
		DeviceName:       session.DeviceName,
		IPAddress:        session.IPAddress,
		UserAgent:        session.UserAgent,
		RefreshTokenHash: session.RefreshToken,
		TenantID:         session.TenantID,
		LastActiveAt:     now,
		ExpiresAt:        session.ExpiresAt.UTC(),
		RevokedAt:        session.RevokedAt,
		RevokedReason:    session.RevokedReason,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		return fmt.Errorf("auth.repository.createSession: %w", err)
	}
	if r.redis != nil {
		sessionCopy := *session
		go func() {
			cacheCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
			defer cancel()
			_ = r.cacheSession(cacheCtx, &sessionCopy)
		}()
	}
	return nil
}

// FindSessionByID returns a session by its sessionID.
func (r *Repo) FindSessionByID(ctx context.Context, sessionID string, tenantID string) (*entities.UserSession, error) {
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
		return nil, fmt.Errorf("auth.repository.findSessionByID: %w", err)
	}
	return mapAuthSessionRecord(&record)
}

// RevokeSession marks a session as revoked by setting RevokedAt.
func (r *Repo) RevokeSession(ctx context.Context, sessionID string, tenantID string) error {
	return r.RevokeSessionWithReason(ctx, sessionID, tenantID, "logout")
}

// RevokeSessionWithReason marks a session as revoked and stores why it was revoked.
func (r *Repo) RevokeSessionWithReason(ctx context.Context, sessionID string, tenantID string, reason string) error {
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
			"last_active_at": now,
			"updated_at":     now,
		})
	if result.Error != nil {
		return fmt.Errorf("auth.repository.revokeSession: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrSessionNotFound
	}
	return nil
}

// RevokeAllUserSessions revokes every non-revoked session for a user.
func (r *Repo) RevokeAllUserSessions(ctx context.Context, userID bson.ObjectID, tenantID string) error {
	return r.RevokeAllUserSessionsWithReason(ctx, userID, tenantID, "logout")
}

// RevokeAllUserSessionsWithReason revokes every non-revoked session for a user and stores why.
func (r *Repo) RevokeAllUserSessionsWithReason(ctx context.Context, userID bson.ObjectID, tenantID string, reason string) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	now := time.Now().UTC()
	query := r.db.WithContext(ctx).
		Model(&postgrescore.AuthSession{}).
		Where("user_id = ? AND revoked_at IS NULL", userID.Hex())
	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}
	if err := query.Updates(map[string]any{
		"revoked_at":     &now,
		"revoked_reason": strings.TrimSpace(reason),
		"updated_at":     now,
	}).Error; err != nil {
		return fmt.Errorf("auth.repository.revokeAllUserSessions: %w", err)
	}
	return nil
}

// RevokeActiveUserSessionsWithReason revokes only currently usable sessions.
func (r *Repo) RevokeActiveUserSessionsWithReason(ctx context.Context, userID bson.ObjectID, tenantID string, reason string) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	now := time.Now().UTC()
	query := r.db.WithContext(ctx).
		Model(&postgrescore.AuthSession{}).
		Where("user_id = ? AND revoked_at IS NULL AND expires_at > ?", userID.Hex(), now)
	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}
	if err := query.Updates(map[string]any{
		"revoked_at":     &now,
		"revoked_reason": strings.TrimSpace(reason),
		"updated_at":     now,
	}).Error; err != nil {
		return fmt.Errorf("auth.repository.revokeActiveUserSessions: %w", err)
	}
	return nil
}

// RevokeOtherActiveUserSessionsWithReason revokes active sessions except the newly issued one.
func (r *Repo) RevokeOtherActiveUserSessionsWithReason(ctx context.Context, userID bson.ObjectID, tenantID string, keepSessionID string, reason string) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	now := time.Now().UTC()
	query := r.db.WithContext(ctx).
		Model(&postgrescore.AuthSession{}).
		Where("user_id = ? AND revoked_at IS NULL AND expires_at > ?", userID.Hex(), now)
	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}
	if strings.TrimSpace(keepSessionID) != "" {
		query = query.Where("session_id <> ?", strings.TrimSpace(keepSessionID))
	}
	if err := query.Updates(map[string]any{
		"revoked_at":     &now,
		"revoked_reason": strings.TrimSpace(reason),
		"updated_at":     now,
	}).Error; err != nil {
		return fmt.Errorf("auth.repository.revokeOtherActiveUserSessions: %w", err)
	}
	return nil
}

// FindActiveSessions returns all active (non-revoked, non-expired) sessions for a user.
func (r *Repo) FindActiveSessions(ctx context.Context, userID bson.ObjectID, tenantID string) ([]entities.UserSession, error) {
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
		return nil, fmt.Errorf("auth.repository.findActiveSessions: %w", err)
	}

	sessions := make([]entities.UserSession, 0, len(records))
	for i := range records {
		session, err := mapAuthSessionRecord(&records[i])
		if err != nil {
			return nil, fmt.Errorf("auth.repository.findActiveSessions.map: %w", err)
		}
		sessions = append(sessions, *session)
	}
	return sessions, nil
}

// IsSessionCached checks if a session exists in Redis cache.
func (r *Repo) IsSessionCached(ctx context.Context, userID, sessionID string) (bool, error) {
	if r.redis == nil {
		return false, nil
	}
	exists, err := r.redis.Exists(ctx, sessionCacheKey(userID, sessionID))
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

// MarkRefreshTokenUsed stores the refresh token hash to detect reuse (rotation detection).
// Stored in Redis with TTL = refresh token TTL.
func (r *Repo) MarkRefreshTokenUsed(ctx context.Context, userID, tokenHash string, ttl time.Duration) error {
	if r.redis == nil {
		return nil
	}
	key := fmt.Sprintf("rt_used:%s:%s", userID, tokenHash)
	return r.redis.Set(ctx, key, "1", ttl)
}

// IsRefreshTokenUsed checks if a refresh token has already been used (token reuse attack detection).
func (r *Repo) IsRefreshTokenUsed(ctx context.Context, userID, tokenHash string) (bool, error) {
	if r.redis == nil {
		return false, nil
	}
	exists, err := r.redis.Exists(ctx, fmt.Sprintf("rt_used:%s:%s", userID, tokenHash))
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

// ─── PIN ─────────────────────────────────────────────────────────────────────

// pinTTL is the default expiry duration for PIN codes.
const pinTTL = 15 * time.Minute

// CreatePin inserts a new PIN code.
func (r *Repo) CreatePin(ctx context.Context, pin *entities.PinCode) error {
	if err := r.ensureDB(); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	now := time.Now().UTC()
	pin.ID = bson.NewObjectID()
	pin.CreatedAt = now
	pin.ExpiresAt = now.Add(pinTTL)

	record := &postgrescore.AuthPin{
		ID:        pin.ID.Hex(),
		Email:     normalizeEmail(pin.Email),
		Code:      pin.Code,
		Purpose:   string(pin.Purpose),
		ExpiresAt: pin.ExpiresAt.UTC(),
		UsedAt:    pin.UsedAt,
		CreatedAt: now,
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("email = ? AND purpose = ? AND used_at IS NULL", record.Email, record.Purpose).
			Delete(&postgrescore.AuthPin{}).Error; err != nil {
			return err
		}
		return tx.Create(record).Error
	})
	if err != nil {
		return fmt.Errorf("auth.repository.createPin: %w", err)
	}
	return nil
}

// ValidateAndConsumePin finds an unused, non-expired PIN and marks it as used atomically.
func (r *Repo) ValidateAndConsumePin(ctx context.Context, email string, code string, purpose entities.PinPurpose) (*entities.PinCode, error) {
	if err := r.ensureDB(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var pinRecord postgrescore.AuthPin
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where(
			"email = ? AND code = ? AND purpose = ? AND used_at IS NULL AND expires_at > ?",
			normalizeEmail(email), code, string(purpose), time.Now().UTC(),
		).First(&pinRecord).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			return err
		}
		now := time.Now().UTC()
		if err := tx.Model(&postgrescore.AuthPin{}).
			Where("id = ? AND used_at IS NULL", pinRecord.ID).
			Update("used_at", &now).Error; err != nil {
			return err
		}
		pinRecord.UsedAt = &now
		return nil
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.pinLookupFailure(ctx, email, code, purpose)
		}
		return nil, fmt.Errorf("auth.repository.validateAndConsumePin: %w", err)
	}
	return mapAuthPinRecord(&pinRecord)
}

func (r *Repo) pinLookupFailure(ctx context.Context, email string, code string, purpose entities.PinPurpose) (*entities.PinCode, error) {
	var existing postgrescore.AuthPin
	err := r.db.WithContext(ctx).
		Where("email = ? AND code = ? AND purpose = ?", normalizeEmail(email), code, string(purpose)).
		First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPinNotFound
		}
		return nil, fmt.Errorf("auth.repository.validateAndConsumePin.lookup: %w", err)
	}
	if existing.UsedAt != nil {
		return nil, ErrPinAlreadyUsed
	}
	if time.Now().UTC().After(existing.ExpiresAt) {
		return nil, ErrPinExpired
	}
	return nil, ErrPinNotFound
}

// CleanupExpiredPins removes all expired PINs.
func (r *Repo) CleanupExpiredPins(ctx context.Context) (int64, error) {
	if err := r.ensureDB(); err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result := r.db.WithContext(ctx).
		Where("expires_at < ? OR used_at IS NOT NULL", time.Now().UTC()).
		Delete(&postgrescore.AuthPin{})
	if result.Error != nil {
		return 0, fmt.Errorf("auth.repository.cleanupExpiredPins: %w", result.Error)
	}
	return result.RowsAffected, nil
}

func (r *Repo) loadUserRoleNames(ctx context.Context, userID string) ([]string, error) {
	type row struct {
		Name string
	}
	var rows []row
	err := r.db.WithContext(ctx).
		Table("roles").
		Select("roles.name").
		Joins("JOIN user_roles ON user_roles.role_id = roles.id").
		Where("user_roles.user_id = ?", userID).
		Order("roles.name ASC").
		Scan(&rows).Error
	if err != nil {
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

func (r *Repo) replaceUserRolesTx(ctx context.Context, tx *gorm.DB, userID string, roles []string) error {
	if err := tx.Where("user_id = ?", userID).Delete(&postgrescore.UserRole{}).Error; err != nil {
		return err
	}
	roles = uniqueStrings(roles)
	if len(roles) == 0 {
		return nil
	}
	var roleRecords []postgrescore.ACRole
	if err := tx.WithContext(ctx).Where("name IN ?", roles).Find(&roleRecords).Error; err != nil {
		return err
	}
	roleIDByName := make(map[string]string, len(roleRecords))
	for _, role := range roleRecords {
		roleIDByName[role.Name] = role.ID
	}
	now := time.Now().UTC()
	joins := make([]postgrescore.UserRole, 0, len(roles))
	for _, roleName := range roles {
		roleID := roleIDByName[roleName]
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
	return tx.Create(&joins).Error
}

func newAuthUserRecord(user *entities.User) (*postgrescore.AuthUser, error) {
	socialLinksJSON := ""
	if len(user.SocialLinks) > 0 {
		raw, err := json.Marshal(user.SocialLinks)
		if err != nil {
			return nil, err
		}
		socialLinksJSON = string(raw)
	}
	return &postgrescore.AuthUser{
		ID:                  user.ID.Hex(),
		TenantID:            user.TenantID,
		Email:               normalizeEmail(user.Email),
		PasswordHash:        user.PasswordHash,
		FullName:            user.FullName,
		AvatarURL:           user.AvatarURL,
		Status:              string(user.Status),
		Provider:            user.Provider,
		ProviderID:          user.ProviderID,
		AccountType:         defaultAccountType(user.AccountType, user.Provider, user.GoogleSub),
		GoogleSub:           strings.TrimSpace(user.GoogleSub),
		GoogleEmail:         normalizeEmail(defaultString(user.GoogleEmail, user.Email)),
		GoogleEmailVerified: user.GoogleEmailVerified,
		LastLoginProvider:   defaultString(user.LastLoginProvider, user.Provider),
		Phone:               user.Phone,
		Bio:                 user.Bio,
		Gender:              user.Gender,
		DateOfBirth:         user.DateOfBirth,
		Address:             user.Address,
		City:                user.City,
		District:            user.District,
		JobTitle:            user.JobTitle,
		Region:              user.Region,
		SocialLinksJSON:     socialLinksJSON,
		ExtendedProfile:     user.ExtendedProfile,
		IsProfileCompleted:  user.IsProfileCompleted,
		LastLoginAt:         user.LastLoginAt,
		LoginCount:          user.LoginCount,
		CreatedAt:           user.CreatedAt.UTC(),
		UpdatedAt:           user.UpdatedAt.UTC(),
	}, nil
}

func mapAuthUserRecord(record *postgrescore.AuthUser, roles []string) (*entities.User, error) {
	if record == nil {
		return nil, nil
	}
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

func mapAuthSessionRecord(record *postgrescore.AuthSession) (*entities.UserSession, error) {
	if record == nil {
		return nil, nil
	}
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

func mapAuthPinRecord(record *postgrescore.AuthPin) (*entities.PinCode, error) {
	if record == nil {
		return nil, nil
	}
	id, err := bson.ObjectIDFromHex(record.ID)
	if err != nil {
		return nil, err
	}
	return &entities.PinCode{
		ID:        id,
		Email:     record.Email,
		Code:      record.Code,
		Purpose:   entities.PinPurpose(record.Purpose),
		ExpiresAt: record.ExpiresAt,
		UsedAt:    record.UsedAt,
		CreatedAt: record.CreatedAt,
	}, nil
}

func normalizeEmail(email string) string {
	return strings.TrimSpace(strings.ToLower(email))
}

func defaultString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func defaultAccountType(current, provider, googleSub string) string {
	current = strings.TrimSpace(strings.ToLower(current))
	if current != "" {
		return current
	}
	provider = strings.TrimSpace(strings.ToLower(provider))
	if strings.TrimSpace(googleSub) != "" || provider == "google" {
		return "google"
	}
	return "erg"
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

func isDuplicateKey(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate key") || strings.Contains(msg, "unique constraint")
}
