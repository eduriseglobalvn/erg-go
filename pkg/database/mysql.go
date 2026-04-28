// Package database provides MySQL/PostgreSQL support via GORM v2 and pgx v5.
// MongoDB uses the go.mongodb.org/mongo-driver directly (see mongo.go).
// Note: gorm.io/driver/mongodb does NOT exist — GORM v2 officially supports
// only PostgreSQL, MySQL, SQLite, SQL Server, TiDB, Clickhouse, GaussDB, Oracle.
package database

import (
	"context"
	"fmt"
	"time"

	"erg.ninja/pkg/logger"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlog "gorm.io/gorm/logger"
)

// MySQLConfig holds MySQL connection settings for GORMMySQLClient.
type MySQLConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	Database        string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// GORMMySQLClient wraps a GORM *gorm.DB connected to MySQL via gorm.io/driver/mysql.
type GORMMySQLClient struct {
	db  *gorm.DB
	cfg MySQLConfig
	log *logger.Logger
}

// MySQLOption configures a GORMMySQLClient.
type MySQLOption func(*GORMMySQLClient)

// WithMySQLLogger sets the logger for the GORMMySQLClient.
func WithMySQLLogger(log *logger.Logger) MySQLOption {
	return func(g *GORMMySQLClient) { g.log = log }
}

// NewGORMMySQLClient creates a new GORM MySQL connection from MySQLConfig.
// Uses gorm.io/driver/mysql under the hood.
// A 5-second context timeout is applied for the initial connection and ping.
func NewGORMMySQLClient(ctx context.Context, cfg MySQLConfig, opts ...MySQLOption) (*GORMMySQLClient, error) {
	g := &GORMMySQLClient{cfg: cfg, log: logger.NoOp()}
	for _, o := range opts {
		o(g)
	}

	// Apply defaults.
	if g.cfg.Port == 0 {
		g.cfg.Port = 3306
	}
	if g.cfg.MaxOpenConns == 0 {
		g.cfg.MaxOpenConns = 25
	}
	if g.cfg.MaxIdleConns == 0 {
		g.cfg.MaxIdleConns = 10
	}
	if g.cfg.ConnMaxLifetime == 0 {
		g.cfg.ConnMaxLifetime = 5 * time.Minute
	}

	dsn := buildMySQLDSN(g.cfg)

	gormConfig := &gorm.Config{
		// Suppress GORM's default logger — we use zerolog instead.
		Logger: gormlog.Default.LogMode(gormlog.Silent),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	}

	db, err := gorm.Open(mysql.Open(dsn), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("mysql: open connection: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("mysql: get underlying sql.DB: %w", err)
	}

	// Connection pool settings.
	sqlDB.SetMaxOpenConns(g.cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(g.cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(g.cfg.ConnMaxLifetime)

	// Verify connectivity with a short timeout.
	ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx2); err != nil {
		return nil, fmt.Errorf("mysql: ping: %w", err)
	}

	g.db = db
	g.log.Info().
		Str("database", g.cfg.Database).
		Str("host", g.cfg.Host).
		Int("port", g.cfg.Port).
		Int("max_open_conns", g.cfg.MaxOpenConns).
		Msg("mysql connected via GORM")

	return g, nil
}

// DB returns the underlying GORM *gorm.DB instance. Use for all GORM operations.
func (g *GORMMySQLClient) DB() *gorm.DB {
	return g.db
}

// Close shuts down the MySQL connection pool.
func (g *GORMMySQLClient) Close() error {
	sqlDB, err := g.db.DB()
	if err != nil {
		return fmt.Errorf("mysql: get underlying sql.DB: %w", err)
	}
	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("mysql: close: %w", err)
	}
	g.log.Info().Msg("mysql connection closed")
	return nil
}

// Ping checks database connectivity.
func (g *GORMMySQLClient) Ping(ctx context.Context) error {
	sqlDB, err := g.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

// AutoMigrate runs GORM AutoMigrate for the given models.
func (g *GORMMySQLClient) AutoMigrate(dst ...interface{}) error {
	return g.db.AutoMigrate(dst...)
}

// buildMySQLDSN builds a MySQL DSN string from MySQLConfig.
func buildMySQLDSN(cfg MySQLConfig) string {
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Database,
	)
}
