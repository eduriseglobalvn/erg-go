// Package notifications provides digest aggregation scheduling for notifications.
package notifications

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"erg.ninja/internal/modules/notifications/entities"
	"erg.ninja/internal/modules/notifications/repository"
	"erg.ninja/internal/modules/notifications/templates"
	"erg.ninja/pkg/logger"
)

// DigestService handles batching notifications into digest messages.
type DigestService struct {
	repo      *repository.Repository
	notifSvc  *Service
	renderer  *templates.Renderer
	log       *logger.Logger
	mu        sync.Mutex
	stopCh    chan struct{}
	stopFlush context.CancelFunc // cancels the flush goroutine's context
}

// DigestServiceOption configures the DigestService.
type DigestServiceOption func(*DigestService)

// WithDigestLogger sets the logger.
func WithDigestLogger(log *logger.Logger) DigestServiceOption {
	return func(s *DigestService) { s.log = log }
}

// NewDigestService creates a new digest service.
func NewDigestService(
	repo *repository.Repository,
	notifSvc *Service,
	opts ...DigestServiceOption,
) *DigestService {
	s := &DigestService{
		repo:     repo,
		notifSvc: notifSvc,
		renderer: templates.NewRenderer(),
		log:      logger.NoOp(),
		stopCh:   make(chan struct{}),
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// DigestConfig controls digest batching behavior.
type DigestConfig struct {
	// MaxItems is the maximum number of notifications to batch into one digest.
	MaxItems int
	// FlushInterval is how often to check for pending digests.
	FlushInterval time.Duration
	// DefaultFrequency is the default digest frequency.
	DefaultFrequency entities.DigestFrequency
}

const defaultMaxItems = 20

// Start begins the background digest flush goroutine.
func (s *DigestService) Start(ctx context.Context, cfg DigestConfig) {
	if cfg.MaxItems <= 0 {
		cfg.MaxItems = defaultMaxItems
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 5 * time.Minute
	}

	s.log.Info().
		Int("max_items", cfg.MaxItems).
		Dur("flush_interval", cfg.FlushInterval).
		Msg("digest: starting background flush")

	// Start the flush loop with a combined context: respects both the
	// caller's context (for cancellation) and the module's stop channel.
	flushCtx, flushCancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.stopFlush = flushCancel
	s.mu.Unlock()
	go s.runFlushLoop(flushCtx, cfg)
}

// Stop is called to signal the flush goroutine to exit.
func (s *DigestService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopFlush != nil {
		s.stopFlush()
	}
	select {
	case <-s.stopCh:
	default:
		close(s.stopCh)
	}
}

func (s *DigestService) runFlushLoop(ctx context.Context, cfg DigestConfig) {
	ticker := time.NewTicker(cfg.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.log.Info().Msg("digest: context canceled, stopping")
			return
		case <-s.stopCh:
			s.log.Info().Msg("digest: stop signal received")
			return
		case <-ticker.C:
			s.flushPendingDigests(context.Background(), cfg)
		}
	}
}

// flushPendingDigests collects pending notifications and sends them as digests.
func (s *DigestService) flushPendingDigests(ctx context.Context, cfg DigestConfig) {
	// Find all users with pending notifications eligible for digest.
	users, err := s.getUsersWithPendingDigests(ctx)
	if err != nil {
		s.log.ErrorContext(ctx).Err(err).Msg("digest: get users failed")
		return
	}

	for _, userID := range users {
		if err := s.sendUserDigest(ctx, userID, cfg.MaxItems); err != nil {
			s.log.ErrorContext(ctx).Err(err).Str("user_id", userID).Msg("digest: send user digest failed")
		}
	}
}

// getUsersWithPendingDigests returns user IDs with pending undigested notifications.
func (s *DigestService) getUsersWithPendingDigests(ctx context.Context) ([]string, error) {
	notifications, _, err := s.repo.List(ctx, repository.ListParams{
		Status: entities.StatusPending,
		Limit:  500,
	})
	if err != nil {
		return nil, err
	}

	userSet := make(map[string]struct{})
	for _, n := range notifications {
		if !n.Digested {
			userSet[n.UserID.Hex()] = struct{}{}
		}
	}

	users := make([]string, 0, len(userSet))
	for u := range userSet {
		users = append(users, u)
	}
	return users, nil
}

// sendUserDigest fetches pending notifications for a user and sends them as a digest.
func (s *DigestService) sendUserDigest(ctx context.Context, userID string, maxItems int) error {
	pref, err := s.repo.GetPreference(ctx, userID)
	if err != nil {
		return fmt.Errorf("digest: get preference: %w", err)
	}

	// If digest is disabled for this user, skip.
	freq := entities.DigestDaily
	if pref != nil {
		freq = pref.DigestFreq
	}
	if freq == entities.DigestNone {
		return nil
	}

	// Fetch pending notifications.
	pending, err := s.repo.GetPendingByUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("digest: get pending: %w", err)
	}
	if len(pending) == 0 {
		return nil
	}

	// Build digest items.
	var items []string
	var notifIDs []string
	for i, n := range pending {
		if i >= maxItems {
			break
		}
		item := s.formatDigestItem(n)
		items = append(items, item)
		notifIDs = append(notifIDs, n.ID.Hex())
	}

	objUserID, _ := bson.ObjectIDFromHex(userID)

	// Create digest record.
	digest := &entities.Digest{
		ID:        bson.NewObjectID(),
		UserID:    objUserID,
		Channel:   entities.ChannelEmail, // default to email for digests
		Recipient: "",
		Frequency: freq,
		Subject:   s.buildDigestSubject(freq, time.Now()),
		Items:     items,
		Count:     len(items),
		Status:    entities.StatusPending,
	}

	// Get recipient from preference.
	if pref != nil {
		digest.Recipient = pref.UserID.Hex() // Email uses userID as recipient in this system
	}

	if err := s.repo.CreateDigest(ctx, digest); err != nil {
		return fmt.Errorf("digest: create: %w", err)
	}

	// Render digest body.
	body, err := s.renderer.Render("trending_summary", templates.TemplateData{
		"date":          time.Now().Format("02/01/2006"),
		"items":         strings.Join(items, "\n"),
		"dashboard_url": "https://erg.ninja/dashboard",
	})
	if err != nil {
		s.log.WarnContext(ctx).Err(err).Msg("digest: render body failed")
		body = strings.Join(items, "\n")
	}

	// Send the digest notification.
	msg := &entities.Notification{
		ID:        bson.NewObjectID(),
		UserID:    objUserID,
		Channel:   entities.ChannelEmail,
		Recipient: digest.Recipient,
		Subject:   digest.Subject,
		Body:      body,
		Digested:  false,
		DigestID:  digest.ID.Hex(),
		Status:    entities.StatusPending,
	}

	if err := s.notifSvc.Send(ctx, msg); err != nil {
		return fmt.Errorf("digest: send: %w", err)
	}

	// Mark source notifications as digested.
	if err := s.repo.MarkDigested(ctx, notifIDs, digest.ID.Hex()); err != nil {
		s.log.ErrorContext(ctx).Err(err).Msg("digest: mark digested failed")
	}

	// Mark digest as sent.
	if err := s.repo.MarkDigestSent(ctx, digest.ID.Hex()); err != nil {
		s.log.ErrorContext(ctx).Err(err).Msg("digest: mark sent failed")
	}

	s.log.InfoContext(ctx).
		Str("user_id", userID).
		Int("count", len(items)).
		Str("digest_id", digest.ID.Hex()).
		Msg("digest: sent")

	return nil
}

// formatDigestItem formats a single notification for digest inclusion.
func (s *DigestService) formatDigestItem(n *entities.Notification) string {
	switch n.Channel {
	case entities.ChannelDiscord:
		return fmt.Sprintf("[%s] %s — %s", n.Channel, n.Subject, truncate(n.Body, 80))
	case entities.ChannelTelegram:
		return fmt.Sprintf("[%s] %s", n.Subject, truncate(n.Body, 80))
	default:
		return fmt.Sprintf("%s: %s", n.Subject, truncate(n.Body, 80))
	}
}

// buildDigestSubject creates a subject line for the digest.
func (s *DigestService) buildDigestSubject(freq entities.DigestFrequency, t time.Time) string {
	switch freq {
	case entities.DigestDaily:
		return fmt.Sprintf("📬 Daily Digest — %s", t.Format("02/01/2006"))
	case entities.DigestWeekly:
		_, week := t.ISOWeek()
		return fmt.Sprintf("📬 Weekly Digest — Tuần %d/%d", week, t.Year())
	case entities.DigestMonthly:
		return fmt.Sprintf("📬 Monthly Digest — %s %d", t.Month().String(), t.Year())
	default:
		return "📬 ERG Digest"
	}
}

// truncate truncates a string to at most n characters.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
