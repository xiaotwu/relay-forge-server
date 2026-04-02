package handlers

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apperrors "github.com/relay-forge/relay-forge/services/api/internal/errors"
	"github.com/relay-forge/relay-forge/services/api/internal/middleware"
	"github.com/relay-forge/relay-forge/services/api/internal/models"
	"github.com/relay-forge/relay-forge/services/api/internal/repository"
)

// ChannelHandler handles channel-related endpoints.
type ChannelHandler struct {
	channelRepo *repository.ChannelRepository
	guildRepo   *repository.GuildRepository
}

// NewChannelHandler creates a new ChannelHandler.
func NewChannelHandler(channelRepo *repository.ChannelRepository, guildRepo *repository.GuildRepository) *ChannelHandler {
	return &ChannelHandler{
		channelRepo: channelRepo,
		guildRepo:   guildRepo,
	}
}

// -- Request / Response types ------------------------------------------------

type createChannelRequest struct {
	Name         string     `json:"name"`
	Type         string     `json:"type"`
	Topic        *string    `json:"topic,omitempty"`
	CategoryID   *uuid.UUID `json:"category_id,omitempty"`
	Position     int        `json:"position"`
	IsNSFW       bool       `json:"is_nsfw"`
	SlowmodeSecs int        `json:"slowmode_secs"`
}

type updateChannelRequest struct {
	Name         *string    `json:"name,omitempty"`
	Topic        *string    `json:"topic,omitempty"`
	CategoryID   *uuid.UUID `json:"category_id,omitempty"`
	Position     *int       `json:"position,omitempty"`
	IsNSFW       *bool      `json:"is_nsfw,omitempty"`
	SlowmodeSecs *int       `json:"slowmode_secs,omitempty"`
}

// -- Handlers ----------------------------------------------------------------

// CreateChannel creates a new channel in a guild.
func (h *ChannelHandler) CreateChannel(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	guildID, err := parseUUID(chi.URLParam(r, "guildID"))
	if err != nil {
		respondError(w, err)
		return
	}

	// Verify the user is the guild owner.
	guild, err := h.guildRepo.GetByID(r.Context(), guildID)
	if err != nil {
		respondError(w, err)
		return
	}
	if guild.OwnerID != userID {
		respondError(w, apperrors.Forbidden("only the guild owner can create channels"))
		return
	}

	var req createChannelRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, err)
		return
	}

	if req.Name == "" {
		respondError(w, apperrors.Validation("name is required", nil))
		return
	}
	if req.Type == "" {
		req.Type = "text"
	}

	now := time.Now()
	channel := &models.Channel{
		ID:           uuid.New(),
		GuildID:      guildID,
		CategoryID:   req.CategoryID,
		Name:         req.Name,
		Type:         req.Type,
		Topic:        req.Topic,
		Position:     req.Position,
		IsNSFW:       req.IsNSFW,
		SlowmodeSecs: req.SlowmodeSecs,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := h.channelRepo.Create(r.Context(), channel); err != nil {
		respondError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, channel)
}

// ListChannels returns the channels in a guild.
func (h *ChannelHandler) ListChannels(w http.ResponseWriter, r *http.Request) {
	guildID, err := parseUUID(chi.URLParam(r, "guildID"))
	if err != nil {
		respondError(w, err)
		return
	}

	channels, err := h.channelRepo.ListByGuild(r.Context(), guildID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, channels)
}

// GetChannel returns a specific channel.
func (h *ChannelHandler) GetChannel(w http.ResponseWriter, r *http.Request) {
	channelID, err := parseUUID(chi.URLParam(r, "channelID"))
	if err != nil {
		respondError(w, err)
		return
	}

	channel, err := h.channelRepo.GetByID(r.Context(), channelID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, channel)
}

// UpdateChannel updates a channel.
func (h *ChannelHandler) UpdateChannel(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	guildID, err := parseUUID(chi.URLParam(r, "guildID"))
	if err != nil {
		respondError(w, err)
		return
	}

	channelID, err := parseUUID(chi.URLParam(r, "channelID"))
	if err != nil {
		respondError(w, err)
		return
	}

	// Verify the user is the guild owner.
	guild, err := h.guildRepo.GetByID(r.Context(), guildID)
	if err != nil {
		respondError(w, err)
		return
	}
	if guild.OwnerID != userID {
		respondError(w, apperrors.Forbidden("only the guild owner can update channels"))
		return
	}

	channel, err := h.channelRepo.GetByID(r.Context(), channelID)
	if err != nil {
		respondError(w, err)
		return
	}

	if channel.GuildID != guildID {
		respondError(w, apperrors.NotFound("channel not found in this guild"))
		return
	}

	var req updateChannelRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, err)
		return
	}

	if req.Name != nil {
		channel.Name = *req.Name
	}
	if req.Topic != nil {
		channel.Topic = req.Topic
	}
	if req.CategoryID != nil {
		channel.CategoryID = req.CategoryID
	}
	if req.Position != nil {
		channel.Position = *req.Position
	}
	if req.IsNSFW != nil {
		channel.IsNSFW = *req.IsNSFW
	}
	if req.SlowmodeSecs != nil {
		channel.SlowmodeSecs = *req.SlowmodeSecs
	}

	if err := h.channelRepo.Update(r.Context(), channel); err != nil {
		respondError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, channel)
}

// DeleteChannel deletes a channel.
func (h *ChannelHandler) DeleteChannel(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	guildID, err := parseUUID(chi.URLParam(r, "guildID"))
	if err != nil {
		respondError(w, err)
		return
	}

	channelID, err := parseUUID(chi.URLParam(r, "channelID"))
	if err != nil {
		respondError(w, err)
		return
	}

	// Verify the user is the guild owner.
	guild, err := h.guildRepo.GetByID(r.Context(), guildID)
	if err != nil {
		respondError(w, err)
		return
	}
	if guild.OwnerID != userID {
		respondError(w, apperrors.Forbidden("only the guild owner can delete channels"))
		return
	}

	channel, err := h.channelRepo.GetByID(r.Context(), channelID)
	if err != nil {
		respondError(w, err)
		return
	}

	if channel.GuildID != guildID {
		respondError(w, apperrors.NotFound("channel not found in this guild"))
		return
	}

	if err := h.channelRepo.SoftDelete(r.Context(), channelID); err != nil {
		respondError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
