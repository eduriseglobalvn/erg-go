package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"erg.ninja/internal/modules/lms"
	"erg.ninja/pkg/config"
	"erg.ninja/pkg/database"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg := config.NewDefault()
	loader := config.NewLoader(
		config.WithConfigPaths(".", "./config"),
		config.WithFileNames("application", "config"),
	)
	if err := loader.Load(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	mongoClient, err := database.NewMongoClient(ctx, cfg.MongoDB)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect mongo: %v\n", err)
		os.Exit(1)
	}
	defer mongoClient.Close(context.Background())

	tenantID := cfg.Tenant.DefaultID
	if tenantID == "" {
		tenantID = "default"
	}
	repo := lms.NewRepository(mongoClient)
	svc := lms.NewService(repo, nil)
	if err := svc.SeedDefaultEducationUnits(ctx, tenantID); err != nil {
		fmt.Fprintf(os.Stderr, "seed lms education units: %v\n", err)
		os.Exit(1)
	}

	units, total, err := repo.ListCenters(ctx, tenantID, lms.CenterListRequestDTO{Page: 1, Limit: 100}, "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "list seeded units: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("lms education-unit seed ok tenant=%s total=%d\n", tenantID, total)
	for _, unit := range units {
		fmt.Printf("- %s %s %s\n", unit.Type, unit.Code, unit.Name)
	}
}
