// Package notifications implements the core notification delivery service.
package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	entities "erg.ninja/internal/modules/notifications/domain/entity"
	"erg.ninja/internal/modules/notifications/infrastructure/providers"
	"erg.ninja/internal/modules/notifications/infrastructure/repository"
	"erg.ninja/internal/modules/notifications/infrastructure/templates"
	"erg.ninja/pkg/event"
	"erg.ninja/pkg/logger"
)

// SendEvent represents a request to send a notification from an external event.
type SendEvent struct {
	UserID    string
	Recipient string
	Subject   string
	Body      string
	Template  string
	Data      map[string]string
	Channel   entities.ChannelType
	Priority  string
}

var (
	ErrNotificationNotFound = errors.New("notification: not found")
	ErrNoProvider           = errors.New("notification: no provider for channel")
	ErrAlreadySent          = errors.New("notification: already sent")
	ErrCanceled             = errors.New("notification: canceled")
)

// NotifierProvider is the interface satisfied by all channel providers.
type NotifierProvider interface {
	Send(ctx context.Context, msg *entities.Notification) error
	Supports(channel entities.ChannelType) bool
	Name() string
	RateLimit() (int, time.Duration)
}

// Service handles notification sending, batching, canceling, and resending.
type Service struct {
	repo       *repository.Repository
	providers  []NotifierProvider
	renderer   *templates.Renderer
	log        *logger.Logger
	eventBus   *event.EventBus
	maxRetries int
}

// ServiceOption configures the Service.
type ServiceOption func(*Service)

// WithNotificationLogger sets the logger.
func WithNotificationLogger(log *logger.Logger) ServiceOption {
	return func(s *Service) { s.log = log }
}

// WithMaxRetries sets the maximum retry count for failed deliveries.
func WithMaxRetries(n int) ServiceOption {
	return func(s *Service) { s.maxRetries = n }
}

// NewService creates a new notification service with the given dependencies.
func NewService(
	repo *repository.Repository,
	eventBus *event.EventBus,
	opts ...ServiceOption,
) *Service {
	s := &Service{
		repo:       repo,
		providers:  make([]NotifierProvider, 0),
		renderer:   templates.NewRenderer(),
		log:        logger.NoOp(),
		eventBus:   eventBus,
		maxRetries: 3,
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// GetByID retrieves a notification by ID.
func (s *Service) GetByID(ctx context.Context, id string) (*entities.Notification, error) {
	return s.repo.GetByID(ctx, id)
}

// List returns a list of notifications.
func (s *Service) List(ctx context.Context, p repository.ListParams) ([]*entities.Notification, int64, error) {
	return s.repo.List(ctx, p)
}

// SendFromEvent sends a notification based on a SendEvent payload.
func (s *Service) SendFromEvent(ctx context.Context, evt SendEvent) error {
	msg := &entities.Notification{
		Channel:   evt.Channel,
		Recipient: evt.Recipient,
		Subject:   evt.Subject,
		Body:      evt.Body,
		Template:  evt.Template,
		Data:      evt.Data,
		Status:    entities.StatusPending,
	}

	if evt.UserID != "" {
		if oid, err := bson.ObjectIDFromHex(evt.UserID); err == nil {
			msg.UserID = oid
		}
	}

	return s.Send(ctx, msg)
}

// RegisterProviders registers a list of providers at once.
func (s *Service) RegisterProviders(providersList ...NotifierProvider) {
	s.providers = append(s.providers, providersList...)
}

// ProviderFor returns the provider for the given channel, or nil.
func (s *Service) ProviderFor(channel entities.ChannelType) NotifierProvider {
	for _, p := range s.providers {
		if p.Supports(channel) {
			return p
		}
	}
	return nil
}

// Send sends a single notification.
func (s *Service) Send(ctx context.Context, msg *entities.Notification) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Render template if specified.
	if msg.Template != "" {
		data := templates.TemplateData(msg.Data)
		body, err := s.renderer.Render(msg.Template, data)
		if err != nil {
			return fmt.Errorf("notification.Send: render: %w", err)
		}
		msg.Body = body
	}

	p := s.ProviderFor(msg.Channel)
	if p == nil {
		return ErrNoProvider
	}

	err := p.Send(ctx, msg)
	if err != nil {
		s.log.Error().Err(err).Str("id", msg.ID.Hex()).Msg("notification delivery failed")
		_ = s.repo.UpdateStatus(ctx, msg.ID.Hex(), entities.StatusFailed, err.Error())
		return err
	}

	_ = s.repo.UpdateStatus(ctx, msg.ID.Hex(), entities.StatusSent, "")
	return nil
}

// SendMany sends a batch of notifications concurrently.
func (s *Service) SendMany(ctx context.Context, msgs []*entities.Notification) {
	var wg sync.WaitGroup
	for _, msg := range msgs {
		wg.Add(1)
		go func(m *entities.Notification) {
			defer wg.Done()
			_ = s.Send(ctx, m)
		}(msg)
	}
	wg.Wait()
}

// ProcessDigest aggregates pending notifications for a user into a single digest.
func (s *Service) ProcessDigest(ctx context.Context, userID string) error {
	pending, err := s.repo.GetPendingByUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("notification.ProcessDigest: %w", err)
	}
	if len(pending) == 0 {
		return nil
	}

	objUserID, _ := bson.ObjectIDFromHex(userID)
	digest := &entities.Digest{
		UserID:    objUserID,
		Channel:   entities.ChannelEmail,
		CreatedAt: time.Now().UTC(),
		Status:    entities.StatusPending,
	}
	if err := s.repo.CreateDigest(ctx, digest); err != nil {
		return fmt.Errorf("notification.ProcessDigest create: %w", err)
	}

	var ids []string
	for _, p := range pending {
		ids = append(ids, p.ID.Hex())
	}
	if err := s.repo.MarkDigested(ctx, ids, digest.ID.Hex()); err != nil {
		return fmt.Errorf("notification.ProcessDigest mark: %w", err)
	}

	return nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func MustDiscordProvider(token string, log *logger.Logger) *providers.DiscordProvider {
	if token == "" {
		return nil
	}
	return providers.NewDiscordProvider(
		providers.WithDiscordLogger(log),
	)
}

func MustTelegramProvider(botToken string, log *logger.Logger) *providers.TelegramProvider {
	if botToken == "" {
		return nil
	}
	return providers.NewTelegramProvider(botToken,
		providers.WithTelegramLogger(log),
	)
}

func MustWhatsAppProvider(phoneID, token string, log *logger.Logger) *providers.WhatsAppProvider {
	if phoneID == "" || token == "" {
		return nil
	}
	return providers.NewWhatsAppProvider(
		providers.WithWhatsAppLogger(log),
		providers.WithWhatsAppCredentials(phoneID, token),
	)
}

func MustEmailProvider(host string, port int, username, password, from string, log *logger.Logger) *providers.EmailProvider {
	if host == "" || from == "" {
		return nil
	}
	return providers.NewEmailProvider(
		providers.WithEmailLogger(log),
		providers.WithSMTPCredentials(host, port, username, password),
		providers.WithFromAddress(from),
	)
}
