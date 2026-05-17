package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"erg.ninja/internal/modules/hoclieu"
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
	svc := hoclieu.NewService()
	svc.UseRepository(hoclieu.NewRepository(mongoClient), tenantID)
	if err := svc.EnsurePersistentStore(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "prepare hoclieu storage: %v\n", err)
		os.Exit(1)
	}

	taxonomy := svc.Taxonomy(ctx)
	resources, total := svc.ListResources(ctx, hoclieu.ListResourceParams{Limit: 1})
	_ = resources
	fmt.Printf("hoclieu storage ready tenant=%s programs=%d subjects=%d grades=%d bookSeries=%d topics=%d resources=%d\n",
		tenantID,
		len(taxonomy.Programs),
		len(taxonomy.Subjects),
		len(taxonomy.Grades),
		len(taxonomy.BookSeries),
		len(taxonomy.Topics),
		total,
	)
}
