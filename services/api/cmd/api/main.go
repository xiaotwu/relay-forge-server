package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/relay-forge/relay-forge/services/api/internal/config"
	"github.com/relay-forge/relay-forge/services/api/internal/database"
	"github.com/relay-forge/relay-forge/services/api/internal/handlers"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	setupLogger(cfg.LogLevel, cfg.LogFormat)
	log.Info().Str("version", version).Str("build_time", buildTime).Msg("starting RelayForge API")

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "migrate":
			runMigrate(cfg, os.Args[2:])
			return
		case "seed":
			runSeed(cfg)
			return
		}
	}

	db, err := database.Connect(cfg.Database)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer db.Close()

	router := handlers.NewRouter(cfg, db)

	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info().Str("addr", srv.Addr).Msg("API server listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server error")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("shutting down server")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("server forced to shutdown")
	}
	log.Info().Msg("server stopped")
}

func setupLogger(level, format string) {
	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(lvl)

	if format == "text" || format == "console" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})
	}
}

func runMigrate(cfg *config.Config, args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: api migrate [up|down]")
		os.Exit(1)
	}
	db, err := database.Connect(cfg.Database)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer db.Close()

	switch args[0] {
	case "up":
		if err := database.MigrateUp(db); err != nil {
			log.Fatal().Err(err).Msg("migration up failed")
		}
		log.Info().Msg("migrations applied successfully")
	case "down":
		if err := database.MigrateDown(db); err != nil {
			log.Fatal().Err(err).Msg("migration down failed")
		}
		log.Info().Msg("migration rolled back successfully")
	default:
		fmt.Printf("unknown migrate command: %s\n", args[0])
		os.Exit(1)
	}
}

func runSeed(cfg *config.Config) {
	db, err := database.Connect(cfg.Database)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer db.Close()

	if err := database.Seed(db); err != nil {
		log.Fatal().Err(err).Msg("seeding failed")
	}
	log.Info().Msg("database seeded successfully")
}
