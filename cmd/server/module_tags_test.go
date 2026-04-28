package main

import (
	"context"
	"testing"

	"erg.ninja/pkg/plugin"
)

// TestEnabledModules returns non-empty when build tags are active.
func TestEnabledModules(t *testing.T) {
	mods := EnabledModules()
	if len(mods) == 0 {
		t.Skip("no build tags active — this test requires at least one module tag")
	}
}

// TestPluginRegistryBotNonEmpty bots non-empty when module tags active.
func TestPluginRegistryBotNonEmpty(t *testing.T) {
	specs := plugin.Registered()
	if len(specs) == 0 {
		t.Skip("plugin registry empty — no build tags active")
	}
	for _, s := range specs {
		if s.Name == "" {
			t.Errorf("plugin spec has empty name: %+v", s)
		}
		if s.Module == nil {
			t.Errorf("plugin spec %q has nil Module", s.Name)
		}
	}
}

// TestPluginEnabledFiltering tests plugin.Enabled() allowlist.
func TestPluginEnabledFiltering(t *testing.T) {
	specs := plugin.Registered()
	if len(specs) == 0 {
		t.Skip("no modules registered")
	}

	// Empty = all registered
	all := plugin.Enabled(nil)
	if len(all) != len(specs) {
		t.Errorf("Enabled(nil): got %d, want %d", len(all), len(specs))
	}

	all2 := plugin.Enabled([]string{})
	if len(all2) != len(specs) {
		t.Errorf("Enabled([]): got %d, want %d", len(all2), len(specs))
	}

	// Unknown module → filtered out
	none := plugin.Enabled([]string{"nonexistent-module-xyz"})
	if len(none) != 0 {
		t.Errorf("Enabled([unknown]): got %d, want 0", len(none))
	}

	// Known module → included
	if len(specs) >= 1 {
		first := []string{specs[0].Name}
		filtered := plugin.Enabled(first)
		if len(filtered) != 1 || filtered[0].Name != specs[0].Name {
			t.Errorf("Enabled(%q): got %v, want [%v]", specs[0].Name, filtered, specs[0])
		}
	}
}

// TestPluginIsRegistered tests IsRegistered.
func TestPluginIsRegistered(t *testing.T) {
	specs := plugin.Registered()
	for _, s := range specs {
		if !plugin.IsRegistered(s.Name) {
			t.Errorf("IsRegistered(%q) = false, want true", s.Name)
		}
	}
	if plugin.IsRegistered("nonexistent-xyz-module") {
		t.Error("IsRegistered(nonexistent) = true, want false")
	}
}

// TestPluginModuleLifecycle tests Module interface compliance.
// Only runs when at least one module is registered via build tags.
// Setup() is NOT called here because modules need fully-wired dependencies
// (Mongo, Redis, Log, etc.) which are only available during normal server startup.
// The plugin registry / type-level checks (Name, Module non-nil, Stop signature)
// are sufficient to verify the build-tag wiring is correct.
func TestPluginModuleLifecycle(t *testing.T) {
	specs := plugin.Registered()
	if len(specs) == 0 {
		t.Skip("no modules registered via build tags")
	}

	for _, s := range specs {
		t.Run(s.Name, func(t *testing.T) {
			if s.Name == "" {
				t.Error("Module.Name() returned empty string")
			}
			if s.Module == nil {
				t.Fatal("Module is nil")
			}
			// Stop with a real context is always safe to call (no-op if not started).
			_ = s.Module.Stop(context.Background())
		})
	}
}
