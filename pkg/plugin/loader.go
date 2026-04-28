// Package plugin provides a runtime plugin loader for loading erg-go modules
// from .so files at startup (requires CGO_ENABLED=1 and identical Go version).
//
// For most use-cases, prefer compile-time build tags (see module_*.go files).
// Runtime loading is intended for advanced consumers who need dynamic module
// selection without recompilation.
//
// See Phase 4 of task3.md for the full design.
package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"plugin"
	"strings"
)

// Loader loads erg-go modules from .so plugin files at runtime.
type Loader struct {
	pluginDir string
}

// NewLoader creates a new plugin loader that reads .so files from pluginDir.
func NewLoader(pluginDir string) *Loader {
	return &Loader{pluginDir: pluginDir}
}

// LoadedModules tracks which plugins have been successfully loaded to avoid re-loading.
type LoadedModules struct {
	plugins map[string]*plugin.Plugin
	syms    map[string]Module // module name → resolved Module instance
}

// LoadAll loads all .so files in the loader's plugin directory whose names
// match "erg-<name>.so". Returns successfully loaded modules and errors for
// files that failed to load (partial success is returned alongside errors).
func (l *Loader) LoadAll() ([]Module, []error) {
	entries, err := os.ReadDir(l.pluginDir)
	if err != nil {
		return nil, []error{fmt.Errorf("plugin: read plugin dir %q: %w", l.pluginDir, err)}
	}

	var loaded []Module
	var errs []error

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "erg-") || !strings.HasSuffix(entry.Name(), ".so") {
			continue
		}
		name := strings.TrimSuffix(strings.TrimPrefix(entry.Name(), "erg-"), ".so")
		mod, err := l.Load(name)
		if err != nil {
			errs = append(errs, fmt.Errorf("plugin: load %s: %w", entry.Name(), err))
			continue
		}
		loaded = append(loaded, mod)
	}

	return loaded, errs
}

// Load loads a single plugin by name.
// It expects a file named "erg-<name>.so" in the plugin directory.
func (l *Loader) Load(name string) (Module, error) {
	if name == "" {
		return nil, fmt.Errorf("plugin.Load: name cannot be empty")
	}
	path := filepath.Join(l.pluginDir, fmt.Sprintf("erg-%s.so", name))

	p, err := plugin.Open(path)
	if err != nil {
		return nil, fmt.Errorf("plugin.Open(%s): %w\nNote: runtime plugins require CGO_ENABLED=1 and the same Go version as the host binary", path, err)
	}

	sym, err := p.Lookup("Module")
	if err != nil {
		return nil, fmt.Errorf("plugin.Lookup(%q): symbol %q not found: %w", name, "Module", err)
	}

	mod, ok := sym.(Module)
	if !ok {
		return nil, fmt.Errorf("plugin.Lookup(%q): symbol %q does not implement plugin.Module (got %T)", name, "Module", sym)
	}

	return mod, nil
}
