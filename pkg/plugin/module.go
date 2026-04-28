// Package plugin defines the plugin interface used for compile-time (build tags)
// and runtime (.so) module loading in erg-go.
//
// Consumers who want only a subset of modules can:
//   - Use build tags at compile time: go build -tags "module_crawler,module_notification"
//   - Load .so plugins at runtime via Loader (requires CGO_ENABLED=1 + same Go version)
//
// All modules follow the same lifecycle: New → Setup → RegisterRoutes → Stop.
package plugin

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
)

// Module is the interface all erg-go modules must implement.
// Mirrors the NestJS module lifecycle: onModuleInit → register controllers → onModuleDestroy.
//
// Both compile-time (build-tag) and runtime (.so) modules implement this interface.
type Module interface {
	// Name returns the module's canonical name (used in logs, metrics, plugin filenames).
	// Must be unique across all modules. Examples: "bot", "crawler", "notification", "trending".
	Name() string

	// Setup initialises the module (mirrors NestJS onModuleInit).
	// Called once after NewModule and before RegisterRoutes.
	// All long-running goroutines should be started here; Stop will be called on shutdown.
	Setup() error

	// RegisterRoutes mounts the module's HTTP routes onto the given gin router.
	// Routes are typically prefixed, e.g. /api/bot, /api/crawler, /webhooks/discord.
	// Safe to call multiple times; subsequent calls are no-ops.
	RegisterRoutes(r *gin.Engine)

	// Stop performs graceful shutdown of the module.
	// Should stop all background goroutines, drain connections, flush buffers.
	// Called exactly once during server shutdown before the HTTP server stops.
	Stop(ctx context.Context) error
}

// ModuleSpec bundles a Module instance with its metadata for registration.
type ModuleSpec struct {
	Name   string
	Module Module
}

// String implements fmt.Stringer for ergonomic logging.
func (s ModuleSpec) String() string { return "module/" + s.Name }

// ─── Module registration (compile-time) ─────────────────────────────────────────

// Registry holds all registered modules. It is populated by init() functions
// in build-tag-gated files (module_<name>.go).
var registry = make(map[string]Module)

// Register registers a module by name. Called by each module's init() via build tag guards.
// Panics if a module with the same name is already registered.
func Register(name string, mod Module) {
	if name == "" {
		panic("plugin.Register: name cannot be empty")
	}
	if mod == nil {
		panic("plugin.Register: module cannot be nil")
	}
	if _, exists := registry[name]; exists {
		panic(fmt.Sprintf("plugin.Register: module %q already registered", name))
	}
	registry[name] = mod
}

// Registered returns a snapshot of all currently registered modules.
// The returned slice is a copy; modifying it does not affect the registry.
func Registered() []ModuleSpec {
	specs := make([]ModuleSpec, 0, len(registry))
	for name, mod := range registry {
		specs = append(specs, ModuleSpec{Name: name, Module: mod})
	}
	return specs
}

// Enabled returns modules whose names appear in the enabled set.
// If enabled is nil or empty, all registered modules are returned (allowlist fallback).
func Enabled(enabled []string) []ModuleSpec {
	if len(enabled) == 0 {
		return Registered()
	}
	enabledSet := make(map[string]struct{}, len(enabled))
	for _, n := range enabled {
		enabledSet[n] = struct{}{}
	}
	specs := make([]ModuleSpec, 0)
	for name, mod := range registry {
		if _, ok := enabledSet[name]; ok {
			specs = append(specs, ModuleSpec{Name: name, Module: mod})
		}
	}
	return specs
}

// IsRegistered reports whether a module with the given name is registered.
func IsRegistered(name string) bool {
	_, ok := registry[name]
	return ok
}

// count returns the number of registered modules (useful for diagnostics).
func count() int { return len(registry) }
