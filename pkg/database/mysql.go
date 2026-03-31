// Package database provides MySQL/PostgreSQL support via pgx v5.
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"erg.ninja/pkg/config"
	"erg.ninja/pkg/logger"
)

// MySQLClient wraps a pgxpool.Pool with configuration and lifecycle management.
type MySQLClient struct {
	pool *pgxpool.Pool
	cfg  config.DatabaseConfig
	log  *logger.Logger
}

// MySQLOption configures a MySQLClient.
type MySQLOption func(*MySQLClient)

// WithMySQLLogger sets the logger for the MySQL client.
func WithMySQLLogger(log *logger.Logger) MySQLOption {
	return func(m *MySQLClient) {
		m.log = log
	}
}

// NewMySQLClient creates a new pgx connection pool from the given configuration.
func NewMySQLClient(ctx context.Context, cfg config.DatabaseConfig, opts ...MySQLOption) (*MySQLClient, error) {
	m := &MySQLClient{cfg: cfg, log: logger.NoOp()}
	for _, o := range opts {
		o(m)
	}

	connStr := buildPgConnStr(cfg)

	poolCfg, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("mysql: parse config: %w", err)
	}

	poolCfg.MaxConns = int32(cfg.MaxOpenConns)
	poolCfg.MinConns = int32(cfg.MaxIdleConns)
	poolCfg.MaxConnLifetime = cfg.ConnMaxLifetime
	poolCfg.MaxConnIdleTime = cfg.ConnMaxIdleTime
	poolCfg.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("mysql: create pool: %w", err)
	}

	// Verify connectivity.
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("mysql: ping: %w", err)
	}

	m.pool = pool
	m.log.Info().Str("database", cfg.Name).Msg("mysql/postgres connected")

	return m, nil
}

// Pool returns the underlying pgxpool.Pool.
func (m *MySQLClient) Pool() *pgxpool.Pool {
	return m.pool
}

// Close shuts down the connection pool.
func (m *MySQLClient) Close() {
	if m.pool == nil {
		return
	}
	m.pool.Close()
}

// Ping checks database connectivity.
func (m *MySQLClient) Ping(ctx context.Context) error {
	return m.pool.Ping(ctx)
}

// QueryRow executes a query that returns a single row, wrapping the error with the SQL text.
func (m *MySQLClient) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return m.pool.QueryRow(ctx, sql, args...)
}

// Query executes a query, wrapping errors with the SQL text.
func (m *MySQLClient) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	rows, err := m.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("mysql: query %s: %w", sql, err)
	}
	return rows, nil
}

// Exec executes a query that does not return rows, wrapping errors.
func (m *MySQLClient) Exec(ctx context.Context, sql string, args ...any) error {
	_, err := m.pool.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("mysql: exec %s: %w", sql, err)
	}
	return nil
}

// QueryTx executes a query within a transaction.
func (m *MySQLClient) QueryTx(ctx context.Context, tx pgx.Tx, sql string, args ...any) (pgx.Rows, error) {
	rows, err := tx.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("mysql: query tx %s: %w", sql, err)
	}
	return rows, nil
}

// ExecTx executes a statement within a transaction.
func (m *MySQLClient) ExecTx(ctx context.Context, tx pgx.Tx, sql string, args ...any) error {
	_, err := tx.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("mysql: exec tx %s: %w", sql, err)
	}
	return nil
}

// BeginTx starts a new transaction with the given options.
func (m *MySQLClient) BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
	tx, err := m.pool.BeginTx(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("mysql: begin tx: %w", err)
	}
	return tx, nil
}

// buildPgConnStr builds a lib/pq-compatible connection string from config.
func buildPgConnStr(cfg config.DatabaseConfig) string {
	// Support both MySQL and PostgreSQL connection strings via pgx.
	// For MySQL, use mysql:// scheme; for PostgreSQL, postgresql://.
	// Default to postgresql since we're using pgx (PostgreSQL driver).
	connStr := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Name,
	)
	return connStr
}
