package config

import "testing"

func TestNewDefaultDisablesNonCriticalStartupWork(t *testing.T) {
	cfg := NewDefault()

	if cfg.Lifecycle.AuthBootstrapAdminOnStartup {
		t.Fatal("auth bootstrap should be opt-in")
	}
	if cfg.Lifecycle.ProfileBackfillOnStartup {
		t.Fatal("profile backfill should be opt-in")
	}
	if cfg.Lifecycle.OperationSeedOnStartup {
		t.Fatal("operations seed should be opt-in")
	}
	if cfg.Lifecycle.TrendingRefreshOnStartup {
		t.Fatal("trending refresh should be opt-in")
	}
	if cfg.Lifecycle.LMSSeedOnStartup {
		t.Fatal("lms seed should be opt-in")
	}
}
