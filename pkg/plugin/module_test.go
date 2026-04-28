package plugin

import (
	"context"
	"testing"
)

type testModule struct {
	name   string
	setupF func() error
	stopF  func() error
}

func (t *testModule) Name() string                   { return t.name }
func (t *testModule) Setup() error                   { return t.setupF() }
func (t *testModule) RegisterRoutes(r interface{})   {}
func (t *testModule) Stop(ctx context.Context) error { return t.stopF() }

// TestModuleSpecString tests ModuleSpec String() method.
func TestModuleSpecString(t *testing.T) {
	s := ModuleSpec{Name: "test-module"}
	got := s.String()
	want := "module/test-module"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

// TestEnabledEmpty tests Enabled() with nil/empty input.
func TestEnabledEmpty(t *testing.T) {
	_ = Enabled(nil)
	_ = Enabled([]string{})
}

// TestCount tests that count() returns a non-negative number.
func TestCount(t *testing.T) {
	n := count()
	if n < 0 {
		t.Errorf("count() = %d, want >= 0", n)
	}
}
