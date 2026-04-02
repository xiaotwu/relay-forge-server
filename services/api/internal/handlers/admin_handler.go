package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/relay-forge/relay-forge/services/api/internal/middleware"
	"github.com/relay-forge/relay-forge/services/api/internal/repository"
)

type AdminHandler struct {
	users  *repository.UserRepository
	guilds *repository.GuildRepository
}

func NewAdminHandler(users *repository.UserRepository, guilds *repository.GuildRepository) *AdminHandler {
	return &AdminHandler{users: users, guilds: guilds}
}

func AdminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		roles := middleware.GetUserRoles(r.Context())
		for _, role := range roles {
			if role == "admin" {
				next.ServeHTTP(w, r)
				return
			}
		}
		respondError(w, forbiddenErr("admin access required"))
	})
}

func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	limit, offset := parsePagination(r)
	users, total, err := h.users.List(r.Context(), limit, offset)
	if err != nil {
		respondError(w, err)
		return
	}
	respondJSONWithMeta(w, http.StatusOK, users, map[string]int{"total": total, "limit": limit, "offset": offset})
}

func (h *AdminHandler) ListGuilds(w http.ResponseWriter, r *http.Request) {
	guilds, err := h.guilds.ListAll(r.Context(), 100, 0)
	if err != nil {
		respondError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, guilds)
}

func (h *AdminHandler) DisableUser(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUUID(chi.URLParam(r, "userID"))
	if err != nil {
		respondError(w, err)
		return
	}

	if err := h.users.SoftDelete(r.Context(), userID); err != nil {
		respondError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "disabled"})
}

func (h *AdminHandler) DeleteGuild(w http.ResponseWriter, r *http.Request) {
	guildID, err := parseUUID(chi.URLParam(r, "guildID"))
	if err != nil {
		respondError(w, err)
		return
	}

	if err := h.guilds.SoftDelete(r.Context(), guildID); err != nil {
		respondError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
