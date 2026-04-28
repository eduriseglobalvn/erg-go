// Package routes wires modules onto the gin router.
//
// Phase 4 (task3.md):
//   - Build tags enable compile-time module selection (module_bot, module_crawler, etc.)
//   - Runtime .so loading via plugin.Loader (CGO required)
//   - Cross-module adapters (bot -> crawler, bot -> trending) wired automatically
package routes

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"

	"erg.ninja/internal/modules/bot"
	"erg.ninja/internal/modules/crawler"
	"erg.ninja/internal/modules/notifications"
	"erg.ninja/internal/modules/trending"
	"erg.ninja/pkg/plugin"
)

// PluginConfig describes which modules to enable via the plugin system.
// Populated from config.yaml -> modules.enabled list.
type PluginConfig struct {
	// EnabledModules lists the module names to load from the compile-time registry.
	// Valid values: "bot", "crawler", "notification", "trending".
	// If empty, Register falls back to legacy wiring (all 4 modules).
	EnabledModules []string `mapstructure:"enabled"`
}

// plugable is the internal module interface mirroring plugin.Module.
type plugable interface {
	Name() string
	Setup()
	RegisterRoutes(r *gin.Engine)
	Stop(ctx context.Context) error
}

// ─── adapter wrappers ─────────────────────────────────────────────────────────

type botAdapter struct{ m *bot.Module }

func (a botAdapter) Name() string                   { return "bot" }
func (a botAdapter) Setup()                         { a.m.Setup() }
func (a botAdapter) RegisterRoutes(r *gin.Engine)   { a.m.RegisterRoutes(r) }
func (a botAdapter) Stop(ctx context.Context) error { return a.m.Stop(ctx) }

type crawlerAdapter struct{ m *crawler.Module }

func (a crawlerAdapter) Name() string                   { return "crawler" }
func (a crawlerAdapter) Setup()                         { a.m.Setup() }
func (a crawlerAdapter) RegisterRoutes(r *gin.Engine)   { a.m.RegisterRoutes(r) }
func (a crawlerAdapter) Stop(ctx context.Context) error { return a.m.Stop(ctx) }

type notificationAdapter struct{ m *notifications.Module }

func (a notificationAdapter) Name() string                   { return "notification" }
func (a notificationAdapter) Setup()                         { a.m.Setup() }
func (a notificationAdapter) RegisterRoutes(r *gin.Engine)   { a.m.RegisterRoutes(r) }
func (a notificationAdapter) Stop(ctx context.Context) error { return a.m.Stop(ctx) }

type trendingAdapter struct{ m *trending.Module }

func (a trendingAdapter) Name() string                   { return "trending" }
func (a trendingAdapter) Setup()                         { a.m.Setup() }
func (a trendingAdapter) RegisterRoutes(r *gin.Engine)   { a.m.RegisterRoutes(r) }
func (a trendingAdapter) Stop(ctx context.Context) error { return a.m.Stop(ctx) }

// ─── dependency wiring ───────────────────────────────────────────────────────

func wireFromDeps(deps *Deps, name string) (plugable, error) {
	switch name {
	case "bot":
		return botAdapter{m: bot.NewModule(bot.Deps{
			Mongo: deps.Mongo, Redis: deps.Redis, Bus: deps.Bus, Log: deps.Log, Cfg: deps.Cfg,
			TenantMongoClient: deps.TenantMongoClient,
		})}, nil
	case "crawler":
		return crawlerAdapter{m: crawler.NewModule(crawler.Deps{
			Mongo: deps.Mongo, Redis: deps.Redis, Bus: deps.Bus, Log: deps.Log, Cfg: deps.Cfg,
			Queue: deps.Queue, TenantMongoClient: deps.TenantMongoClient,
		})}, nil
	case "notification", "notifications":
		return notificationAdapter{m: notifications.NewModule(notifications.Deps{
			Mongo: deps.Mongo, Redis: deps.Redis, Bus: deps.Bus, Log: deps.Log, Cfg: deps.Cfg,
			JWTValidator: deps.JWTValidator, TenantMongoClient: deps.TenantMongoClient,
		})}, nil
	case "trending":
		return trendingAdapter{m: trending.NewModule(trending.Deps{
			Mongo: deps.Mongo, Redis: deps.Redis, Bus: deps.Bus, Log: deps.Log, Cfg: deps.Cfg,
			JWTValidator: deps.JWTValidator, TenantMongoClient: deps.TenantMongoClient,
		})}, nil
	default:
		return nil, fmt.Errorf("routes: unknown module %q", name)
	}
}

// injectBotAdapters wires crawler/trending adapters into bot.
func injectBotAdapters(m plugable, deps *Deps, byName map[string]plugable) {
	if m.Name() != "bot" {
		return
	}
	ba, ok := m.(botAdapter)
	if !ok {
		return
	}

	var crawlAdapter *bot.CrawlerAdapter
	var trendSvcAdapter *bot.TrendingAdapter

	if cm, exists := byName["crawler"]; exists {
		if ca, ok := cm.(crawlerAdapter); ok && ca.m != nil && ca.m.Service() != nil {
			crawlAdapter = bot.NewCrawlerAdapter(ca.m.Service())
		}
	}
	if tm, exists := byName["trending"]; exists {
		if ta, ok := tm.(trendingAdapter); ok && ta.m != nil && ta.m.Service() != nil {
			trendSvcAdapter = bot.NewTrendingAdapter(ta.m.Service())
		}
	}
	ba.m.InjectAdapters(crawlAdapter, trendSvcAdapter)
}

// PluginWiring contains modules wired via the plugin system.
type PluginWiring struct {
	Modules   []plugable
	StopFuncs []func(context.Context)
}

// BuildFromRegistry creates a PluginWiring from the compile-time plugin registry.
// Returns an error only on catastrophic failure; skips unknown modules gracefully.
func BuildFromRegistry(deps *Deps, enabled []string) (*PluginWiring, error) {
	specs := plugin.Enabled(enabled)

	wired := make([]plugable, 0, len(specs))
	for _, spec := range specs {
		m, err := wireFromDeps(deps, spec.Name)
		if err != nil {
			deps.Log.Warn().Str("module", spec.Name).Msg("routes: skipping unknown module from registry")
			continue
		}
		wired = append(wired, m)
	}
	byName := make(map[string]plugable, len(wired))
	for _, m := range wired {
		byName[m.Name()] = m
	}
	for _, m := range wired {
		deps.Log.Info().Str("module", m.Name()).Msg("routes: setting up module")
		m.Setup()
	}
	for _, m := range wired {
		injectBotAdapters(m, deps, byName)
	}
	stops := make([]func(context.Context), 0, len(wired))
	for _, mod := range wired {
		m := mod
		stops = append(stops, func(ctx context.Context) {
			deps.Log.Info().Str("module", m.Name()).Msg("routes: stopping module")
			if err := m.Stop(ctx); err != nil {
				deps.Log.WarnContext(ctx).Err(err).Str("module", m.Name()).Msg("routes: module stop error")
			}
		})
	}
	return &PluginWiring{Modules: wired, StopFuncs: stops}, nil
}

// BuildFromLoader creates a PluginWiring by loading .so plugin files at runtime.
// Requires CGO_ENABLED=1 and identical Go version.
// Returns the wiring and a list of non-fatal loading errors (plugin errors are
// accumulated here so the caller can decide whether to warn or abort).
func BuildFromLoader(deps *Deps, loader *plugin.Loader) (*PluginWiring, []error) {
	loaded, errs := loader.LoadAll()
	if len(loaded) == 0 && len(errs) > 0 {
		return nil, errs
	}
	wired := make([]plugable, 0, len(loaded))
	for _, mod := range loaded {
		p, err := wireFromDeps(deps, mod.Name())
		if err != nil {
			errs = append(errs, fmt.Errorf("plugin: wire %q: %w", mod.Name(), err))
			continue
		}
		wired = append(wired, p)
	}
	byName := make(map[string]plugable, len(wired))
	for _, m := range wired {
		byName[m.Name()] = m
	}
	for _, m := range wired {
		m.Setup()
	}
	for _, m := range wired {
		injectBotAdapters(m, deps, byName)
	}
	stops := make([]func(context.Context), 0, len(wired))
	for _, mod := range wired {
		m := mod
		stops = append(stops, func(ctx context.Context) {
			deps.Log.Info().Str("module", m.Name()).Msg("routes: stopping module")
			_ = m.Stop(ctx)
		})
	}
	return &PluginWiring{Modules: wired, StopFuncs: stops}, errs
}
