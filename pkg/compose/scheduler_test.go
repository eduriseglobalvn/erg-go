package compose

import (
	"testing"
)

func TestCycleErrorError(t *testing.T) {
	manifest := &ServiceManifest{
		Services: []ServiceSpec{
			{Name: "a", Enabled: true, Dependencies: []string{"b"}},
			{Name: "b", Enabled: true, Dependencies: []string{"a"}},
		},
	}
	_, err := Resolve(manifest)
	if err == nil {
		t.Fatal("expected cycle error")
	}
	ce, ok := err.(*CycleError)
	if !ok {
		t.Fatalf("expected *CycleError, got %T", err)
	}
	got := ce.Error()
	if got == "" {
		t.Error("Error() returned empty string")
	}
}

func TestCycleErrorServiceNames(t *testing.T) {
	manifest := &ServiceManifest{
		Services: []ServiceSpec{
			{Name: "svc-a", Enabled: true, Dependencies: []string{"svc-b"}},
			{Name: "svc-b", Enabled: true, Dependencies: []string{"svc-a"}},
		},
	}
	_, err := Resolve(manifest)
	ce, ok := err.(*CycleError)
	if !ok {
		t.Fatalf("expected *CycleError, got %T", err)
	}
	names := ce.serviceNames()
	if len(names) != 2 {
		t.Errorf("got %d names, want 2", len(names))
	}
}

func TestDependencyGraph_standaloneService(t *testing.T) {
	manifest := &ServiceManifest{
		Services: []ServiceSpec{
			{Name: "crawler", Enabled: true, Dependencies: []string{}},
		},
	}
	order, err := Resolve(manifest)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(order) != 1 {
		t.Errorf("got %d, want 1", len(order))
	}
	if order[0].Name != "crawler" {
		t.Errorf("got %q, want crawler", order[0].Name)
	}
}

func TestDependencyGraph_noDependencies(t *testing.T) {
	manifest := &ServiceManifest{
		Services: []ServiceSpec{
			{Name: "crawler", Enabled: true, Dependencies: []string{}},
			{Name: "notification", Enabled: true, Dependencies: []string{}},
		},
	}
	order, err := Resolve(manifest)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(order) != 2 {
		t.Errorf("got %d, want 2", len(order))
	}
}

func TestDependencyGraph_parallelBranches(t *testing.T) {
	// crawler feeds into both trending and bot.
	// trending feeds into bot.
	// Order must be: crawler → trending, crawler → bot, trending → bot
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
		t.Error("crawler must be before trending")
	}
	if pos["crawler"] >= pos["bot"] {
		t.Error("crawler must be before bot")
	}
	if pos["trending"] >= pos["bot"] {
		t.Error("trending must be before bot")
	}
}
