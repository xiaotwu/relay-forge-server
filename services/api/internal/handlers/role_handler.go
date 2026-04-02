package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/relay-forge/relay-forge/services/api/internal/middleware"
	"github.com/relay-forge/relay-forge/services/api/internal/models"
	"github.com/relay-forge/relay-forge/services/api/internal/repository"
)

type RoleHandler struct {
	roles  *repository.RoleRepository
	guilds *repository.GuildRepository
}

func NewRoleHandler(roles *repository.RoleRepository, guilds *repository.GuildRepository) *RoleHandler {
	return &RoleHandler{roles: roles, guilds: guilds}
}

type createRoleRequest struct {
	Name        string `json:"name"`
	Color       string `json:"color"`
	Position    int    `json:"position"`
	Permissions int64  `json:"permissions"`
	Mentionable bool   `json:"mentionable"`
	Hoisted     bool   `json:"hoisted"`
}

type updateRoleRequest struct {
	Name        *string `json:"name"`
	Color       *string `json:"color"`
	Position    *int    `json:"position"`
	Permissions *int64  `json:"permissions"`
	Mentionable *bool   `json:"mentionable"`
	Hoisted     *bool   `json:"hoisted"`
}

func (h *RoleHandler) Create(w http.ResponseWriter, r *http.Request) {
	guildID, err := parseUUID(chi.URLParam(r, "guildID"))
	if err != nil {
		respondError(w, err)
		return
	}

	userID := middleware.GetUserID(r.Context())
	if err := h.requireManageRoles(r, guildID, userID); err != nil {
		respondError(w, err)
		return
	}

	var req createRoleRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, err)
		return
	}

	role := &models.Role{
		ID:            uuid.New(),
		GuildID:       guildID,
		Name:          req.Name,
		Color:         req.Color,
		Position:      req.Position,
		Permissions:   req.Permissions,
		IsMentionable: req.Mentionable,
	}

	if err := h.roles.Create(r.Context(), role); err != nil {
		respondError(w, err)
		return
	}
	respondJSON(w, http.StatusCreated, role)
}

func (h *RoleHandler) List(w http.ResponseWriter, r *http.Request) {
	guildID, err := parseUUID(chi.URLParam(r, "guildID"))
	if err != nil {
		respondError(w, err)
		return
	}

	userID := middleware.GetUserID(r.Context())
	if ok, _ := h.guilds.IsMember(r.Context(), guildID, userID); !ok {
		respondError(w, forbiddenErr("not a guild member"))
		return
	}

	roles, err := h.roles.ListByGuild(r.Context(), guildID)
	if err != nil {
		respondError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, roles)
}

func (h *RoleHandler) Update(w http.ResponseWriter, r *http.Request) {
	guildID, err := parseUUID(chi.URLParam(r, "guildID"))
	if err != nil {
		respondError(w, err)
		return
	}

	roleID, err := parseUUID(chi.URLParam(r, "roleID"))
	if err != nil {
		respondError(w, err)
		return
	}

	userID := middleware.GetUserID(r.Context())
	if err := h.requireManageRoles(r, guildID, userID); err != nil {
		respondError(w, err)
		return
	}

	role, err := h.roles.GetByID(r.Context(), roleID)
	if err != nil {
		respondError(w, err)
		return
	}

	var req updateRoleRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, err)
		return
	}

	if req.Name != nil {
		role.Name = *req.Name
	}
	if req.Color != nil {
		role.Color = *req.Color
	}
	if req.Position != nil {
		role.Position = *req.Position
	}
	if req.Permissions != nil {
		role.Permissions = *req.Permissions
	}
	if req.Mentionable != nil {
		role.IsMentionable = *req.Mentionable
	}

	if err := h.roles.Update(r.Context(), role); err != nil {
		respondError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, role)
}

func (h *RoleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	guildID, err := parseUUID(chi.URLParam(r, "guildID"))
	if err != nil {
		respondError(w, err)
		return
	}

	roleID, err := parseUUID(chi.URLParam(r, "roleID"))
	if err != nil {
		respondError(w, err)
		return
	}

	userID := middleware.GetUserID(r.Context())
	if err := h.requireManageRoles(r, guildID, userID); err != nil {
		respondError(w, err)
		return
	}

	if err := h.roles.Delete(r.Context(), roleID); err != nil {
		respondError(w, err)
		return
	}
	respondJSON(w, http.StatusNoContent, nil)
}

func (h *RoleHandler) AssignRole(w http.ResponseWriter, r *http.Request) {
	guildID, err := parseUUID(chi.URLParam(r, "guildID"))
	if err != nil {
		respondError(w, err)
		return
	}

	roleID, err := parseUUID(chi.URLParam(r, "roleID"))
	if err != nil {
		respondError(w, err)
		return
	}

	targetUserID, err := parseUUID(chi.URLParam(r, "userID"))
	if err != nil {
		respondError(w, err)
		return
	}

	userID := middleware.GetUserID(r.Context())
	if err := h.requireManageRoles(r, guildID, userID); err != nil {
		respondError(w, err)
		return
	}

	if err := h.roles.AssignRole(r.Context(), guildID, targetUserID, roleID); err != nil {
		respondError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "assigned"})
}

func (h *RoleHandler) RemoveRole(w http.ResponseWriter, r *http.Request) {
	guildID, err := parseUUID(chi.URLParam(r, "guildID"))
	if err != nil {
		respondError(w, err)
		return
	}

	roleID, err := parseUUID(chi.URLParam(r, "roleID"))
	if err != nil {
		respondError(w, err)
		return
	}

	targetUserID, err := parseUUID(chi.URLParam(r, "userID"))
	if err != nil {
		respondError(w, err)
		return
	}

	userID := middleware.GetUserID(r.Context())
	if err := h.requireManageRoles(r, guildID, userID); err != nil {
		respondError(w, err)
		return
	}

	if err := h.roles.RemoveRole(r.Context(), guildID, targetUserID, roleID); err != nil {
		respondError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

const permManageRoles int64 = 1 << 4

func (h *RoleHandler) requireManageRoles(r *http.Request, guildID, userID uuid.UUID) error {
	guild, err := h.guilds.GetByID(r.Context(), guildID)
	if err != nil {
		return err
	}
	if guild.OwnerID == userID {
		return nil
	}
	perms, err := h.roles.GetUserPermissions(r.Context(), guildID, userID)
	if err != nil {
		return err
	}
	if perms&permManageRoles == 0 {
		return forbiddenErr("missing manage_roles permission")
	}
	return nil
}
