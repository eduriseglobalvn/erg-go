package database

import (
	"testing"

	"erg.ninja/pkg/config"
)

func TestBuildPgConnStr(t *testing.T) {
	cfg := config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Password: "secret",
		Name:     "erg",
	}
	s := buildPgConnStr(cfg)
	if s == "" {
		t.Error("buildPgConnStr returned empty string")
	}
	if !contains(s, "localhost") {
		t.Error("connection string missing host")
	}
	if !contains(s, "5432") {
		t.Error("connection string missing port")
	}
	if !contains(s, "erg") {
		t.Error("connection string missing database name")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsImpl(s, substr))
}

func containsImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestDatabaseConfigDefaults(t *testing.T) {
	cfg := config.NewDefault().Database
	if cfg.Host == "" {
		t.Error("Database Host should not be empty")
	}
	if cfg.MaxOpenConns == 0 {
		t.Error("MaxOpenConns should not be zero")
	}
}
