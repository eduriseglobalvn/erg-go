// Package main is the entry point for the bot-service.
package main

// This file is a placeholder for Google Wire dependency injection.
// When migrating to Wire, run: wire gen
// Until then, dependencies are initialized directly in main.go.

import (
	"erg.ninja/pkg/cache"
	"erg.ninja/pkg/database"
)

// wire.go provides type declarations for Wire code generation.
// Currently unused — dependencies are initialized in main.go.

// Dependencies is a placeholder struct for Wire.
type Dependencies struct {
	Mongo *database.MongoClient
	Redis *cache.RedisClient
}

// InitializeDependencies is called by Wire to construct the dependency graph.
func InitializeDependencies() (*Dependencies, error) {
	return nil, nil
}
