// Package bot implements the BOT module.
// It mirrors the NestJS module pattern: NewModule → Setup → RegisterRoutes → Stop.
package bot

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/modules/bot/commands"
	"erg.ninja/internal/modules/bot/handlers"
	"erg.ninja/internal/modules/bot/middleware"
	"erg.ninja/internal/modules/bot/models"
	"erg.ninja/internal/modules/bot/platform"
	"erg.ninja/internal/modules/bot/services"
	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/event"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/tenant"
)

// Deps holds the bot module's dependencies (mirrors NestJS module imports).
type Deps struct {
	Mongo             *database.MongoClient
	Redis             *cache.RedisClient
	Bus               *event.EventBus
	Log               *logger.Logger
	Cfg               *config.Config
	TenantMongoClient *tenant.TenantMongoClient
}

// Module is the bot module. It implements the module pattern used across all modules.
type Module struct {
	deps           Deps
	ctrl           *handlers.BotController
	discordCtrl    *handlers.DiscordWebhookHandler
	telegramCtrl   *handlers.TelegramWebhookHandler
	healthCtrl     *handlers.HealthHandler
	crawlerAdapter *CrawlerAdapter
	trendAdapter   *TrendingAdapter
	workflowSvc    *services.WorkflowEngine
}

// InjectAdapters sets cross-module service adapters (called before Setup).
func (m *Module) InjectAdapters(crawler *CrawlerAdapter, trend *TrendingAdapter) {
	m.crawlerAdapter = crawler
	m.trendAdapter = trend
}

// NewModule creates a new bot module with the given dependencies.
func NewModule(deps Deps) *Module {
	return &Module{deps: deps}
}

// Name implements plugin.Module.
func (m *Module) Name() string { return "bot" }

// Setup implements plugin.Module (như NestJS onModuleInit).
func (m *Module) Setup() error {
	if m.crawlerAdapter != nil {
		commands.SetCrawlerService(m.crawlerAdapter)
	}
	if m.trendAdapter != nil {
		commands.SetTrendingService(m.trendAdapter)
	}

	botConvColl := m.deps.Mongo.Collection(models.BotConversationCollection)
	linkedAccColl := m.deps.Mongo.Collection(models.BotLinkedAccountCollection)
	workflowColl := m.deps.Mongo.Collection(models.WorkflowExecutionCollection)

	permSvc := middleware.NewPermissionService(botConvColl,
		middleware.WithAdminIDs(m.deps.Cfg.Bot.AdminIDs),
		middleware.WithPermissionLogger(m.deps.Log),
	)

	convSvc := services.NewConversationService(botConvColl, m.deps.Redis,
		services.WithConversationLogger(m.deps.Log),
	)

	workflowSvc := services.NewWorkflowEngine(workflowColl,
		services.WithWorkflowEngineLogger(m.deps.Log),
	)
	m.workflowSvc = workflowSvc

	linkSvc := services.NewLinkService(linkedAccColl, m.deps.Redis,
		services.WithLinkLogger(m.deps.Log),
	)

	if m.crawlerAdapter != nil {
		workflowSvc.RegisterStepHandler("crawl", func(ctx context.Context, step models.WorkflowStep, data map[string]any) (map[string]any, string, error) {
			url, _ := data["url"].(string)
			jobID, err := m.crawlerAdapter.EnqueueURL(ctx, url, "workflow", 3)
			if err != nil {
				return nil, "", fmt.Errorf("workflow step crawl: %w", err)
			}
			return map[string]any{"job_id": jobID}, step.OnSuccess, nil
		})
	}
	if m.trendAdapter != nil {
		workflowSvc.RegisterStepHandler("trending", func(ctx context.Context, step models.WorkflowStep, data map[string]any) (map[string]any, string, error) {
			return nil, step.OnSuccess, nil
		})
	}
	workflowSvc.RegisterStepHandler("notification", func(ctx context.Context, step models.WorkflowStep, data map[string]any) (map[string]any, string, error) {
		return nil, step.OnSuccess, nil
	})
	workflowSvc.RegisterStepHandler("delay", func(ctx context.Context, step models.WorkflowStep, data map[string]any) (map[string]any, string, error) {
		duration := 5 * time.Second
		if d, ok := data["duration"].(string); ok {
			if parsed, err := time.ParseDuration(d); err == nil {
				duration = parsed
			}
		}
		time.Sleep(duration)
		return nil, step.OnSuccess, nil
	})
	workflowSvc.RegisterStepHandler("input", func(ctx context.Context, step models.WorkflowStep, data map[string]any) (map[string]any, string, error) {
		return data, step.OnSuccess, nil
	})

	commands.SetWorkflowService(workflowSvc)

	cmdHandler := services.NewCommandHandler(
		permSvc,
		convSvc,
		m.deps.Bus,
		workflowSvc,
		nil,
		nil,
		services.WithCommandHandlerLogger(m.deps.Log),
	)

	discordClient := platform.NewDiscordClient(m.deps.Cfg.Discord.Token,
		platform.WithDiscordLogger(m.deps.Log),
	)
	telegramClient := platform.NewTelegramClient(m.deps.Cfg.Telegram.BotToken,
		platform.WithTelegramLogger(m.deps.Log),
	)

	m.healthCtrl = handlers.NewHealthHandler(m.deps.Mongo, m.deps.Redis, m.deps.Log)
	m.discordCtrl = handlers.NewDiscordWebhookHandler(
		cmdHandler, linkSvc, convSvc,
		discordClient,
		m.deps.Cfg.Discord.WebhookSecret,
		m.deps.Log,
	)
	m.telegramCtrl = handlers.NewTelegramWebhookHandler(
		cmdHandler, linkSvc, convSvc,
		telegramClient,
		m.deps.Cfg.Telegram.BotToken,
		m.deps.Log,
	)
	m.ctrl = handlers.NewBotController(
		convSvc, linkSvc, workflowSvc,
		discordClient, telegramClient,
		botConvColl,
		m.deps.Log,
	)
	return nil
}

// RegisterRoutes mounts the bot module's HTTP routes onto the Gin router.
func (m *Module) RegisterRoutes(r *gin.Engine) {
	m.healthCtrl.RegisterRoutes(r)
	m.discordCtrl.RegisterRoutes(r)
	m.telegramCtrl.RegisterRoutes(r)
	api := r.Group("/api/bot")
	m.ctrl.RegisterRoutes(api)
}

// Stop implements plugin.Module — performs graceful shutdown.
func (m *Module) Stop(ctx context.Context) error {
	if m.deps.Log != nil {
		m.deps.Log.Info().Msg("bot: module stopped")
	}
	return nil
}
