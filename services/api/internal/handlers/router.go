package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/relay-forge/relay-forge/services/api/internal/auth"
	"github.com/relay-forge/relay-forge/services/api/internal/config"
	"github.com/relay-forge/relay-forge/services/api/internal/health"
	"github.com/relay-forge/relay-forge/services/api/internal/middleware"
	"github.com/relay-forge/relay-forge/services/api/internal/repository"
)

func NewRouter(cfg *config.Config, db *pgxpool.Pool) http.Handler {
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.RequestID())
	r.Use(middleware.RequestLogger())
	r.Use(chimw.Recoverer)
	r.Use(chimw.RealIP)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORS.Origins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	if cfg.RateLimit.Enabled {
		r.Use(middleware.RateLimit(cfg.RateLimit.RequestsPerSecond, cfg.RateLimit.Burst))
	}

	// Services
	jwtSvc := auth.NewJWTService(cfg.Auth.JWTSecret, cfg.Auth.AccessTTL, cfg.Auth.RefreshTTL)

	// Repositories
	userRepo := repository.NewUserRepository(db)
	sessionRepo := repository.NewSessionRepository(db)
	guildRepo := repository.NewGuildRepository(db)
	channelRepo := repository.NewChannelRepository(db)
	messageRepo := repository.NewMessageRepository(db)
	roleRepo := repository.NewRoleRepository(db)

	// Handlers
	authHandler := NewAuthHandler(userRepo, sessionRepo, jwtSvc, cfg)
	userHandler := NewUserHandler(userRepo, sessionRepo, jwtSvc, cfg, db)
	guildHandler := NewGuildHandler(guildRepo, db)
	channelHandler := NewChannelHandler(channelRepo, guildRepo)
	messageHandler := NewMessageHandler(messageRepo, channelRepo, guildRepo)
	roleHandler := NewRoleHandler(roleRepo, guildRepo)
	adminHandler := NewAdminHandler(userRepo, guildRepo)

	// Health checks
	r.Get("/healthz", health.Healthz(db))
	r.Get("/readyz", health.Readyz(db))

	// API v1
	r.Route("/api/v1", func(r chi.Router) {
		// Auth routes (public)
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", authHandler.Register)
			r.Post("/login", authHandler.Login)
			r.Post("/refresh", authHandler.Refresh)
			r.With(middleware.AuthRequired(jwtSvc)).Post("/logout", authHandler.Logout)
				r.Post("/password-reset/request", authHandler.PasswordResetRequest)
				r.Post("/password-reset/confirm", authHandler.PasswordResetConfirm)
		})

		// Authenticated routes
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthRequired(jwtSvc))

			// User routes
			r.Route("/users", func(r chi.Router) {
				r.Get("/me", userHandler.GetMe)
				r.Patch("/me", userHandler.UpdateMe)
				r.Get("/me/sessions", userHandler.ListSessions)
				r.Delete("/me/sessions/{sessionID}", userHandler.RevokeSession)
				r.Post("/me/2fa/enable", userHandler.Enable2FA)
				r.Post("/me/2fa/verify", userHandler.Verify2FA)
				r.Post("/me/2fa/disable", userHandler.Disable2FA)
			})

			// Guild routes
			r.Route("/guilds", func(r chi.Router) {
					r.Post("/", guildHandler.CreateGuild)
					r.Get("/", guildHandler.ListGuilds)
					r.Get("/{guildID}", guildHandler.GetGuild)
					r.Patch("/{guildID}", guildHandler.UpdateGuild)
					r.Delete("/{guildID}", guildHandler.DeleteGuild)
				r.Get("/{guildID}/members", guildHandler.ListMembers)
				r.Post("/{guildID}/members", guildHandler.JoinGuild)
				r.Delete("/{guildID}/members/{userID}", guildHandler.KickMember)
				r.Post("/{guildID}/invites", guildHandler.CreateInvite)
				r.Get("/{guildID}/invites", guildHandler.ListInvites)

				// Channel routes (nested under guild)
				r.Route("/{guildID}/channels", func(r chi.Router) {
						r.Post("/", channelHandler.CreateChannel)
						r.Get("/", channelHandler.ListChannels)
						r.Get("/{channelID}", channelHandler.GetChannel)
						r.Patch("/{channelID}", channelHandler.UpdateChannel)
						r.Delete("/{channelID}", channelHandler.DeleteChannel)
				})

				// Role routes (nested under guild)
				r.Route("/{guildID}/roles", func(r chi.Router) {
					r.Post("/", roleHandler.Create)
					r.Get("/", roleHandler.List)
					r.Patch("/{roleID}", roleHandler.Update)
					r.Delete("/{roleID}", roleHandler.Delete)
					r.Post("/{roleID}/members/{userID}", roleHandler.AssignRole)
					r.Delete("/{roleID}/members/{userID}", roleHandler.RemoveRole)
				})
			})

			// Message routes
			r.Route("/channels/{channelID}/messages", func(r chi.Router) {
					r.Get("/", messageHandler.ListMessages)
					r.Post("/", messageHandler.SendMessage)
					r.Get("/search", messageHandler.SearchMessages)
					r.Get("/pins", messageHandler.ListPins)
					r.Patch("/{messageID}", messageHandler.EditMessage)
					r.Delete("/{messageID}", messageHandler.DeleteMessage)
					r.Post("/{messageID}/pin", messageHandler.PinMessage)
					r.Delete("/{messageID}/pin", messageHandler.UnpinMessage)
					r.Post("/{messageID}/reactions", messageHandler.AddReaction)
					r.Delete("/{messageID}/reactions/{emoji}", messageHandler.RemoveReaction)
				})

			// Admin routes
			r.Route("/admin", func(r chi.Router) {
				r.Use(AdminOnly)
				r.Get("/users", adminHandler.ListUsers)
				r.Get("/guilds", adminHandler.ListGuilds)
				r.Delete("/users/{userID}", adminHandler.DisableUser)
				r.Delete("/guilds/{guildID}", adminHandler.DeleteGuild)
			})
		})
	})

	return r
}
