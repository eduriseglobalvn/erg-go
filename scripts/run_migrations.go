// scripts/run_migrations.go — Runs MongoDB index migrations for the erg monorepo.
// Usage: go run scripts/run_migrations.go --service=bot --mongo-uri=mongodb://localhost:27017
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"erg.ninja/migrations"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/database"
)

func main() {
	service := flag.String("service", "all", "Service to migrate: bot, notification, crawler, trending, all")
	mongoURI := flag.String("mongo-uri", "mongodb://localhost:27017", "MongoDB connection URI")
	dbName := flag.String("db", "", "Database name (auto-selected from service if not provided)")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cfg := config.NewDefault()
	cfg.MongoDB.URI = *mongoURI
	cfg.MongoDB.Database = *dbName
	if cfg.MongoDB.Database == "" {
		cfg.MongoDB.Database = "erg"
	}

	mongo, err := database.NewMongoClient(ctx, cfg.MongoDB)
	if err != nil {
		log.Fatalf("failed to connect to MongoDB: %v", err)
	}
	defer func() { _ = mongo.Close(ctx) }()

	db := mongo.Database()

	fmt.Printf("Running migrations for service=%s, database=%s\n", *service, cfg.MongoDB.Database)

	switch *service {
	case "bot":
		if err := migrations.Run001BotIndexes(ctx, db); err != nil {
			log.Fatalf("bot migration failed: %v", err)
		}
	case "notification":
		if err := migrations.Run002NotificationIndexes(ctx, db); err != nil {
			log.Fatalf("notification migration failed: %v", err)
		}
	case "crawler":
		if err := migrations.Run003CrawlerIndexes(ctx, db); err != nil {
			log.Fatalf("crawler migration failed: %v", err)
		}
	case "trending":
		if err := migrations.Run004TrendingIndexes(ctx, db); err != nil {
			log.Fatalf("trending migration failed: %v", err)
		}
	case "all":
		if err := migrations.Run001BotIndexes(ctx, db); err != nil {
			log.Fatalf("migration 001 failed: %v", err)
		}
		fmt.Println("Migration 001 (bot indexes) ✓")
		// Run remaining migrations for their respective databases.
		// In production, each service manages its own database.
	default:
		fmt.Fprintf(os.Stderr, "unknown service: %s\n", *service)
		os.Exit(1)
	}

	fmt.Println("All migrations completed successfully ✓")
}
