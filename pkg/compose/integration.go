package compose

import (
	"context"
	"fmt"
	"net/http"

	"erg.ninja/internal/modules/bot"
	"erg.ninja/internal/modules/crawler"
	"erg.ninja/internal/modules/notifications"
	"erg.ninja/internal/modules/trending"
	"erg.ninja/internal/routes"
	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/database"
	"erg.ninja/pkg/event"
	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/plugin"
	"erg.ninja/pkg/queue"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ModuleKey identifies a built-in service module.
type ModuleKey string

const (
	ModuleBot          ModuleKey = "bot"
	ModuleCrawler      ModuleKey = "crawler"
	ModuleNotification ModuleKey = "notification"
	ModuleTrending     ModuleKey = "trending"
)

// String implements fmt.Stringer.
func (k ModuleKey) String() string { return string(k) }

// ModuleConstructor is the signature for a module factory.
type ModuleConstructor func(deps *routes.Deps, serviceCfg map[string]any) (plugin.Module, error)

// moduleRegistry maps module names to their constructors.
var moduleRegistry = make(map[ModuleKey]ModuleConstructor)

// RegisterModule registers a module constructor.
// It panics if a module name is registered twice.
func RegisterModule(key ModuleKey, fn ModuleConstructor) {
	if _, ok := moduleRegistry[key]; ok {
		panic(fmt.Sprintf("compose: module %q is already registered", key))
	}
	moduleRegistry[key] = fn
}

// getConstructor looks up a module constructor by name.
// It returns false if the module is not registered.
func getConstructor(name string) (ModuleKey, ModuleConstructor, bool) {
	key := ModuleKey(name)
	fn, ok := moduleRegistry[key]
	return key, fn, ok
}

// ── Init functions (wired into moduleRegistry) ─────────────────────────────────

func init() {
	RegisterModule(ModuleBot, newBotModule)
	RegisterModule(ModuleCrawler, newCrawlerModule)
	RegisterModule(ModuleNotification, newNotificationModule)
	RegisterModule(ModuleTrending, newTrendingModule)
}

func newBotModule(deps *routes.Deps, serviceCfg map[string]any) (plugin.Module, error) {
	cfg := deps.Cfg
	if len(serviceCfg) > 0 {
		merged, err := MergeServiceConfig(cfg, serviceCfg)
		if err != nil {
			return nil, fmt.Errorf("compose: merge bot config: %w", err)
		}
		cfg = merged
	}
	botDeps := bot.Deps{
		Mongo: deps.Mongo,
		Redis: deps.Redis,
		Bus:   deps.Bus,
		Log:   deps.Log,
		Cfg:   cfg,
	}
	return bot.NewModule(botDeps), nil
}

func newCrawlerModule(deps *routes.Deps, serviceCfg map[string]any) (plugin.Module, error) {
	cfg := deps.Cfg
	if len(serviceCfg) > 0 {
		merged, err := MergeServiceConfig(cfg, serviceCfg)
		if err != nil {
			return nil, fmt.Errorf("compose: merge crawler config: %w", err)
		}
		cfg = merged
	}
	crawlerDeps := crawler.Deps{
		Mongo: deps.Mongo,
		Redis: deps.Redis,
		Bus:   deps.Bus,
		Log:   deps.Log,
		Cfg:   cfg,
		Queue: deps.Queue,
	}
	return crawler.NewModule(crawlerDeps), nil
}

func newNotificationModule(deps *routes.Deps, serviceCfg map[string]any) (plugin.Module, error) {
	cfg := deps.Cfg
	if len(serviceCfg) > 0 {
		merged, err := MergeServiceConfig(cfg, serviceCfg)
		if err != nil {
			return nil, fmt.Errorf("compose: merge notification config: %w", err)
		}
		cfg = merged
	}
	notifDeps := notifications.Deps{
		Mongo:        deps.Mongo,
		Redis:        deps.Redis,
		Bus:          deps.Bus,
		Log:          deps.Log,
		Cfg:          cfg,
		JWTValidator: deps.JWTValidator,
	}
	return notifications.NewModule(notifDeps), nil
}

func newTrendingModule(deps *routes.Deps, serviceCfg map[string]any) (plugin.Module, error) {
	cfg := deps.Cfg
	if len(serviceCfg) > 0 {
		merged, err := MergeServiceConfig(cfg, serviceCfg)
		if err != nil {
			return nil, fmt.Errorf("compose: merge trending config: %w", err)
		}
		cfg = merged
	}
	trendDeps := trending.Deps{
		Mongo:        deps.Mongo,
		Redis:        deps.Redis,
		Bus:          deps.Bus,
		Log:          deps.Log,
		Cfg:          cfg,
		JWTValidator: deps.JWTValidator,
	}
	return trending.NewModule(trendDeps), nil
}

// ── ComposeEngine ───────────────────────────────────────────────────────────────

// ComposeEngine wires services from a manifest onto a gin router.
type ComposeEngine struct {
	deps    *routes.Deps
	modules map[string]plugin.Module // name → started module
}

// NewComposeEngine creates a new engine backed by the shared service dependencies.
func NewComposeEngine(deps *routes.Deps) *ComposeEngine {
	return &ComposeEngine{
		deps:    deps,
		modules: make(map[string]plugin.Module),
	}
}

// Bootstrap loads manifest, resolves dependency order, constructs enabled
// modules, wires cross-module adapters, and registers routes onto r.
func (e *ComposeEngine) Bootstrap(ctx context.Context, r *gin.Engine, manifest *ServiceManifest) ([]*ServiceSpec, []func(context.Context), error) {
	// 1. Resolve dependency order.
	order, err := Resolve(manifest)
	if err != nil {
		return nil, nil, fmt.Errorf("compose: bootstrap: %w", err)
	}

	// 2. Attach /metrics handler once.
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// 3. Start modules in dependency order.
	for _, svc := range order {
		_, fn, ok := getConstructor(svc.Name)
		if !ok {
			e.deps.Log.Warn().Str("service", svc.Name).Msg("compose: unknown service, skipping")
			continue
		}
		module, err := fn(e.deps, svc.Config)
		if err != nil {
			return nil, nil, fmt.Errorf("compose: construct service %q: %w", svc.Name, err)
		}
		e.modules[svc.Name] = module
	}

	// 4. Wire cross-module adapters (bot needs crawler + trending).
	if botMod, ok := e.modules["bot"]; ok {
		crawlerMod, hasCrawler := e.modules["crawler"]
		trendMod, hasTrend := e.modules["trending"]
		if hasCrawler && hasTrend {
			injectBotAdapters(botMod, crawlerMod, trendMod)
		} else if !hasCrawler && !hasTrend {
			// Bot standalone — that's fine.
		} else {
			e.deps.Log.Warn().Msg("compose: bot adapter injection skipped — crawler or trending not available")
		}
	}

	// 5. Register routes and collect stop functions.
	var stops []func(context.Context)
	for _, svc := range order {
		mod, started := e.modules[svc.Name]
		if !started {
			continue
		}
		subRouter := gin.New()
		if err := mod.Setup(); err != nil {
			e.deps.Log.WarnContext(ctx).Err(err).Str("service", svc.Name).Msg("compose: module setup failed")
		}
		mod.RegisterRoutes(subRouter)
		// Mount subRouter at /svcName — wildcard catches everything under the prefix.
		r.Any("/"+svc.Name+"/{*path}", gin.WrapH(http.StripPrefix("/"+svc.Name, subRouter)))

		svcName := svc.Name
		stops = append(stops, func(ctx context.Context) {
			if err := mod.Stop(ctx); err != nil {
				e.deps.Log.WarnContext(ctx).Err(err).
					Str("service", svcName).Msg("compose: module stop failed")
			}
		})
	}

	return order, stops, nil
}

// Shutdown stops all modules in reverse dependency order.
func (e *ComposeEngine) Shutdown(ctx context.Context, manifest *ServiceManifest) {
	order, err := Resolve(manifest)
	if err != nil {
		e.deps.Log.Warn().Err(err).Msg("compose: shutdown: could not resolve order")
		return
	}
	for _, svc := range ReverseOrder(order) {
		mod, ok := e.modules[svc.Name]
		if !ok {
			continue
		}
		if err := mod.Stop(ctx); err != nil {
			e.deps.Log.WarnContext(ctx).Err(err).
				Str("service", svc.Name).Msg("compose: shutdown stop failed")
		}
	}
}

// Module returns a started module by name, or nil if not found.
func (e *ComposeEngine) Module(name string) plugin.Module {
	return e.modules[name]
}

// Modules returns all started modules.
func (e *ComposeEngine) Modules() map[string]plugin.Module {
	m := make(map[string]plugin.Module, len(e.modules))
	for k, v := range e.modules {
		m[k] = v
	}
	return m
}

// ValidateManifest performs deep validation on manifest.
func ValidateManifest(manifest *ServiceManifest) error {
	for _, svc := range manifest.Services {
		if !svc.Enabled {
			continue
		}
		if _, _, ok := getConstructor(svc.Name); !ok {
			return fmt.Errorf("compose: manifest references unknown service %q", svc.Name)
		}
	}
	return nil
}

// injectBotAdapters wires crawler and trending adapters into the bot module.
func injectBotAdapters(botMod, crawlerMod, trendMod plugin.Module) {
	bm, ok := botMod.(*bot.Module)
	if !ok {
		return
	}
	ca := bot.NewCrawlerAdapter(crawlerMod.(interface{ Service() *crawler.Service }).Service())
	ta := bot.NewTrendingAdapter(trendMod.(interface{ Service() *trending.Service }).Service())
	bm.InjectAdapters(ca, ta)
}

// ── Convenience helpers ───────────────────────────────────────────────────────

// MinimalDeps returns a Deps struct with only the fields that compose needs.
func MinimalDeps(log *logger.Logger) *routes.Deps {
	return &routes.Deps{
		Log: log,
		Cfg: config.NewDefault(),
	}
}

// WithMongo attaches a MongoDB client to deps.
func WithMongo(deps *routes.Deps, mongo *database.MongoClient) *routes.Deps {
	deps.Mongo = mongo
	return deps
}

// WithRedis attaches a Redis client to deps.
func WithRedis(deps *routes.Deps, redis *cache.RedisClient) *routes.Deps {
	deps.Redis = redis
	return deps
}

// WithEventBus attaches an event bus to deps.
func WithEventBus(deps *routes.Deps, bus *event.EventBus) *routes.Deps {
	deps.Bus = bus
	return deps
}

// WithQueue attaches an Asynq queue client to deps.
func WithQueue(deps *routes.Deps, q *queue.AsynqClient) *routes.Deps {
	deps.Queue = q
	return deps
}
