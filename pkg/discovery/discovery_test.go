package discovery

import (
	"context"
	"testing"
	"time"
)

func TestStaticCatalog_RegisterAndFind(t *testing.T) {
	c := NewStaticCatalog(nil)
	ctx := context.Background()

	svc := Service{
		ID:      "crawler-1",
		Name:    "crawler",
		Version: "v1",
		Address: "10.0.1.5:8083",
		Tags:    []string{"grpc"},
	}
	if err := c.Register(ctx, svc); err != nil {
		t.Fatalf("Register: %v", err)
	}

	got, err := c.Find(ctx, "crawler", nil)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(got) != 1 || got[0].ID != "crawler-1" {
		t.Errorf("Find: got %v, want [crawler-1]", got)
	}
}

func TestStaticCatalog_FindByTags(t *testing.T) {
	c := NewStaticCatalog(nil)
	ctx := context.Background()

	c.Register(ctx, Service{ID: "n1", Name: "notification", Tags: []string{"grpc", "tenant=acme"}})
	c.Register(ctx, Service{ID: "n2", Name: "notification", Tags: []string{"grpc"}})

	got, _ := c.Find(ctx, "notification", []string{"grpc", "tenant=acme"})
	if len(got) != 1 || got[0].ID != "n1" {
		t.Errorf("Find with tags: got %v, want [n1]", got)
	}
}

func TestStaticCatalog_Deregister(t *testing.T) {
	c := NewStaticCatalog(nil)
	ctx := context.Background()

	c.Register(ctx, Service{ID: "svc-1", Name: "crawler", Address: "localhost:8083"})
	c.Register(ctx, Service{ID: "svc-2", Name: "crawler", Address: "localhost:8084"})

	if err := c.Deregister(ctx, "svc-1"); err != nil {
		t.Fatalf("Deregister: %v", err)
	}

	got, _ := c.Find(ctx, "crawler", nil)
	if len(got) != 1 || got[0].ID != "svc-2" {
		t.Errorf("Find after deregister: got %v, want [svc-2]", got)
	}
}

func TestBuildCatalog_Static(t *testing.T) {
	cfg := Config{
		Enabled: true,
		Backend: "static",
		Static: StaticCfg{
			Services: map[string][]StaticServiceEntry{
				"crawler": {
					{Address: "localhost:8083", Tags: []string{"grpc"}, Version: "v1"},
				},
			},
		},
	}

	c, err := BuildCatalog(context.Background(), cfg)
	if err != nil {
		t.Fatalf("BuildCatalog: %v", err)
	}

	svcs, err := c.Find(context.Background(), "crawler", nil)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(svcs) != 1 {
		t.Fatalf("expected 1 crawler instance, got %d", len(svcs))
	}
	if svcs[0].Address != "localhost:8083" {
		t.Errorf("Address: got %s, want localhost:8083", svcs[0].Address)
	}
}

func TestBuildCatalog_Disabled(t *testing.T) {
	cfg := Config{Enabled: false}
	_, err := BuildCatalog(context.Background(), cfg)
	if err == nil {
		t.Error("expected error when discovery disabled")
	}
}

func TestStaticCatalog_FindServiceNotFound(t *testing.T) {
	c := NewStaticCatalog(nil)
	got, _ := c.Find(context.Background(), "nonexistent", nil)
	if got != nil {
		t.Errorf("Find nonexistent: got %v, want nil", got)
	}
}

func TestServiceNotFoundError(t *testing.T) {
	err := &ServiceNotFoundError{Name: "crawler", Tags: []string{"grpc"}}
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

func TestPickFirst(t *testing.T) {
	svcs := []Service{
		{ID: "s1", Name: "crawler", TTL: time.Now().Add(time.Hour)},
		{ID: "s2", Name: "crawler", TTL: time.Now().Add(time.Hour)},
	}

	// Run enough times that we likely see both IDs if rand is working.
	seen := make(map[string]bool)
	for i := 0; i < 50; i++ {
		s, err := PickFirst(svcs)
		if err != nil {
			t.Fatalf("PickFirst: %v", err)
		}
		seen[s.ID] = true
	}
	if !seen["s1"] || !seen["s2"] {
		t.Errorf("PickFirst did not randomise across both instances: seen=%v", seen)
	}
}

func TestPickFirst_Expired(t *testing.T) {
	svcs := []Service{
		{ID: "s1", Name: "crawler", TTL: time.Now().Add(-time.Hour)},
		{ID: "s2", Name: "crawler", TTL: time.Now().Add(-time.Hour)},
	}
	_, err := PickFirst(svcs)
	if err == nil {
		t.Error("expected error when all instances expired")
	}
}

func TestPickFirst_Empty(t *testing.T) {
	_, err := PickFirst(nil)
	if err == nil {
		t.Error("expected error for empty slice")
	}
}

func TestResolveEnv(t *testing.T) {
	tests := []struct {
		input    string
		envKey   string
		envVal   string
		expected string
	}{
		{"${FOO}", "FOO", "bar", "bar"},
		{"$FOO", "FOO", "baz", "baz"},
		{"${FOO_BAR}", "FOO_BAR", "secret", "secret"},
		{"localhost:8083", "", "", "localhost:8083"}, // no env var reference
		{"", "", "", ""},
	}
	for _, tt := range tests {
		if tt.envKey != "" {
			t.Setenv(tt.envKey, tt.envVal)
		}
		got := resolveEnv(tt.input)
		if got != tt.expected {
			t.Errorf("resolveEnv(%q): got %q, want %q", tt.input, got, tt.expected)
		}
	}
}
