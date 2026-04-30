// Package main is a standalone runtime plugin loader for erg-go modules.
// Requires CGO_ENABLED=1 and identical Go version as the host binary.
//
// Usage:
//
//	plugin-server --dir ./plugins --list        # list available plugins
//	plugin-server --dir ./plugins --load bot   # load specific plugin
//	plugin-server --dir ./plugins              # load all plugins and run HTTP health server
//
// This binary is separate from cmd/server to keep CGO concerns isolated.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"erg.ninja/pkg/logger"
	"erg.ninja/pkg/plugin"
)

var (
	flagPluginDir = flag.String("dir", "./plugins", "directory containing .so plugin files")
	flagList      = flag.Bool("list", false, "list available plugins in --dir and exit")
	flagLoad      = flag.String("load", "", "load a specific plugin by name and exit")
	flagHealth    = flag.Bool("health", false, "run HTTP health server on :8081")
)

func main() {
	flag.Parse()

	log := logger.New(logger.WithServiceName("plugin-server"))

	if *flagList {
		listPlugins(log)
		return
	}

	if *flagLoad != "" {
		loadPlugin(log, *flagLoad)
		return
	}

	// Default: load all plugins
	runServer(log)
}

func listPlugins(log *logger.Logger) {
	entries, err := os.ReadDir(*flagPluginDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading plugin dir %q: %v\n", *flagPluginDir, err)
		os.Exit(1)
	}

	fmt.Println("Available plugins in", *flagPluginDir+":")
	found := false
	for _, e := range entries {
		if e.IsDir() || len(e.Name()) < 4 || e.Name()[:3] != "erg" || e.Name()[len(e.Name())-3:] != ".so" {
			continue
		}
		name := e.Name()[3 : len(e.Name())-3] // strip "erg-" prefix and ".so" suffix
		fmt.Printf("  • erg-%s.so → %s\n", name, name)
		found = true
	}
	if !found {
		fmt.Println("  (no .so plugins found)")
	}
}

func loadPlugin(log *logger.Logger, name string) {
	loader := plugin.NewLoader(*flagPluginDir)
	mod, err := loader.Load(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading plugin %q: %v\n", name, err)
		os.Exit(1)
	}
	log.Info().Str("module", mod.Name()).Msg("plugin: loaded successfully")
	_ = mod.Setup()
	_ = mod.Stop(context.Background())
	fmt.Printf("Plugin %q loaded and verified OK\n", name)
}

func runServer(log *logger.Logger) {
	loader := plugin.NewLoader(*flagPluginDir)
	mods, errs := loader.LoadAll()
	if len(mods) == 0 && len(errs) > 0 {
		fmt.Fprintf(os.Stderr, "No plugins loaded, errors:\n")
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "  • %v\n", e)
		}
		os.Exit(1)
	}

	for _, m := range mods {
		log.Info().Str("module", m.Name()).Msg("plugin-server: module loaded")
		_ = m.Setup()
		defer func(mod plugin.Module) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = mod.Stop(ctx)
		}(m)
	}

	if *flagHealth {
		go runHealthServer(log, len(mods))
	}

	log.Info().Int("count", len(mods)).Msg("plugin-server: ready")
	waitForSignal(log)
}

func runHealthServer(log *logger.Logger, n int) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "OK — %d plugin(s) loaded\n", n)
	})
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "# plugin server metrics placeholder")
	})
	addr := ":8081"
	log.Info().Str("addr", addr).Msg("plugin-server: health endpoint starting")
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error().Err(err).Msg("plugin-server: health endpoint stopped")
	}
}

func waitForSignal(log *logger.Logger) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Info().Str("signal", sig.String()).Msg("plugin-server: received shutdown signal")
}
