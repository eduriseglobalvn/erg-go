package compose

import (
	"os"
	"path/filepath"
	"testing"

	"erg.ninja/pkg/config"
)

func TestEnabledServices(t *testing.T) {
	manifest := &ServiceManifest{
		Services: []ServiceSpec{
			{Name: "a", Enabled: true},
			{Name: "b", Enabled: false},
			{Name: "c", Enabled: true},
		},
	}
	got := EnabledServices(manifest)
	want := []string{"a", "c"}
	if len(got) != len(want) {
		t.Fatalf("got %d, want %d", len(got), len(want))
	}
	for i, s := range got {
		if s.Name != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, s.Name, want[i])
		}
	}
}

func TestResolve_linear(t *testing.T) {
	manifest := &ServiceManifest{
		Services: []ServiceSpec{
			{Name: "crawler", Enabled: true, Dependencies: []string{}},
			{Name: "trending", Enabled: true, Dependencies: []string{"crawler"}},
			{Name: "bot", Enabled: true, Dependencies: []string{"crawler", "trending"}},
		},
	}
	order, err := Resolve(manifest)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	pos := make(map[string]int)
	for i, s := range order {
		pos[s.Name] = i
	}
	if pos["crawler"] >= pos["trending"] {
		t.Error("crawler must come before trending")
	}
	if pos["trending"] >= pos["bot"] {
		t.Error("trending must come before bot")
	}
}

func TestResolve_disabledServiceSkipped(t *testing.T) {
	manifest := &ServiceManifest{
		Services: []ServiceSpec{
			{Name: "crawler", Enabled: false, Dependencies: []string{}},
			{Name: "trending", Enabled: true, Dependencies: []string{"crawler"}},
		},
	}
	order, err := Resolve(manifest)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(order) != 1 || order[0].Name != "trending" {
		t.Errorf("got order = %v, want [trending]", order)
	}
}

func TestResolve_cycle(t *testing.T) {
	manifest := &ServiceManifest{
		Services: []ServiceSpec{
			{Name: "a", Enabled: true, Dependencies: []string{"b"}},
			{Name: "b", Enabled: true, Dependencies: []string{"a"}},
		},
	}
	_, err := Resolve(manifest)
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
	if _, ok := err.(*CycleError); !ok {
		t.Errorf("expected *CycleError, got %T", err)
	}
}

func TestResolve_selfCycle(t *testing.T) {
	manifest := &ServiceManifest{
		Services: []ServiceSpec{
			{Name: "svc", Enabled: true, Dependencies: []string{"svc"}},
		},
	}
	_, err := Resolve(manifest)
	if err == nil {
		t.Fatal("expected self-cycle error, got nil")
	}
}

func TestResolve_empty(t *testing.T) {
	manifest := &ServiceManifest{Services: nil}
	order, err := Resolve(manifest)
	if err != nil {
		t.Fatalf("Resolve(nil services) error = %v", err)
	}
	if len(order) != 0 {
		t.Errorf("got order len %d, want 0", len(order))
	}
}

func TestValidateManifest_missingDep(t *testing.T) {
	manifest := &ServiceManifest{
		Services: []ServiceSpec{
			{Name: "bot", Enabled: true, Dependencies: []string{"nonexistent"}},
		},
	}
	err := validateManifest(manifest)
	if err == nil {
		t.Fatal("expected error for missing dependency")
	}
	snf, ok := err.(*ServiceNotFoundError)
	if !ok {
		t.Fatalf("expected *ServiceNotFoundError, got %T", err)
	}
	if snf.MissingService != "nonexistent" || snf.DependedBy != "bot" {
		t.Errorf("got %+v, want MissingService=nonexistent, DependedBy=bot", snf)
	}
}

func TestValidateManifest_disabledDep(t *testing.T) {
	manifest := &ServiceManifest{
		Services: []ServiceSpec{
			{Name: "crawler", Enabled: false, Dependencies: []string{}},
			{Name: "trending", Enabled: true, Dependencies: []string{"crawler"}},
		},
	}
	err := validateManifest(manifest)
	if err == nil {
		t.Fatal("expected error for disabled dependency")
	}
	if _, ok := err.(*DisabledDependencyError); !ok {
		t.Fatalf("expected *DisabledDependencyError, got %T", err)
	}
}

func TestMergeServiceConfig_noOverrides(t *testing.T) {
	cfg := &config.Config{
		App: config.AppConfig{Name: "test-app", Port: 8080},
	}
	merged, err := MergeServiceConfig(cfg, nil)
	if err != nil {
		t.Fatalf("MergeServiceConfig nil map: %v", err)
	}
	if merged.App.Name != "test-app" {
		t.Errorf("App.Name = %q, want %q", merged.App.Name, "test-app")
	}
	if merged.App.Port != 8080 {
		t.Errorf("App.Port = %d, want %d", merged.App.Port, 8080)
	}
}

func TestMergeServiceConfig_withOverrides(t *testing.T) {
	cfg := &config.Config{
		App: config.AppConfig{Name: "test-app", Port: 8080},
	}
	overrides := map[string]any{
		"app": map[string]any{
			"port": 9090,
		},
	}
	merged, err := MergeServiceConfig(cfg, overrides)
	if err != nil {
		t.Fatalf("MergeServiceConfig overrides: %v", err)
	}
	if merged.App.Port != 9090 {
		t.Errorf("App.Port = %d, want %d", merged.App.Port, 9090)
	}
	if merged.App.Name != "test-app" {
		t.Errorf("App.Name = %q, want %q (should not change)", merged.App.Name, "test-app")
	}
}

func TestMergeServiceConfig_nestedMerge(t *testing.T) {
	cfg := &config.Config{
		Scraper: config.ScraperConfig{MinDelay: 3, MaxDelay: 10},
	}
	overrides := map[string]any{
		"scraper": map[string]any{
			"max_delay": 60,
		},
	}
	merged, err := MergeServiceConfig(cfg, overrides)
	if err != nil {
		t.Fatalf("MergeServiceConfig nested: %v", err)
	}
	if merged.Scraper.MaxDelay != 60 {
		t.Errorf("Scraper.MaxDelay = %d, want %d", merged.Scraper.MaxDelay, 60)
	}
	if merged.Scraper.MinDelay != 3 {
		t.Errorf("Scraper.MinDelay = %d, want %d (should not change)", merged.Scraper.MinDelay, 3)
	}
}

func TestReverseOrder(t *testing.T) {
	svcs := []*ServiceSpec{
		{Name: "a"}, {Name: "b"}, {Name: "c"},
	}
	rev := ReverseOrder(svcs)
	if len(rev) != 3 {
		t.Fatalf("len = %d, want 3", len(rev))
	}
	want := []string{"c", "b", "a"}
	for i, s := range rev {
		if s.Name != want[i] {
			t.Errorf("rev[%d] = %q, want %q", i, s.Name, want[i])
		}
	}
}

func TestGetService(t *testing.T) {
	manifest := &ServiceManifest{
		Services: []ServiceSpec{
			{Name: "bot", Enabled: true},
			{Name: "crawler", Enabled: false},
		},
	}
	if got := GetService(manifest, "bot"); got == nil || got.Name != "bot" {
		t.Errorf("GetService(bot) = %v, want non-nil bot", got)
	}
	if got := GetService(manifest, "missing"); got != nil {
		t.Errorf("GetService(missing) = %v, want nil", got)
	}
}

func TestDirectDependencies(t *testing.T) {
	svc := &ServiceSpec{Name: "bot", Dependencies: []string{"crawler", "trending"}}
	deps := DirectDependencies(svc)
	if len(deps) != 2 {
		t.Fatalf("len = %d, want 2", len(deps))
	}
	if deps[0] != "crawler" || deps[1] != "trending" {
		t.Errorf("got %v, want [crawler trending]", deps)
	}
}

func TestIsDependedOnBy(t *testing.T) {
	manifest := &ServiceManifest{
		Services: []ServiceSpec{
			{Name: "crawler", Enabled: true, Dependencies: []string{}},
			{Name: "trending", Enabled: true, Dependencies: []string{"crawler"}},
			{Name: "bot", Enabled: true, Dependencies: []string{"crawler"}},
		},
	}
	if !IsDependedOnBy(manifest, "crawler") {
		t.Error("crawler should be depended on by trending and bot")
	}
	if IsDependedOnBy(manifest, "trending") {
		t.Error("trending has no dependents")
	}
}

func TestServiceSpec_HasDependency(t *testing.T) {
	svc := &ServiceSpec{Dependencies: []string{"a", "b"}}
	if !svc.HasDependency("a") {
		t.Error("should have dep a")
	}
	if svc.HasDependency("c") {
		t.Error("should not have dep c")
	}
}

func TestLoad_deployNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/deploy.yaml", config.NewDefault())
	if err == nil {
		t.Fatal("expected ErrNoDeployManifest")
	}
}

func TestLoad_validManifest(t *testing.T) {
	tmpDir := t.TempDir()
	deployPath := filepath.Join(tmpDir, "deploy.yaml")
	const yamlContent = `
services:
  - name: bot
    enabled: true
    port: 8081
    dependencies: []
    config:
      bot:
        command_prefix: "/"
`
	if err := os.WriteFile(deployPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	manifest, err := Load(deployPath, config.NewDefault())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(manifest.Services) != 1 {
		t.Fatalf("got %d services, want 1", len(manifest.Services))
	}
	if manifest.Services[0].Name != "bot" {
		t.Errorf("got name %q, want bot", manifest.Services[0].Name)
	}
	if !manifest.Services[0].Enabled {
		t.Error("bot should be enabled")
	}
}

func TestLoad_missingDependency(t *testing.T) {
	tmpDir := t.TempDir()
	deployPath := filepath.Join(tmpDir, "deploy.yaml")
	const yamlContent = `
services:
  - name: bot
    enabled: true
    dependencies:
      - nonexistent
`
	if err := os.WriteFile(deployPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	_, err := Load(deployPath, config.NewDefault())
	if err == nil {
		t.Fatal("expected error for missing dependency")
	}
}

func TestLoad_disabledDependency(t *testing.T) {
	tmpDir := t.TempDir()
	deployPath := filepath.Join(tmpDir, "deploy.yaml")
	const yamlContent = `
services:
  - name: crawler
    enabled: false
    dependencies: []
  - name: trending
    enabled: true
    dependencies:
      - crawler
`
	if err := os.WriteFile(deployPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	_, err := Load(deployPath, config.NewDefault())
	if err == nil {
		t.Fatal("expected error for disabled dependency")
	}
}

func TestResolve_threeLayer(t *testing.T) {
	manifest := &ServiceManifest{
		Services: []ServiceSpec{
			{Name: "notification", Enabled: true, Dependencies: []string{}},
			{Name: "crawler", Enabled: true, Dependencies: []string{}},
			{Name: "trending", Enabled: true, Dependencies: []string{"crawler"}},
			{Name: "bot", Enabled: true, Dependencies: []string{"crawler", "trending"}},
		},
	}
	order, err := Resolve(manifest)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	pos := make(map[string]int)
	for i, s := range order {
		pos[s.Name] = i
	}
	if pos["crawler"] >= pos["trending"] {
		t.Error("crawler must come before trending")
	}
	if pos["crawler"] >= pos["bot"] {
		t.Error("crawler must come before bot")
	}
	if pos["trending"] >= pos["bot"] {
		t.Error("trending must come before bot")
	}
}
