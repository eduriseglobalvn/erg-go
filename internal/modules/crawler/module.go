// Package crawler implements the Crawler module.
// It mirrors the NestJS module pattern: NewModule → Setup → RegisterRoutes → Stop.
// Handles RSS feed polling, HTML scraping, quality gate, deduplication, and SEO tagging.
package crawler

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	crawlercontroller "erg.ninja/internal/modules/crawler/api/controller"
	crawlerservice "erg.ninja/internal/modules/crawler/application/service"
	entities "erg.ninja/internal/modules/crawler/domain/entity"
	"erg.ninja/internal/modules/crawler/infrastructure/repository"
	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/event"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/queue"
	"erg.ninja/pkg/scraper"
	"erg.ninja/pkg/tenant"
)

// Deps holds the crawler module's dependencies (mirrors NestJS module imports).
type Deps struct {
	Mongo             *database.MongoClient
	GORMClient        *database.GORMPostgresClient
	Redis             *cache.RedisClient
	Bus               *event.EventBus
	Log               *logger.Logger
	Cfg               *config.Config
	Queue             *queue.AsynqClient
	TenantMongoClient *tenant.TenantMongoClient
}

// Module is the crawler module. It implements the module pattern used across all modules.
type Module struct {
	deps    Deps
	svc     *crawlerservice.Service
	repo    *repository.Repository
	ctrl    *crawlercontroller.Controller
	rssCtrl *crawlercontroller.RSSController
	blCtrl  *crawlercontroller.BlacklistController
}

type Service = crawlerservice.Service
type SSEHub = crawlerservice.SSEHub

// NewModule creates a new crawler module with the given dependencies.
func NewModule(deps Deps) *Module {
	return &Module{deps: deps}
}

// Name implements plugin.Module.
func (m *Module) Name() string { return "crawler" }

// Setup implements plugin.Module (như NestJS onModuleInit).
func (m *Module) Setup() error {
	m.deps.Log.Info().Msg("crawler: module setup")

	// Repository.
	m.repo = repository.NewRepository(m.deps.Mongo,
		repository.WithCrawlerRepositoryPostgres(m.deps.GORMClient),
		repository.WithCrawlerRepositoryLogger(m.deps.Log),
	)

	// Scraper (fetcher).
	fetcher := scraper.NewFetcher(m.deps.Cfg.Scraper,
		scraper.WithFetcherLogger(m.deps.Log),
	)

	// Service.
	m.svc = crawlerservice.NewService(m.repo, fetcher,
		m.deps.Log,
		m.deps.Bus,
		crawlerservice.WithCrawlerLogger(m.deps.Log),
	)

	// Controllers.
	m.ctrl = crawlercontroller.NewController(m.svc, m.repo, m.deps.Log)
	m.rssCtrl = crawlercontroller.NewRSSController(m.repo, m.deps.Queue, m.deps.Log)
	m.blCtrl = crawlercontroller.NewBlacklistController(m.repo, m.deps.Log)

	// Background Ticker (Periodic Cron) for polling RSS feeds.
	if m.deps.Queue != nil {
		go m.startRSSCron()
	}

	return nil
}

// startRSSCron is a simple periodic task that polls enabled RSS feeds and queues them for refresh every 15 minutes.
func (m *Module) startRSSCron() {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.deps.Log.Info().Msg("crawler: running periodic RSS feed refresher")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		enabled := true
		feeds, err := m.repo.ListFeeds(ctx, &enabled, "")
		cancel()

		if err != nil {
			m.deps.Log.Error().Err(err).Msg("crawler: rss cron list feeds failed")
			continue
		}

		for _, feed := range feeds {
			payload := entities.RefreshFeedPayload{
				FeedID: feed.ID,
				Force:  false,
			}
			jobCtx, jobCancel := context.WithTimeout(context.Background(), 5*time.Second)
			_, err := m.deps.Queue.Enqueue(
				jobCtx,
				"crawler:refresh_feed",
				payload,
				queue.WithQueue(queue.PriorityDefault),
			)
			jobCancel()
			if err != nil {
				m.deps.Log.Warn().Err(err).Str("feed", feed.URL).Msg("crawler: rss cron enqueue failed")
			}
		}
	}
}

// RegisterRoutes mounts the crawler module's HTTP routes onto the Gin router.
// Route prefix: /api/crawler, /api/rss, /api/blacklist (như NestJS @Controller decorators).
func (m *Module) RegisterRoutes(r *gin.Engine) {
	// Crawl endpoints.
	m.ctrl.RegisterRoutes(r)

	// RSS feed endpoints.
	m.rssCtrl.RegisterRoutes(r)

	// Blacklist endpoints.
	m.blCtrl.RegisterRoutes(r)
}

// Stop implements plugin.Module — performs graceful shutdown.
func (m *Module) Stop(ctx context.Context) error {
	if m.deps.Log != nil {
		m.deps.Log.Info().Msg("crawler: module stopping")
	}
	if m.svc != nil && m.svc.SSEHub() != nil {
		m.svc.SSEHub().Drain()
	}
	time.Sleep(500 * time.Millisecond)
	if m.deps.Log != nil {
		m.deps.Log.Info().Msg("crawler: module stopped")
	}
	return nil
}

// Service returns the crawler's service instance (for cross-module integration).
func (m *Module) Service() *crawlerservice.Service { return m.svc }

// Repository returns the crawler's repository instance (for cross-module integration).
func (m *Module) Repository() *repository.Repository { return m.repo }
