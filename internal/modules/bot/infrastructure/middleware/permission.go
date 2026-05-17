// Package middleware provides HTTP middleware and RBAC permission service for the bot-service.
package middleware

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"erg.ninja/internal/modules/bot/domain/entity"
	"erg.ninja/internal/modules/bot/infrastructure/cache"
	"erg.ninja/pkg/logger"
)

var (
	// ErrForbidden is returned when a user lacks the required permission.
	ErrForbidden = errors.New("permission: forbidden")
	// ErrUserNotFound is returned when the user cannot be found.
	ErrUserNotFound = errors.New("permission: user not found")
)

// UserClaims represents JWT claims extracted by the auth middleware.
type UserClaims struct {
	UserID      string   `json:"user_id"`
	Permissions []string `json:"permissions,omitempty"`
	Roles       []string `json:"roles,omitempty"`
	Email       string   `json:"email,omitempty"`
}

// PermissionService enforces RBAC for bot commands.
type PermissionService struct {
	coll         *mongo.Collection
	redis        cache.RedisCache
	log          *logger.Logger
	commandPerms map[string]models.PermissionLevel
	adminIDs     map[string]struct{} // immutable admin bypass set
}

// PermissionOption configures a PermissionService.
type PermissionOption func(*PermissionService)

// WithPermissionLogger sets the logger.
func WithPermissionLogger(log *logger.Logger) PermissionOption {
	return func(s *PermissionService) {
		s.log = log
	}
}

// WithAdminIDs configures the immutable admin bypass set.
func WithAdminIDs(adminIDs []string) PermissionOption {
	return func(s *PermissionService) {
		if len(adminIDs) == 0 {
			s.adminIDs = nil
			return
		}

		s.adminIDs = make(map[string]struct{}, len(adminIDs))
		for _, id := range adminIDs {
			if id == "" {
				continue
			}
			s.adminIDs[id] = struct{}{}
		}
	}
}

// NewPermissionService creates a PermissionService with the given MongoDB collection.
func NewPermissionService(coll *mongo.Collection, opts ...PermissionOption) *PermissionService {
	s := &PermissionService{
		coll:         coll,
		log:          logger.NoOp(),
		commandPerms: makeCommandPermissions(),
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// NewPermissionServiceWithRedis creates a PermissionService that caches user
// permissions in Redis to avoid repeated MongoDB lookups.
func NewPermissionServiceWithRedis(coll *mongo.Collection, redis cache.RedisCache, opts ...PermissionOption) *PermissionService {
	s := NewPermissionService(coll, opts...)
	s.redis = redis
	return s
}

// Check verifies that the user has the required permission to execute the given command.
func (s *PermissionService) Check(ctx context.Context, userID, command string) error {
	if s.isAdmin(ctx, userID) {
		return nil
	}

	required := s.GetCommandPermission(command)
	level, err := s.GetUserLevel(ctx, userID)
	if err != nil {
		return fmt.Errorf("permission check: %w", err)
	}

	if level < required {
		return fmt.Errorf("%w: requires level %d, you have level %d",
			ErrForbidden, required, level)
	}
	return nil
}

// GetUserLevel retrieves the user's permission level from MongoDB (cached in Redis).
func (s *PermissionService) GetUserLevel(ctx context.Context, userID string) (models.PermissionLevel, error) {
	// Try Redis cache first (TTL: 5 minutes).
	if s.redis != nil {
		cacheKey := fmt.Sprintf("bot:perm:%s", userID)
		val, err := s.redis.Get(ctx, cacheKey)
		if err == nil && val != "" {
			if lvl, parseErr := parsePermissionLevel(val); parseErr == nil && lvl != 0 {
				return lvl, nil
			}
		}
	}

	// Fetch from MongoDB.
	var conv models.BotConversation
	err := s.coll.FindOne(ctx, bson.M{"user_id": userID}).Decode(&conv)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			if s.redis != nil {
				_ = s.redis.Set(ctx, fmt.Sprintf("bot:perm:%s", userID), "1", 5*time.Minute)
			}
			return models.PermissionViewer, nil
		}
		return 0, fmt.Errorf("permission: fetch user level: %w", err)
	}

	level := models.PermissionLevel(conv.PermissionLevel)
	if level == 0 {
		level = models.PermissionViewer
	}

	if s.redis != nil {
		_ = s.redis.Set(ctx, fmt.Sprintf("bot:perm:%s", userID), fmt.Sprintf("%d", level), 5*time.Minute)
	}

	return level, nil
}

// SetUserLevel updates a user's permission level in MongoDB and invalidates the cache.
func (s *PermissionService) SetUserLevel(ctx context.Context, userID string, level models.PermissionLevel) error {
	filter := bson.M{"user_id": userID}
	update := bson.M{"$set": bson.M{"permission_level": level}}
	_, err := s.coll.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("permission: set user level: %w", err)
	}
	if s.redis != nil {
		_ = s.redis.Del(ctx, fmt.Sprintf("bot:perm:%s", userID))
	}
	return nil
}

// GetCommandPermission returns the minimum required permission level for a command.
func (s *PermissionService) GetCommandPermission(command string) models.PermissionLevel {
	if lvl, ok := s.commandPerms[command]; ok {
		return lvl
	}
	return models.PermissionViewer
}

// InvalidateCache removes the permission cache entry for a user.
func (s *PermissionService) InvalidateCache(ctx context.Context, userID string) error {
	if s.redis != nil {
		return s.redis.Del(ctx, fmt.Sprintf("bot:perm:%s", userID))
	}
	return nil
}

// isAdmin checks if a user is in the admin bypass list.
func (s *PermissionService) isAdmin(ctx context.Context, userID string) bool {
	_ = ctx
	if userID == "" || len(s.adminIDs) == 0 {
		return false
	}
	_, ok := s.adminIDs[userID]
	return ok
}

// parsePermissionLevel parses a permission level from a string value.
func parsePermissionLevel(s string) (models.PermissionLevel, error) {
	switch s {
	case "1":
		return models.PermissionViewer, nil
	case "2":
		return models.PermissionEditor, nil
	case "3":
		return models.PermissionCrawler, nil
	case "4":
		return models.PermissionModerator, nil
	case "5":
		return models.PermissionAdmin, nil
	default:
		return 0, fmt.Errorf("unknown permission level: %s", s)
	}
}

// makeCommandPermissions builds the static map of command → required permission.
func makeCommandPermissions() map[string]models.PermissionLevel {
	return map[string]models.PermissionLevel{
		// RSS commands.
		"rss add":     models.PermissionCrawler,
		"rss remove":  models.PermissionCrawler,
		"rss sync":    models.PermissionCrawler,
		"rss list":    models.PermissionViewer,
		"rss preview": models.PermissionViewer,
		// Crawl commands.
		"crawl start":   models.PermissionCrawler,
		"crawl stop":    models.PermissionCrawler,
		"crawl batch":   models.PermissionCrawler,
		"crawl status":  models.PermissionViewer,
		"crawl history": models.PermissionViewer,
		// Trending commands.
		"trending top":     models.PermissionViewer,
		"trending keyword": models.PermissionViewer,
		// Draft commands.
		"draft list":    models.PermissionEditor,
		"draft publish": models.PermissionEditor,
		"draft delete":  models.PermissionEditor,
		// Stats commands.
		"stats users":   models.PermissionModerator,
		"stats crawler": models.PermissionModerator,
		"stats queue":   models.PermissionModerator,
		"stats system":  models.PermissionAdmin,
		// System commands.
		"system health":  models.PermissionViewer,
		"system ping":    models.PermissionViewer,
		"system reload":  models.PermissionAdmin,
		"system version": models.PermissionViewer,
	}
}

// RequirePermission returns a middleware that checks JWT claims for a permission level.
func RequirePermission(level models.PermissionLevel) func(ctx context.Context, claims *UserClaims) error {
	return func(ctx context.Context, claims *UserClaims) error {
		if claims == nil {
			return ErrUserNotFound
		}
		for _, perm := range claims.Permissions {
			if perm == "*" || perm == level.String() {
				return nil
			}
		}
		return fmt.Errorf("%w: requires %s", ErrForbidden, level)
	}
}
