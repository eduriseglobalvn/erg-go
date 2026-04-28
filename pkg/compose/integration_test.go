package compose

import (
	"testing"

	"erg.ninja/internal/routes"
	"erg.ninja/pkg/logger"
)

func TestNewComposeEngine(t *testing.T) {
	deps := &routes.Deps{
		Log: logger.New(logger.WithServiceName("test")),
	}
	engine := NewComposeEngine(deps)
	if engine == nil {
		t.Fatal("NewComposeEngine returned nil")
	}
	if engine.deps != deps {
		t.Error("deps not set correctly")
	}
	if engine.modules == nil {
		t.Error("modules map should be initialized")
	}
}

func TestComposeEngineModule(t *testing.T) {
	deps := &routes.Deps{
		Log: logger.New(logger.WithServiceName("test")),
	}
	engine := NewComposeEngine(deps)
	if engine.Module("nonexistent") != nil {
		t.Error("Module(nonexistent) should return nil")
	}
}

func TestComposeEngineModules(t *testing.T) {
	deps := &routes.Deps{
		Log: logger.New(logger.WithServiceName("test")),
	}
	engine := NewComposeEngine(deps)
	mods := engine.Modules()
	if len(mods) != 0 {
		t.Errorf("Modules() on empty engine = %d, want 0", len(mods))
	}
}

func TestValidateManifest_unknownService(t *testing.T) {
	manifest := &ServiceManifest{
		Services: []ServiceSpec{
			{Name: "unknown-service-xyz", Enabled: true, Dependencies: []string{}},
		},
	}
	err := ValidateManifest(manifest)
	if err == nil {
		t.Fatal("expected error for unknown service")
	}
}

func TestValidateManifest_emptyServices(t *testing.T) {
	manifest := &ServiceManifest{Services: []ServiceSpec{}}
	err := ValidateManifest(manifest)
	if err != nil {
		t.Errorf("ValidateManifest on empty services: %v", err)
	}
}

func TestValidateManifest_allValid(t *testing.T) {
	manifest := &ServiceManifest{
		Services: []ServiceSpec{
			{Name: "crawler", Enabled: true, Dependencies: []string{}},
			{Name: "notification", Enabled: true, Dependencies: []string{}},
		},
	}
	err := ValidateManifest(manifest)
	if err != nil {
		t.Errorf("ValidateManifest on valid manifest: %v", err)
	}
}
