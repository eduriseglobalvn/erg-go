// Package database provides GORM-based database clients for PostgreSQL and MySQL.
// MongoDB uses the go.mongodb.org/mongo-driver directly (see mongo.go).
// Note: gorm.io/driver/mongodb does NOT exist — GORM v2 officially supports
// only PostgreSQL, MySQL, SQLite, SQL Server, TiDB, Clickhouse, GaussDB, Oracle.
package database

import (
	"context"
	"fmt"
	"time"

	"erg.ninja/pkg/config"
	erglog "erg.ninja/pkg/logger"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlog "gorm.io/gorm/logger"
)

// PostgresClient wraps a GORM *gorm.DB connected to PostgreSQL.
type PostgresClient struct {
	db  *gorm.DB
	cfg config.DatabaseConfig
	log *erglog.Logger
}

// PostgresOption configures a PostgresClient.
type PostgresOption func(*PostgresClient)

// WithPostgresLogger sets the logger for the PostgreSQL client.
func WithPostgresLogger(log *erglog.Logger) PostgresOption {
	return func(p *PostgresClient) {
		p.log = log
	}
}

// NewPostgresClient creates a new GORM PostgreSQL connection from the given config.
// It configures connection pooling, sslmode, and health-check ping.
func NewPostgresClient(ctx context.Context, cfg config.DatabaseConfig, opts ...PostgresOption) (*PostgresClient, error) {
	p := &PostgresClient{cfg: cfg, log: erglog.NoOp()}
	for _, o := range opts {
		o(p)
	}

	dsn := buildPostgresDSN(cfg)

	gormConfig := &gorm.Config{
		// Suppress GORM's default logger — we use zerolog instead.
		Logger: gormlog.Default.LogMode(gormlog.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	}

	db, err := gorm.Open(postgres.Open(dsn), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("postgres: open connection: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("postgres: get underlying sql.DB: %w", err)
	}

	// Connection pool settings.
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	if cfg.ConnMaxIdleTime > 0 {
		sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	}

	// Verify connectivity.
	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("postgres: ping: %w", err)
	}

	p.db = db
	p.log.Info().
		Str("database", cfg.Name).
		Str("host", cfg.Host).
		Int("port", cfg.Port).
		Int("max_open_conns", cfg.MaxOpenConns).
		Msg("postgres connected via GORM")

	return p, nil
}

// DB returns the underlying GORM *gorm.DB instance. Use for all GORM operations.
func (p *PostgresClient) DB() *gorm.DB {
	return p.db
}

// Close shuts down the PostgreSQL connection pool.
func (p *PostgresClient) Close() error {
	sqlDB, err := p.db.DB()
	if err != nil {
		return fmt.Errorf("postgres: get underlying sql.DB: %w", err)
	}
	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("postgres: close: %w", err)
	}
	p.log.Info().Msg("postgres connection closed")
	return nil
}

// Ping checks database connectivity.
func (p *PostgresClient) Ping(ctx context.Context) error {
	sqlDB, err := p.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

// AutoMigrate runs GORM AutoMigrate for the given models.
func (p *PostgresClient) AutoMigrate(dst ...interface{}) error {
	return p.db.AutoMigrate(dst...)
}

// buildPostgresDSN builds a lib/pq-compatible DSN from config.
// Uses sslmode=require for production; sslmode=disable for local dev.
func buildPostgresDSN(cfg config.DatabaseConfig) string {
	sslmode := "disable"
	if cfg.Name != "" {
		// Default to sslmode=require for non-empty names (most cloud DBs).
		sslmode = "require"
	}
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host,
		cfg.Port,
		cfg.User,
		cfg.Password,
		cfg.Name,
		sslmode,
	)
	return dsn
}

// ─────────────────────────────────────────────────────────────────────────────
// PostgreSQL Config (Phase F0-01)
// ─────────────────────────────────────────────────────────────────────────────

// PostgresConfig holds PostgreSQL connection settings for GORMPostgresClient.
type PostgresConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	Database        string
	SSLMode         string // "disable" | "require" | "verify-ca" | "verify-full"
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// GORMPostgresClient wraps a GORM *gorm.DB connected to PostgreSQL.
// Alias for PostgresClient to match the GORMMySQLClient naming convention.
// Use NewGORMPostgresClient for construction.
type GORMPostgresClient = PostgresClient

// NewGORMPostgresClient creates a new GORM PostgreSQL connection from PostgresConfig.
// Falls back to the existing config.DatabaseConfig when cfg fields are zero-valued.
func NewGORMPostgresClient(ctx context.Context, cfg PostgresConfig, log *erglog.Logger) (*GORMPostgresClient, error) {
	// Fall back to sensible defaults.
	if cfg.Port == 0 {
		cfg.Port = 5432
	}
	if cfg.MaxOpenConns == 0 {
		cfg.MaxOpenConns = 25
	}
	if cfg.MaxIdleConns == 0 {
		cfg.MaxIdleConns = 5
	}
	if cfg.ConnMaxLifetime == 0 {
		cfg.ConnMaxLifetime = 30 * time.Minute
	}
	if cfg.SSLMode == "" {
		cfg.SSLMode = "disable"
	}

	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host,
		cfg.Port,
		cfg.User,
		cfg.Password,
		cfg.Database,
		cfg.SSLMode,
	)

	gormConfig := &gorm.Config{
		Logger: gormlog.Default.LogMode(gormlog.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	}

	db, err := gorm.Open(postgres.Open(dsn), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("postgres: open connection: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("postgres: get underlying sql.DB: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	if log == nil {
		log = erglog.NoOp()
	}

	// 5s timeout for DB ping.
	ctx5s, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx5s); err != nil {
		return nil, fmt.Errorf("postgres: ping: %w", err)
	}

	log.Info().
		Str("database", cfg.Database).
		Str("host", cfg.Host).
		Int("port", cfg.Port).
		Int("max_open_conns", cfg.MaxOpenConns).
		Msg("postgres connected via GORM")

	return &GORMPostgresClient{db: db, log: log}, nil
}

// MySQL client types are defined in mysql.go to avoid duplicate package declarations.
