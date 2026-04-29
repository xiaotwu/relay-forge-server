package handlers

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apperrors "github.com/relay-forge/relay-forge/services/api/internal/errors"
	"github.com/relay-forge/relay-forge/services/api/internal/middleware"
	apirealtime "github.com/relay-forge/relay-forge/services/api/internal/realtime"
	"github.com/relay-forge/relay-forge/services/api/internal/repository"
)

type DMHandler struct {
	dmRepo    *repository.DMRepository
	userRepo  *repository.UserRepository
	publisher *apirealtime.Publisher
}

func NewDMHandler(
	dmRepo *repository.DMRepository,
	userRepo *repository.UserRepository,
	publishers ...*apirealtime.Publisher,
) *DMHandler {
	var publisher *apirealtime.Publisher
	if len(publishers) > 0 {
		publisher = publishers[0]
	}
	return &DMHandler{
		dmRepo:    dmRepo,
		userRepo:  userRepo,
		publisher: publisher,
	}
}

type createDMChannelRequest struct {
	ParticipantIDs []uuid.UUID `json:"participant_ids"`
	Name           *string     `json:"name,omitempty"`
}

type updateDMChannelRequest struct {
	Name *string `json:"name,omitempty"`
}

type sendDMMessageRequest struct {
	Content   string     `json:"content"`
	ReplyToID *uuid.UUID `json:"reply_to_id,omitempty"`
}

func (h *DMHandler) ListChannels(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == uuid.Nil {
		respondError(w, apperrors.Unauthorized("not authenticated"))
		return
	}

	channels, err := h.dmRepo.ListChannelsForUser(r.Context(), userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, channels)
}

func (h *DMHandler) UpdateChannel(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == uuid.Nil {
		respondError(w, apperrors.Unauthorized("not authenticated"))
		return
	}

	channelID, err := parseUUID(chi.URLParam(r, "dmChannelID"))
	if err != nil {
		respondError(w, err)
		return
	}

	var req updateDMChannelRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, err)
		return
	}

	channel, err := h.dmRepo.UpdateChannel(r.Context(), channelID, userID, req.Name)
	if err != nil {
		respondError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, channel)
}

func (h *DMHandler) CreateChannel(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == uuid.Nil {
		respondError(w, apperrors.Unauthorized("not authenticated"))
		return
	}

	var req createDMChannelRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, err)
		return
	}

	if len(req.ParticipantIDs) == 0 {
		respondError(w, apperrors.Validation("participant_ids is required", nil))
		return
	}

	for _, participantID := range req.ParticipantIDs {
		if participantID == userID {
			continue
		}
		if _, err := h.userRepo.GetByID(r.Context(), participantID); err != nil {
			respondError(w, err)
			return
		}
	}

	channel, err := h.dmRepo.CreateOrGetDirectChannel(r.Context(), userID, req.ParticipantIDs, req.Name)
	if err != nil {
		respondError(w, err)
		return
	}

	channel, err = h.dmRepo.GetChannelForUser(r.Context(), channel.ID, userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, channel)
}

func (h *DMHandler) ListMessages(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == uuid.Nil {
		respondError(w, apperrors.Unauthorized("not authenticated"))
		return
	}

	channelID, err := parseUUID(chi.URLParam(r, "dmChannelID"))
	if err != nil {
		respondError(w, err)
		return
	}

	messages, err := h.dmRepo.ListMessages(r.Context(), channelID, userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, messages)
}

func (h *DMHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == uuid.Nil {
		respondError(w, apperrors.Unauthorized("not authenticated"))
		return
	}

	channelID, err := parseUUID(chi.URLParam(r, "dmChannelID"))
	if err != nil {
		respondError(w, err)
		return
	}

	var req sendDMMessageRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, err)
		return
	}

	content := strings.TrimSpace(req.Content)
	if content == "" {
		respondError(w, apperrors.Validation("content is required", nil))
		return
	}

	message, err := h.dmRepo.CreateMessage(r.Context(), channelID, userID, content, req.ReplyToID)
	if err != nil {
		respondError(w, err)
		return
	}

	h.publishDMEvent(r, "DM_MESSAGE_CREATE", channelID, map[string]any{
		"message":    message,
		"channel_id": channelID,
	})

	respondJSON(w, http.StatusCreated, message)
}

func (h *DMHandler) DeleteMessage(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == uuid.Nil {
		respondError(w, apperrors.Unauthorized("not authenticated"))
		return
	}

	channelID, err := parseUUID(chi.URLParam(r, "dmChannelID"))
	if err != nil {
		respondError(w, err)
		return
	}

	messageID, err := parseUUID(chi.URLParam(r, "messageID"))
	if err != nil {
		respondError(w, err)
		return
	}

	if err := h.dmRepo.DeleteMessage(r.Context(), channelID, messageID, userID); err != nil {
		respondError(w, err)
		return
	}

	h.publishDMEvent(r, "DM_MESSAGE_DELETE", channelID, map[string]any{
		"message_id": messageID,
		"channel_id": channelID,
	})

	w.WriteHeader(http.StatusNoContent)
}

func (h *DMHandler) publishDMEvent(r *http.Request, eventType string, channelID uuid.UUID, payload map[string]any) {
	recipients, err := h.dmRepo.ListMemberIDs(r.Context(), channelID)
	if err != nil {
		return
	}
	h.publisher.Publish(r.Context(), apirealtime.Event{
		Type:    eventType,
		UserIDs: recipients,
		Data:    apirealtime.MustRaw(payload),
	})
}
