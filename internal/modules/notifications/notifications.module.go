// Package notifications implements the Notification module.
// It mirrors the NestJS module pattern: NewModule → Setup → RegisterRoutes → Stop.
// Handles multi-channel notifications: Discord, Telegram, WhatsApp, Email.
package notifications

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"erg.ninja/internal/modules/notifications/entities"
	"erg.ninja/internal/modules/notifications/providers"
	"erg.ninja/internal/modules/notifications/repository"
	"erg.ninja/pkg/auth"
	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/event"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/tenant"
)

// Deps holds the notifications module's dependencies (mirrors NestJS module imports).
type Deps struct {
	Mongo             *database.MongoClient
	Redis             *cache.RedisClient
	Bus               *event.EventBus
	Log               *logger.Logger
	Cfg               *config.Config
	JWTValidator      *auth.JWTValidator
	TenantMongoClient *tenant.TenantMongoClient
}

// Module is the notifications module. It implements the module pattern used across all modules.
type Module struct {
	deps     Deps
	svc      *Service
	digest   *DigestService
	consumer *EventConsumer
	ctrl     *Controller
}

// NewModule creates a new notifications module with the given dependencies.
func NewModule(deps Deps) *Module {
	return &Module{deps: deps}
}

// Name implements plugin.Module.
func (m *Module) Name() string { return "notification" }

// Setup implements plugin.Module (như NestJS onModuleInit).
func (m *Module) Setup() error {
	m.deps.Log.Info().Msg("notifications: module setup")

	// Repository.
	repo := repository.NewRepository(m.deps.Mongo,
		repository.WithRepositoryLogger(m.deps.Log),
	)

	// Core service.
	svc := NewService(repo, m.deps.Bus,
		WithNotificationLogger(m.deps.Log),
		WithMaxRetries(3),
	)

	// Providers — skip nil providers if credentials are not configured.
	if discord := MustDiscordProvider(m.deps.Cfg.Discord.WebhookURL, m.deps.Log); discord != nil {
		svc.RegisterProviders(discord)
	}
	if telegram := MustTelegramProvider(m.deps.Cfg.Telegram.BotToken, m.deps.Log); telegram != nil {
		svc.RegisterProviders(telegram)
	}
	if whatsapp := MustWhatsAppProvider(
		m.deps.Cfg.WhatsApp.PhoneID,
		m.deps.Cfg.WhatsApp.AccessToken,
		m.deps.Log,
	); whatsapp != nil {
		svc.RegisterProviders(whatsapp)
	}
	if email := MustEmailProvider(
		m.deps.Cfg.SMTP.Host,
		m.deps.Cfg.SMTP.Port,
		m.deps.Cfg.SMTP.Username,
		m.deps.Cfg.SMTP.Password,
		m.deps.Cfg.SMTP.From,
		m.deps.Log,
	); email != nil {
		svc.RegisterProviders(email)
	}

	m.svc = svc

	// Digest service.
	digestSvc := NewDigestService(repo, svc,
		WithDigestLogger(m.deps.Log),
	)
	digestCfg := DigestConfig{
		MaxItems:      20,
		FlushInterval: 5 * time.Minute,
	}
	// Start digest background flush (will be stopped in Stop()).
	ctx := context.Background()
	digestSvc.Start(ctx, digestCfg)
	m.digest = digestSvc

	// Event consumer.
	consumer := NewEventConsumer(svc, m.deps.Bus,
		WithEventConsumerLogger(m.deps.Log),
	)
	_ = consumer.Start(ctx)
	m.consumer = consumer

	// Controller.
	discord := svc.ProviderFor(entities.ChannelDiscord)
	telegram := svc.ProviderFor(entities.ChannelTelegram)
	whatsapp := svc.ProviderFor(entities.ChannelWhatsApp)
	email := svc.ProviderFor(entities.ChannelEmail)

	m.ctrl = NewController(
		svc, repo,
		toDiscord(discord),
		toTelegram(telegram),
		toWhatsApp(whatsapp),
		toEmail(email),
		m.deps.Log,
	)
	return nil
}

// RegisterRoutes mounts the notifications module's HTTP routes onto the chi router.
// Route prefix: /api/notifications, /api/channels (như NestJS @Controller decorators).
func (m *Module) RegisterRoutes(r *gin.Engine) {
	if m.ctrl != nil {
		m.ctrl.RegisterRoutes(r, m.deps.JWTValidator)
	}
}

// Stop implements plugin.Module — performs graceful shutdown.
func (m *Module) Stop(ctx context.Context) error {
	if m.deps.Log != nil {
		m.deps.Log.Info().Msg("notifications: module stopping")
	}
	if m.consumer != nil {
		m.consumer.Stop()
	}
	if m.digest != nil {
		m.digest.Stop()
	}
	if m.deps.Log != nil {
		m.deps.Log.Info().Msg("notifications: module stopped")
	}
	return nil
}

// ─── Type helpers for controller providers ─────────────────────────────────────

func toDiscord(p NotifierProvider) *providers.DiscordProvider {
	if dp, ok := p.(*providers.DiscordProvider); ok {
		return dp
	}
	return nil
}

func toTelegram(p NotifierProvider) *providers.TelegramProvider {
	if tp, ok := p.(*providers.TelegramProvider); ok {
		return tp
	}
	return nil
}

func toWhatsApp(p NotifierProvider) *providers.WhatsAppProvider {
	if wp, ok := p.(*providers.WhatsAppProvider); ok {
		return wp
	}
	return nil
}

func toEmail(p NotifierProvider) *providers.EmailProvider {
	if ep, ok := p.(*providers.EmailProvider); ok {
		return ep
	}
	return nil
}
