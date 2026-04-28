package database

import (
	"testing"
	"time"

	"erg.ninja/pkg/config"
)

func TestBuildMySQLDSN(t *testing.T) {
	cfg := MySQLConfig{
		Host:     "localhost",
		Port:     3306,
		User:     "root",
		Password: "secret",
		Database: "erg",
	}
	s := buildMySQLDSN(cfg)
	if s == "" {
		t.Error("buildMySQLDSN returned empty string")
	}
	if !containsStr(s, "localhost") {
		t.Error("DSN missing host")
	}
	if !containsStr(s, "3306") {
		t.Error("DSN missing port")
	}
	if !containsStr(s, "erg") {
		t.Error("DSN missing database name")
	}
	if !containsStr(s, "utf8mb4") {
		t.Error("DSN missing charset")
	}
}

func TestBuildPostgresDSN(t *testing.T) {
	cfg := config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "postgres",
		Password: "secret",
		Name:     "erg",
	}
	s := buildPostgresDSN(cfg)
	if s == "" {
		t.Error("buildPostgresDSN returned empty string")
	}
	if !containsStr(s, "localhost") {
		t.Error("DSN missing host")
	}
	if !containsStr(s, "5432") {
		t.Error("DSN missing port")
	}
	if !containsStr(s, "erg") {
		t.Error("DSN missing database name")
	}
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

func TestMySQLConfigDefaults(t *testing.T) {
	cfg := MySQLConfig{}
	if cfg.Port == 0 {
		cfg.Port = 3306
	}
	if cfg.MaxOpenConns == 0 {
		cfg.MaxOpenConns = 25
	}
	if cfg.MaxIdleConns == 0 {
		cfg.MaxIdleConns = 10
	}
	if cfg.ConnMaxLifetime == 0 {
		cfg.ConnMaxLifetime = 5 * time.Minute
	}

	if cfg.Port != 3306 {
		t.Errorf("expected port 3306, got %d", cfg.Port)
	}
	if cfg.MaxOpenConns != 25 {
		t.Errorf("expected MaxOpenConns 25, got %d", cfg.MaxOpenConns)
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && containsStrImpl(s, substr)
}

func containsStrImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
