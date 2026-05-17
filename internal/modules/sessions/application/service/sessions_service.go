// Package service provides business logic for the sessions module.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	ac "erg.ninja/internal/modules/access_control/application/service"
	"erg.ninja/internal/modules/sessions/api/dto"
	entities "erg.ninja/internal/modules/sessions/domain/entity"
	"erg.ninja/internal/modules/sessions/infrastructure/repository"
	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/logger"
)

const (
	sessionCtxCacheTTL = 15 * time.Minute
	sessionCtxCacheKey = "session_ctx:%s:%s" // userID:sessionID
	appVersion         = "1.0.0"
)

// Deps holds the service's dependencies.
type Deps struct {
	Repo  *repository.Repository
	Redis *cache.RedisClient
	Log   *logger.Logger
	AC    *ac.Service
}

// Service provides session business logic.
type Service struct {
	deps Deps
}

// NewService creates a new sessions service.
func NewService(deps Deps) *Service {
	return &Service{deps: deps}
}

// GetCurrentSession returns the full session context for the authenticated user.
// It checks Redis cache first, falls back to MongoDB, then caches the result.
func (s *Service) GetCurrentSession(ctx context.Context, tenantID, userID, sessionID, ipAddress string) (*dto.SessionContextResponse, error) {
	// ── 1. Try Redis cache ──────────────────────────────────────────────────
	if s.deps.Redis != nil {
		cacheKey := fmt.Sprintf(sessionCtxCacheKey, userID, sessionID)
		cached, err := s.deps.Redis.Get(ctx, cacheKey)
		if err == nil && cached != "" {
			var cachedResp dto.SessionContextResponse
			if err := json.Unmarshal([]byte(cached), &cachedResp); err == nil {
				s.updateLastActiveAsync(ctx, tenantID, sessionID)
				return &cachedResp, nil
			}
		}
	}

	// ── 2. Fetch from MongoDB ───────────────────────────────────────────────
	user, err := s.deps.Repo.GetUserByID(ctx, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("service.GetCurrentSession: %w", err)
	}

	session, err := s.deps.Repo.GetSessionByID(ctx, tenantID, sessionID)
	if err != nil {
		return nil, fmt.Errorf("service.GetCurrentSession: %w", err)
	}

	// ── 3. Verify user status ─────────────────────────────────────────────────
	if err := validateUserStatus(user.Status); err != nil {
		return nil, err
	}

	// ── 4. Build response ─────────────────────────────────────────────────────
	roles := extractRoles(user)
	permissions, err := s.resolvePermissions(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("service.GetCurrentSession.resolvePermissions: %w", err)
	}

	resp := dto.NewSessionContextResponse(
		user.ID.Hex(),
		user.Email,
		user.FullName,
		user.AvatarURL,
		user.Status,
		user.Provider,
		user.AccountType,
		user.LastLoginProvider,
		user.IsProfileCompleted,
		roles,
		permissions,
		session.SessionID,
		sessionIP(session, ipAddress),
		session.LastActive,
		session.ExpiresAt,
		appVersion,
	)

	// ── 5. Cache in Redis ─────────────────────────────────────────────────────
	if s.deps.Redis != nil {
		cacheKey := fmt.Sprintf(sessionCtxCacheKey, userID, sessionID)
		if data, err := json.Marshal(resp); err == nil {
			_ = s.deps.Redis.Set(ctx, cacheKey, string(data), sessionCtxCacheTTL)
		}
	}

	// ── 6. Update last active (fire-and-forget) ─────────────────────────────
	s.updateLastActiveAsync(ctx, tenantID, sessionID)

	return &resp, nil
}

func (s *Service) updateLastActiveAsync(ctx context.Context, tenantID, sessionID string) {
	go func() {
		updateCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		_ = s.deps.Repo.UpdateSessionLastActive(updateCtx, tenantID, sessionID)
	}()
}

// validateUserStatus ensures the user is allowed to access the session.
func validateUserStatus(status string) error {
	switch status {
	case entities.UserStatusActive:
		return nil
	case entities.UserStatusPending:
		return fmt.Errorf("user account is pending")
	case entities.UserStatusBanned:
		return fmt.Errorf("user account is banned")
	case entities.UserStatusBlocked:
		return fmt.Errorf("user account is blocked")
	default:
		return fmt.Errorf("user account status is unknown: %s", status)
	}
}

func extractRoles(user *entities.User) []string {
	if len(user.Roles) == 0 {
		return []string{}
	}
	roles := make([]string, len(user.Roles))
	copy(roles, user.Roles)
	return roles
}

func (s *Service) resolvePermissions(ctx context.Context, user *entities.User) ([]string, error) {
	if user == nil {
		return []string{}, nil
	}
	if grantsAllPermissions(extractRoles(user)) {
		return []string{"*"}, nil
	}
	if s.deps.AC == nil {
		return extractRoles(user), nil
	}

	effective, err := s.deps.AC.GetEffectivePermissions(ctx, user.ID.Hex())
	if err != nil {
		return nil, err
	}
	if effective == nil || len(effective.EffectivePermissions) == 0 {
		return []string{}, nil
	}

	permissions := make([]string, len(effective.EffectivePermissions))
	copy(permissions, effective.EffectivePermissions)
	return permissions, nil
}

func grantsAllPermissions(roles []string) bool {
	for _, role := range roles {
		if strings.EqualFold(role, "admin") || strings.EqualFold(role, "SUPER_ADMIN") {
			return true
		}
	}
	return false
}

// sessionIP returns the stored IP if available, otherwise falls back to the request IP.
func sessionIP(session *entities.Session, requestIP string) string {
	if session.IPAddress != "" {
		return session.IPAddress
	}
	return requestIP
}

// IsNotFound returns true if err is a "not found" sentinel.
func IsNotFound(err error) bool {
	return errors.Is(err, repository.ErrUserNotFound) ||
		errors.Is(err, repository.ErrSessionNotFound)
}
