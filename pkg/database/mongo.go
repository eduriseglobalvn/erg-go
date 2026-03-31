// Package database provides MongoDB and MySQL client wrappers with production-grade
// connection pooling, health checks, and graceful shutdown.
package database

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
	"go.mongodb.org/mongo-driver/v2/mongo/writeconcern"

	"erg.ninja/pkg/config"
	"erg.ninja/pkg/logger"
)

// MongoClient wraps a mongo.Client with configuration and lifecycle management.
type MongoClient struct {
	client   *mongo.Client
	database *mongo.Database
	cfg      config.MongoDBConfig
	log      *logger.Logger
	mu       sync.RWMutex
	closed   bool
}

// MongoOption configures a MongoClient.
type MongoOption func(*MongoClient)

// WithMongoLogger sets the logger for the MongoDB client.
func WithMongoLogger(log *logger.Logger) MongoOption {
	return func(m *MongoClient) {
		m.log = log
	}
}

// NewMongoClient creates a new MongoDB client from the given configuration.
// It applies all pool settings, authentication, TLS, and retry policies.
func NewMongoClient(ctx context.Context, cfg config.MongoDBConfig, opts ...MongoOption) (*MongoClient, error) {
	m := &MongoClient{cfg: cfg, log: logger.NoOp()}
	for _, o := range opts {
		o(m)
	}

	uri, err := buildURI(cfg)
	if err != nil {
		return nil, fmt.Errorf("mongo: build connection URI: %w", err)
	}

	clientOpts := options.Client().
		ApplyURI(uri).
		SetMaxPoolSize(cfg.MaxPoolSize).
		SetMinPoolSize(cfg.MinPoolSize).
		SetServerSelectionTimeout(cfg.ServerSelectionTimeout).
		SetConnectTimeout(cfg.ConnectTimeout).
		SetTimeout(cfg.SocketTimeout).
		SetRetryWrites(true).
		SetRetryReads(true).
		SetAppName("erg-service")

	// Configure read preference.
	rp, err := parseReadPreference(cfg.ReadPreference)
	if err != nil {
		return nil, fmt.Errorf("mongo: parse read preference: %w", err)
	}
	clientOpts.SetReadPreference(rp)

	// Configure write concern for durability.
	clientOpts.SetWriteConcern(writeconcern.Majority())

	// Configure TLS if scheme is mongodb+srv.
	if strings.HasPrefix(uri, "mongodb+srv") {
		clientOpts.SetTLSConfig(&tls.Config{
			MinVersion: tls.VersionTLS12,
		})
	}

	m.client, err = mongo.Connect(clientOpts)
	if err != nil {
		return nil, fmt.Errorf("mongo: connect: %w", err)
	}

	// Verify the connection.
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := m.client.Ping(pingCtx, rp); err != nil {
		_ = m.client.Disconnect(context.Background())
		return nil, fmt.Errorf("mongo: ping: %w", err)
	}

	m.database = m.client.Database(cfg.Database)
	m.log.Info().Str("database", cfg.Database).Msg("mongodb connected")

	return m, nil
}

// Client returns the underlying mongo.Client. Thread-safe.
func (m *MongoClient) Client() *mongo.Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.client
}

// Database returns the database handle for the configured database name.
func (m *MongoClient) Database() *mongo.Database {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.database
}

// Collection returns a collection handle from the configured database.
func (m *MongoClient) Collection(name string) *mongo.Collection {
	return m.Database().Collection(name)
}

// Ping checks MongoDB connectivity. Returns nil on success.
func (m *MongoClient) Ping(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rp, _ := parseReadPreference(m.cfg.ReadPreference)
	return m.client.Ping(ctx, rp)
}

// Close disconnects the MongoDB client gracefully within the context deadline.
func (m *MongoClient) Close(ctx context.Context) error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true
	m.mu.Unlock()

	m.log.Info().Msg("mongodb disconnecting")
	_ = m.client.Disconnect(ctx)
	return nil
}

// IsDuplicateKey checks if the error is a MongoDB duplicate key error (code 11000).
func IsDuplicateKey(err error) bool {
	if err == nil {
		return false
	}
	var de mongo.WriteException
	if errors.As(err, &de) {
		for _, we := range de.WriteErrors {
			if we.Code == 11000 {
				return true
			}
		}
	}
	return strings.Contains(err.Error(), "duplicate key")
}

// buildURI returns the URI from the config, or constructs one from individual fields.
func buildURI(cfg config.MongoDBConfig) (string, error) {
	if cfg.URI != "" {
		return cfg.URI, nil
	}
	return "", fmt.Errorf("mongo: uri is required")
}

// parseReadPreference converts a string read preference to a *readpref.ReadPref.
func parseReadPreference(s string) (*readpref.ReadPref, error) {
	switch strings.ToLower(s) {
	case "primary":
		return readpref.Primary(), nil
	case "secondarypreferred", "secondary_preferred":
		return readpref.SecondaryPreferred(), nil
	case "secondary":
		return readpref.Secondary(), nil
	case "primarypreferred", "primary_preferred":
		return readpref.PrimaryPreferred(), nil
	case "nearest":
		return readpref.Nearest(), nil
	default:
		return readpref.SecondaryPreferred(), nil
	}
}
