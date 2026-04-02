package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/relay-forge/relay-forge/services/media/internal/config"
	"github.com/relay-forge/relay-forge/services/media/internal/handlers"
	"github.com/relay-forge/relay-forge/services/media/internal/storage"
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
	log.Info().Str("version", version).Str("build_time", buildTime).Msg("starting RelayForge Media")

	store, err := storage.NewS3Store(cfg.S3)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize storage")
	}

	r := chi.NewRouter()

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok"}`)
	})

	uploadHandler := handlers.NewUploadHandler(store, cfg)
	livekitHandler := handlers.NewLiveKitHandler(cfg)

	r.Route("/api/v1/media", func(r chi.Router) {
		r.Post("/upload/presign", uploadHandler.CreatePresignedUpload)
		r.Post("/upload/complete", uploadHandler.CompleteUpload)
		r.Get("/files/{fileID}", uploadHandler.GetFile)
	})

	r.Route("/api/v1/voice", func(r chi.Router) {
		r.Post("/token", livekitHandler.GenerateToken)
		r.Post("/rooms", livekitHandler.CreateRoom)
		r.Delete("/rooms/{roomName}", livekitHandler.DeleteRoom)
		r.Get("/rooms", livekitHandler.ListRooms)
	})

	srv := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Info().Str("addr", srv.Addr).Msg("media server listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server error")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("shutting down media server")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("server forced to shutdown")
	}
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
