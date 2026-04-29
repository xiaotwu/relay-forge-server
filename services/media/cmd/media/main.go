package main

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/relay-forge/relay-forge/services/media/internal/acl"
	"github.com/relay-forge/relay-forge/services/media/internal/config"
	"github.com/relay-forge/relay-forge/services/media/internal/handlers"
	"github.com/relay-forge/relay-forge/services/media/internal/storage"
)

var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	loadDotEnv()

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

	dbConfig, err := pgxpool.ParseConfig(cfg.Database.DSN())
	if err != nil {
		log.Fatal().Err(err).Msg("failed to parse database config")
	}
	if cfg.Database.MaxOpenConns > 0 {
		dbConfig.MaxConns = int32(cfg.Database.MaxOpenConns)
	}
	if cfg.Database.MaxIdleConns > 0 {
		dbConfig.MinConns = int32(cfg.Database.MaxIdleConns)
	}
	dbConfig.MaxConnLifetime = cfg.Database.ConnMaxLifetime

	db, err := pgxpool.NewWithConfig(context.Background(), dbConfig)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect database")
	}
	defer db.Close()

	if err := db.Ping(context.Background()); err != nil {
		log.Fatal().Err(err).Msg("failed to ping database")
	}

	aclStore := acl.NewPostgresStore(db)
	aclService := acl.New(aclStore)

	r := chi.NewRouter()
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORS.Origins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := fmt.Fprint(w, `{"status":"ok"}`); err != nil {
			log.Warn().Err(err).Msg("failed to write health response")
		}
	})

	uploadHandler := handlers.NewUploadHandler(store, cfg, aclStore, aclService)
	livekitHandler := handlers.NewLiveKitHandler(cfg)

	r.Route("/api/v1/media", func(r chi.Router) {
		r.With(handlers.AuthRequired(cfg.Auth.JWTSecret)).Post("/upload/presign", uploadHandler.CreatePresignedUpload)
		r.With(handlers.AuthRequired(cfg.Auth.JWTSecret)).Post("/upload/complete", uploadHandler.CompleteUpload)
		r.With(handlers.OptionalAuth(cfg.Auth.JWTSecret)).Get("/files/{fileID}", uploadHandler.GetFile)
	})

	r.Route("/api/v1/voice", func(r chi.Router) {
		r.Use(handlers.AuthRequired(cfg.Auth.JWTSecret))
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

func loadDotEnv() {
	path, ok := findDotEnv()
	if !ok {
		return
	}

	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Debug().Err(err).Str("path", path).Msg("failed to close dotenv file")
		}
	}()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}

		key = strings.TrimSpace(key)
		if key == "" || os.Getenv(key) != "" {
			continue
		}

		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		_ = os.Setenv(key, value)
	}
}

func findDotEnv() (string, bool) {
	dir, err := os.Getwd()
	if err != nil {
		return "", false
	}

	for {
		candidate := filepath.Join(dir, ".env")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, true
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}
