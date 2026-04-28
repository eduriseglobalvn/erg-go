package services

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"erg.ninja/internal/modules/bot/cache"
	"erg.ninja/internal/modules/bot/models"
	"erg.ninja/pkg/logger"
)

const (
	linkCodeCharset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // omit confusing chars: 0/O, 1/I
	linkCodeLength  = 6
	linkCodeTTL     = 5 * time.Minute
	linkCodePrefix  = "bot:link:"
)

// LinkService manages platform account linking via 6-char verification codes.
// The code is stored in Redis with a 5-minute TTL; once verified it is persisted
// to MongoDB as a permanent BotLinkedAccount.
type LinkService struct {
	coll   *mongo.Collection
	redis  cache.RedisCache
	log    *logger.Logger
	issuer string // bot display name for messages
}

// LinkServiceOption configures a LinkService.
type LinkServiceOption func(*LinkService)

// WithLinkLogger sets the logger.
func WithLinkLogger(log *logger.Logger) LinkServiceOption {
	return func(s *LinkService) {
		s.log = log
	}
}

// NewLinkService creates a LinkService backed by MongoDB and Redis.
func NewLinkService(coll *mongo.Collection, redis cache.RedisCache, opts ...LinkServiceOption) *LinkService {
	s := &LinkService{
		coll:   coll,
		redis:  redis,
		log:    logger.NoOp(),
		issuer: "ERG Bot",
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// CreateLinkCode generates a 6-char verification code, stores it in Redis
// (5-min TTL), and returns the code.
func (s *LinkService) CreateLinkCode(ctx context.Context, userID string) (string, error) {
	code := generateCode(linkCodeLength)
	key := linkCodePrefix + code

	// Store user_id under the code key in Redis.
	if err := s.redis.Set(ctx, key, userID, linkCodeTTL); err != nil {
		return "", fmt.Errorf("link: create code: %w", err)
	}

	s.log.InfoContext(ctx).Str("user_id", userID).Str("code", code).Msg("link code created")
	return code, nil
}

// VerifyLinkCode looks up the user ID for a code, deletes the code from Redis,
// and upserts the BotLinkedAccount in MongoDB. Returns the internal user ID on success.
func (s *LinkService) VerifyLinkCode(ctx context.Context, platform, platformUserID, code string) (string, error) {
	key := linkCodePrefix + code
	userID, err := s.redis.Get(ctx, key)
	if err != nil {
		return "", fmt.Errorf("link: get code: %w", err)
	}
	if userID == "" {
		return "", errors.New("link: code not found or expired")
	}

	// Delete the code immediately after reading (one-time use).
	if err := s.redis.Del(ctx, key); err != nil {
		s.log.WarnContext(ctx).Err(err).Str("code", code).Msg("link: failed to delete verification code after use")
	}

	// Upsert BotLinkedAccount in MongoDB.
	filter := bson.M{
		"platform":         platform,
		"platform_user_id": platformUserID,
	}
	now := time.Now()
	update := bson.M{
		"$set": bson.M{
			"platform_user_id": platformUserID,
			"internal_user_id": userID,
			"verified_at":      now,
			"linked_at":        now,
			"unlinked_at":      nil,
		},
	}
	opts := options.UpdateOne().SetUpsert(true)
	_, err = s.coll.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return "", fmt.Errorf("link: upsert account: %w", err)
	}

	s.log.InfoContext(ctx).
		Str("platform", platform).
		Str("platform_user_id", platformUserID).
		Str("internal_user_id", userID).
		Msg("account linked")

	return userID, nil
}

// VerifyLinkCodeByUserID verifies the link code using the internal user ID
// (for platforms that don't provide platformUserID upfront).
func (s *LinkService) VerifyLinkCodeByUserID(ctx context.Context, platform, internalUserID, code string) (string, error) {
	key := linkCodePrefix + code
	storedUserID, err := s.redis.Get(ctx, key)
	if err != nil {
		return "", fmt.Errorf("link: get code: %w", err)
	}
	if storedUserID == "" {
		return "", errors.New("link: code not found or expired")
	}
	if storedUserID != internalUserID {
		return "", errors.New("link: code does not match user")
	}

	if err := s.redis.Del(ctx, key); err != nil {
		s.log.WarnContext(ctx).Err(err).Str("code", code).Msg("link: failed to delete verification code during verify-by-user-id")
	}

	// Look up platform user ID from existing linked accounts.
	var account models.BotLinkedAccount
	err = s.coll.FindOne(ctx, bson.M{
		"internal_user_id": internalUserID,
		"platform":         platform,
		"unlinked_at":      bson.M{"$exists": false},
	}).Decode(&account)
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return "", fmt.Errorf("link: find account: %w", err)
	}

	now := time.Now()
	filter := bson.M{"internal_user_id": internalUserID, "platform": platform}
	update := bson.M{
		"$set": bson.M{
			"verified_at": now,
			"linked_at":   now,
			"unlinked_at": nil,
		},
		"$setOnInsert": bson.M{
			"platform_user_id": account.PlatformUserID,
		},
	}
	opts := options.UpdateOne().SetUpsert(true)
	_, err = s.coll.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return "", fmt.Errorf("link: verify account: %w", err)
	}

	s.log.InfoContext(ctx).
		Str("platform", platform).
		Str("internal_user_id", internalUserID).
		Msg("account verified")

	return internalUserID, nil
}

// GetLinkedAccounts returns all active linked accounts for a user.
func (s *LinkService) GetLinkedAccounts(ctx context.Context, internalUserID string) ([]*models.BotLinkedAccount, error) {
	cursor, err := s.coll.Find(ctx, bson.M{
		"internal_user_id": internalUserID,
		"unlinked_at":      bson.M{"$exists": false},
	})
	if err != nil {
		return nil, fmt.Errorf("link: list accounts: %w", err)
	}
	defer cursor.Close(ctx)

	var accounts []*models.BotLinkedAccount
	if err := cursor.All(ctx, &accounts); err != nil {
		return nil, fmt.Errorf("link: decode accounts: %w", err)
	}
	return accounts, nil
}

// GetAccountByPlatformUser returns a linked account by platform and platform user ID.
func (s *LinkService) GetAccountByPlatformUser(ctx context.Context, platform, platformUserID string) (*models.BotLinkedAccount, error) {
	var account models.BotLinkedAccount
	err := s.coll.FindOne(ctx, bson.M{
		"platform":         platform,
		"platform_user_id": platformUserID,
		"unlinked_at":      bson.M{"$exists": false},
	}).Decode(&account)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, fmt.Errorf("link: get account: %w", err)
	}
	return &account, nil
}

// UnlinkAccount soft-deletes a linked account.
func (s *LinkService) UnlinkAccount(ctx context.Context, platform, platformUserID string) error {
	now := time.Now()
	filter := bson.M{
		"platform":         platform,
		"platform_user_id": platformUserID,
	}
	update := bson.M{"$set": bson.M{"unlinked_at": now}}
	_, err := s.coll.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("link: unlink: %w", err)
	}
	s.log.InfoContext(ctx).
		Str("platform", platform).
		Str("platform_user_id", platformUserID).
		Msg("account unlinked")
	return nil
}

// generateCode generates a cryptographically secure alphanumeric code.
func generateCode(length int) string {
	charset := linkCodeCharset
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		now := time.Now().UnixNano()
		for i := range b {
			b[i] = byte(now >> (uint(i%8) * 8))
		}
	}
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return string(b)
}

// FormatLinkMessage returns the formatted message shown to the user with the code.
func (s *LinkService) FormatLinkMessage(code string) string {
	return fmt.Sprintf(`🔗 Liên kết tài khoản ERG của bạn!

Nhập mã xác minh này trong chat với bot:
%s

Mã có hiệu lực trong **5 phút**.

Nếu bạn không yêu cầu liên kết, hãy bỏ qua tin nhắn này.`, code)
}

// FormatSuccessMessage returns the confirmation message after successful linking.
func (s *LinkService) FormatSuccessMessage(platform, displayName string) string {
	return fmt.Sprintf("✅ Tài khoản %s của bạn đã được liên kết thành công!\n\nBạn có thể sử dụng các lệnh bot ngay bây giờ.", strings.Title(platform))
}
