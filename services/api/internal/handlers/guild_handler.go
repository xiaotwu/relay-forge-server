package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/relay-forge/relay-forge/services/api/internal/errors"
	"github.com/relay-forge/relay-forge/services/api/internal/middleware"
	"github.com/relay-forge/relay-forge/services/api/internal/models"
	"github.com/relay-forge/relay-forge/services/api/internal/repository"
)

// GuildHandler handles guild-related endpoints.
type GuildHandler struct {
	guildRepo *repository.GuildRepository
	pool      *pgxpool.Pool
}

// NewGuildHandler creates a new GuildHandler.
func NewGuildHandler(guildRepo *repository.GuildRepository, pool *pgxpool.Pool) *GuildHandler {
	return &GuildHandler{
		guildRepo: guildRepo,
		pool:      pool,
	}
}

// -- Request / Response types ------------------------------------------------

type createGuildRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	IsPublic    bool    `json:"is_public"`
}

type updateGuildRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	IconURL     *string `json:"icon_url,omitempty"`
	BannerURL   *string `json:"banner_url,omitempty"`
	IsPublic    *bool   `json:"is_public,omitempty"`
}

type joinGuildRequest struct {
	InviteCode string `json:"invite_code"`
}

type createInviteRequest struct {
	MaxUses    *int `json:"max_uses,omitempty"`
	MaxAgeSecs *int `json:"max_age_secs,omitempty"`
}

type inviteResponse struct {
	Code      string     `json:"code"`
	GuildID   uuid.UUID  `json:"guild_id"`
	InviterID uuid.UUID  `json:"inviter_id"`
	MaxUses   *int       `json:"max_uses,omitempty"`
	UseCount  int        `json:"use_count"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// -- Handlers ----------------------------------------------------------------

// CreateGuild creates a new guild. The authenticated user becomes the owner.
func (h *GuildHandler) CreateGuild(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == uuid.Nil {
		respondError(w, apperrors.Unauthorized("not authenticated"))
		return
	}

	var req createGuildRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, err)
		return
	}

	if req.Name == "" {
		respondError(w, apperrors.Validation("name is required", nil))
		return
	}

	now := time.Now()
	guild := &models.Guild{
		ID:          uuid.New(),
		Name:        req.Name,
		Description: req.Description,
		OwnerID:     userID,
		IsPublic:    req.IsPublic,
		MemberCount: 1,
		MaxMembers:  500,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := h.guildRepo.Create(r.Context(), guild); err != nil {
		respondError(w, err)
		return
	}

	// Add the owner as a member.
	if err := h.guildRepo.AddMember(r.Context(), guild.ID, userID, ""); err != nil {
		respondError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, guild)
}

// ListGuilds returns the guilds the current user is a member of.
func (h *GuildHandler) ListGuilds(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == uuid.Nil {
		respondError(w, apperrors.Unauthorized("not authenticated"))
		return
	}

	guilds, err := h.guildRepo.ListForUser(r.Context(), userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, guilds)
}

// GetGuild returns details of a specific guild.
func (h *GuildHandler) GetGuild(w http.ResponseWriter, r *http.Request) {
	guildID, err := parseUUID(chi.URLParam(r, "guildID"))
	if err != nil {
		respondError(w, err)
		return
	}

	guild, err := h.guildRepo.GetByID(r.Context(), guildID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, guild)
}

// UpdateGuild updates guild details. Only the owner or admin can update.
func (h *GuildHandler) UpdateGuild(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	guildID, err := parseUUID(chi.URLParam(r, "guildID"))
	if err != nil {
		respondError(w, err)
		return
	}

	guild, err := h.guildRepo.GetByID(r.Context(), guildID)
	if err != nil {
		respondError(w, err)
		return
	}

	if guild.OwnerID != userID {
		respondError(w, apperrors.Forbidden("only the guild owner can update the guild"))
		return
	}

	var req updateGuildRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, err)
		return
	}

	if req.Name != nil {
		guild.Name = *req.Name
	}
	if req.Description != nil {
		guild.Description = req.Description
	}
	if req.IconURL != nil {
		guild.IconURL = req.IconURL
	}
	if req.BannerURL != nil {
		guild.BannerURL = req.BannerURL
	}
	if req.IsPublic != nil {
		guild.IsPublic = *req.IsPublic
	}

	if err := h.guildRepo.Update(r.Context(), guild); err != nil {
		respondError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, guild)
}

// DeleteGuild deletes a guild. Only the owner can delete.
func (h *GuildHandler) DeleteGuild(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	guildID, err := parseUUID(chi.URLParam(r, "guildID"))
	if err != nil {
		respondError(w, err)
		return
	}

	guild, err := h.guildRepo.GetByID(r.Context(), guildID)
	if err != nil {
		respondError(w, err)
		return
	}

	if guild.OwnerID != userID {
		respondError(w, apperrors.Forbidden("only the guild owner can delete the guild"))
		return
	}

	if err := h.guildRepo.SoftDelete(r.Context(), guildID); err != nil {
		respondError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListMembers returns the members of a guild.
func (h *GuildHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	guildID, err := parseUUID(chi.URLParam(r, "guildID"))
	if err != nil {
		respondError(w, err)
		return
	}

	limit, offset := parsePagination(r)
	members, total, err := h.guildRepo.ListMembers(r.Context(), guildID, limit, offset)
	if err != nil {
		respondError(w, err)
		return
	}

	respondJSONWithMeta(w, http.StatusOK, members, map[string]int{"total": total})
}

// JoinGuild allows a user to join a guild via invite code.
func (h *GuildHandler) JoinGuild(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	guildID, err := parseUUID(chi.URLParam(r, "guildID"))
	if err != nil {
		respondError(w, err)
		return
	}

	var req joinGuildRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, err)
		return
	}

	if req.InviteCode == "" {
		respondError(w, apperrors.Validation("invite_code is required", nil))
		return
	}

	// Validate the invite code.
	var inviteGuildID uuid.UUID
	var maxUses *int
	var useCount int
	var expiresAt *time.Time
	err = h.pool.QueryRow(r.Context(), `
		SELECT guild_id, max_uses, use_count, expires_at
		FROM invites
		WHERE code = $1 AND type = 'guild'`,
		req.InviteCode,
	).Scan(&inviteGuildID, &maxUses, &useCount, &expiresAt)
	if err != nil {
		respondError(w, apperrors.NotFound("invite not found or expired"))
		return
	}

	if inviteGuildID != guildID {
		respondError(w, apperrors.Validation("invite code does not match this guild", nil))
		return
	}

	if expiresAt != nil && time.Now().After(*expiresAt) {
		respondError(w, apperrors.Validation("invite has expired", nil))
		return
	}

	if maxUses != nil && useCount >= *maxUses {
		respondError(w, apperrors.Validation("invite has reached its maximum uses", nil))
		return
	}

	// Check if already a member.
	isMember, err := h.guildRepo.IsMember(r.Context(), guildID, userID)
	if err != nil {
		respondError(w, err)
		return
	}
	if isMember {
		respondError(w, apperrors.Conflict("already a member of this guild"))
		return
	}

	if err := h.guildRepo.AddMember(r.Context(), guildID, userID, ""); err != nil {
		respondError(w, err)
		return
	}

	// Increment use count.
	_, _ = h.pool.Exec(r.Context(), `
		UPDATE invites SET use_count = use_count + 1 WHERE code = $1`, req.InviteCode)

	respondJSON(w, http.StatusOK, map[string]string{"message": "joined guild"})
}

// KickMember removes a member from the guild.
func (h *GuildHandler) KickMember(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	guildID, err := parseUUID(chi.URLParam(r, "guildID"))
	if err != nil {
		respondError(w, err)
		return
	}

	targetID, err := parseUUID(chi.URLParam(r, "userID"))
	if err != nil {
		respondError(w, err)
		return
	}

	guild, err := h.guildRepo.GetByID(r.Context(), guildID)
	if err != nil {
		respondError(w, err)
		return
	}

	if guild.OwnerID != userID {
		respondError(w, apperrors.Forbidden("only the guild owner can kick members"))
		return
	}

	if targetID == userID {
		respondError(w, apperrors.Validation("cannot kick yourself", nil))
		return
	}

	if err := h.guildRepo.RemoveMember(r.Context(), guildID, targetID); err != nil {
		respondError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// CreateInvite creates an invite for the guild.
func (h *GuildHandler) CreateInvite(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	guildID, err := parseUUID(chi.URLParam(r, "guildID"))
	if err != nil {
		respondError(w, err)
		return
	}

	// Verify the user is a member.
	isMember, err := h.guildRepo.IsMember(r.Context(), guildID, userID)
	if err != nil {
		respondError(w, err)
		return
	}
	if !isMember {
		respondError(w, apperrors.Forbidden("must be a guild member to create invites"))
		return
	}

	var req createInviteRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, err)
		return
	}

	code := generateInviteCode()
	now := time.Now()
	inviteID := uuid.New()

	var expiresAt *time.Time
	if req.MaxAgeSecs != nil && *req.MaxAgeSecs > 0 {
		t := now.Add(time.Duration(*req.MaxAgeSecs) * time.Second)
		expiresAt = &t
	}

	_, err = h.pool.Exec(r.Context(), `
		INSERT INTO invites (id, code, type, guild_id, inviter_id, max_uses, use_count, max_age_secs, is_temporary, created_at, expires_at)
		VALUES ($1, $2, 'guild', $3, $4, $5, 0, $6, false, $7, $8)`,
		inviteID, code, guildID, userID, req.MaxUses, req.MaxAgeSecs, now, expiresAt,
	)
	if err != nil {
		respondError(w, apperrors.Internal("failed to create invite"))
		return
	}

	respondJSON(w, http.StatusCreated, inviteResponse{
		Code:      code,
		GuildID:   guildID,
		InviterID: userID,
		MaxUses:   req.MaxUses,
		UseCount:  0,
		CreatedAt: now,
		ExpiresAt: expiresAt,
	})
}

// ListInvites returns the invites for a guild.
func (h *GuildHandler) ListInvites(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	guildID, err := parseUUID(chi.URLParam(r, "guildID"))
	if err != nil {
		respondError(w, err)
		return
	}

	// Verify the user is a member.
	isMember, err := h.guildRepo.IsMember(r.Context(), guildID, userID)
	if err != nil {
		respondError(w, err)
		return
	}
	if !isMember {
		respondError(w, apperrors.Forbidden("must be a guild member to view invites"))
		return
	}

	rows, err := h.pool.Query(r.Context(), `
		SELECT code, guild_id, inviter_id, max_uses, use_count, created_at, expires_at
		FROM invites
		WHERE guild_id = $1 AND type = 'guild'
		ORDER BY created_at DESC`, guildID,
	)
	if err != nil {
		respondError(w, apperrors.Internal("failed to list invites"))
		return
	}
	defer rows.Close()

	var invites []inviteResponse
	for rows.Next() {
		var inv inviteResponse
		if err := rows.Scan(&inv.Code, &inv.GuildID, &inv.InviterID, &inv.MaxUses, &inv.UseCount, &inv.CreatedAt, &inv.ExpiresAt); err != nil {
			respondError(w, apperrors.Internal("failed to scan invite"))
			return
		}
		invites = append(invites, inv)
	}
	if err := rows.Err(); err != nil {
		respondError(w, apperrors.Internal("failed to iterate invites"))
		return
	}

	respondJSON(w, http.StatusOK, invites)
}

// generateInviteCode generates a short random invite code.
func generateInviteCode() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
