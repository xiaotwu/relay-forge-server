package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apperrors "github.com/relay-forge/relay-forge/services/api/internal/errors"
	"github.com/relay-forge/relay-forge/services/api/internal/middleware"
	"github.com/relay-forge/relay-forge/services/api/internal/models"
	apirealtime "github.com/relay-forge/relay-forge/services/api/internal/realtime"
	"github.com/relay-forge/relay-forge/services/api/internal/repository"
)

// MessageHandler handles message-related endpoints.
type MessageHandler struct {
	messageRepo *repository.MessageRepository
	channelRepo *repository.ChannelRepository
	guildRepo   *repository.GuildRepository
	publisher   *apirealtime.Publisher
}

// NewMessageHandler creates a new MessageHandler.
func NewMessageHandler(
	messageRepo *repository.MessageRepository,
	channelRepo *repository.ChannelRepository,
	guildRepo *repository.GuildRepository,
	publishers ...*apirealtime.Publisher,
) *MessageHandler {
	var publisher *apirealtime.Publisher
	if len(publishers) > 0 {
		publisher = publishers[0]
	}
	return &MessageHandler{
		messageRepo: messageRepo,
		channelRepo: channelRepo,
		guildRepo:   guildRepo,
		publisher:   publisher,
	}
}

// -- Request / Response types ------------------------------------------------

type sendMessageRequest struct {
	Content   string     `json:"content"`
	ReplyToID *uuid.UUID `json:"reply_to_id,omitempty"`
}

type editMessageRequest struct {
	Content string `json:"content"`
}

type addReactionRequest struct {
	Emoji string `json:"emoji"`
}

func (h *MessageHandler) requireChannelMember(r *http.Request, channelID, userID uuid.UUID) (*models.Channel, error) {
	channel, err := h.channelRepo.GetByID(r.Context(), channelID)
	if err != nil {
		return nil, err
	}

	isMember, err := h.guildRepo.IsMember(r.Context(), channel.GuildID, userID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, apperrors.Forbidden("must be a guild member to access this channel")
	}

	return channel, nil
}

// -- Handlers ----------------------------------------------------------------

// ListMessages returns messages in a channel with cursor-based pagination.
func (h *MessageHandler) ListMessages(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	channelID, err := parseUUID(chi.URLParam(r, "channelID"))
	if err != nil {
		respondError(w, err)
		return
	}
	if _, err := h.requireChannelMember(r, channelID, userID); err != nil {
		respondError(w, err)
		return
	}

	limit, _ := parsePagination(r)
	cursor := parseCursor(r)

	messages, err := h.messageRepo.ListByChannel(r.Context(), channelID, cursor, limit)
	if err != nil {
		respondError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, messages)
}

// SendMessage sends a new message to a channel.
func (h *MessageHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	channelID, err := parseUUID(chi.URLParam(r, "channelID"))
	if err != nil {
		respondError(w, err)
		return
	}

	var req sendMessageRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, err)
		return
	}

	content := strings.TrimSpace(req.Content)
	if content == "" {
		respondError(w, apperrors.Validation("content is required", nil))
		return
	}

	channel, err := h.requireChannelMember(r, channelID, userID)
	if err != nil {
		respondError(w, err)
		return
	}

	msg := &models.Message{
		ID:        uuid.New(),
		ChannelID: channelID,
		AuthorID:  userID,
		Content:   &content,
		Type:      "default",
		ReplyToID: req.ReplyToID,
		CreatedAt: time.Now(),
	}

	if err := h.messageRepo.Create(r.Context(), msg); err != nil {
		respondError(w, err)
		return
	}

	h.publisher.Publish(r.Context(), apirealtime.Event{
		Type:    "MESSAGE_CREATE",
		GuildID: &channel.GuildID,
		Data: apirealtime.MustRaw(map[string]any{
			"message":  msg,
			"guild_id": channel.GuildID,
		}),
	})

	respondJSON(w, http.StatusCreated, msg)
}

// EditMessage edits an existing message. Only the author can edit.
func (h *MessageHandler) EditMessage(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	channelID, err := parseUUID(chi.URLParam(r, "channelID"))
	if err != nil {
		respondError(w, err)
		return
	}
	messageID, err := parseUUID(chi.URLParam(r, "messageID"))
	if err != nil {
		respondError(w, err)
		return
	}

	msg, err := h.messageRepo.GetByID(r.Context(), messageID)
	if err != nil {
		respondError(w, err)
		return
	}

	if msg.AuthorID != userID {
		respondError(w, apperrors.Forbidden("can only edit your own messages"))
		return
	}
	if msg.ChannelID != channelID {
		respondError(w, apperrors.NotFound("message not found in this channel"))
		return
	}
	channel, err := h.requireChannelMember(r, channelID, userID)
	if err != nil {
		respondError(w, err)
		return
	}

	var req editMessageRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, err)
		return
	}

	content := strings.TrimSpace(req.Content)
	if content == "" {
		respondError(w, apperrors.Validation("content is required", nil))
		return
	}

	msg.Content = &content
	if err := h.messageRepo.Update(r.Context(), msg); err != nil {
		respondError(w, err)
		return
	}

	h.publisher.Publish(r.Context(), apirealtime.Event{
		Type:    "MESSAGE_UPDATE",
		GuildID: &channel.GuildID,
		Data: apirealtime.MustRaw(map[string]any{
			"message":  msg,
			"guild_id": channel.GuildID,
		}),
	})

	respondJSON(w, http.StatusOK, msg)
}

// DeleteMessage deletes a message. Only the author can delete.
func (h *MessageHandler) DeleteMessage(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	channelID, err := parseUUID(chi.URLParam(r, "channelID"))
	if err != nil {
		respondError(w, err)
		return
	}
	messageID, err := parseUUID(chi.URLParam(r, "messageID"))
	if err != nil {
		respondError(w, err)
		return
	}

	msg, err := h.messageRepo.GetByID(r.Context(), messageID)
	if err != nil {
		respondError(w, err)
		return
	}

	if msg.AuthorID != userID {
		respondError(w, apperrors.Forbidden("can only delete your own messages"))
		return
	}
	if msg.ChannelID != channelID {
		respondError(w, apperrors.NotFound("message not found in this channel"))
		return
	}
	channel, err := h.requireChannelMember(r, channelID, userID)
	if err != nil {
		respondError(w, err)
		return
	}

	if err := h.messageRepo.SoftDelete(r.Context(), messageID); err != nil {
		respondError(w, err)
		return
	}

	h.publisher.Publish(r.Context(), apirealtime.Event{
		Type:    "MESSAGE_DELETE",
		GuildID: &channel.GuildID,
		Data: apirealtime.MustRaw(map[string]any{
			"message_id": messageID,
			"channel_id": channelID,
			"guild_id":   channel.GuildID,
		}),
	})

	w.WriteHeader(http.StatusNoContent)
}

// SearchMessages searches messages in a channel.
func (h *MessageHandler) SearchMessages(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	channelID, err := parseUUID(chi.URLParam(r, "channelID"))
	if err != nil {
		respondError(w, err)
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		respondError(w, apperrors.Validation("q query parameter is required", nil))
		return
	}
	if _, err := h.requireChannelMember(r, channelID, userID); err != nil {
		respondError(w, err)
		return
	}

	limit, offset := parsePagination(r)

	messages, total, err := h.messageRepo.Search(r.Context(), channelID, query, limit, offset)
	if err != nil {
		respondError(w, err)
		return
	}

	respondJSONWithMeta(w, http.StatusOK, messages, map[string]int{"total": total})
}

// ListPins returns pinned messages in a channel.
func (h *MessageHandler) ListPins(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	channelID, err := parseUUID(chi.URLParam(r, "channelID"))
	if err != nil {
		respondError(w, err)
		return
	}
	if _, err := h.requireChannelMember(r, channelID, userID); err != nil {
		respondError(w, err)
		return
	}

	messages, err := h.messageRepo.ListPins(r.Context(), channelID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, messages)
}

// PinMessage pins a message in a channel.
func (h *MessageHandler) PinMessage(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	channelID, err := parseUUID(chi.URLParam(r, "channelID"))
	if err != nil {
		respondError(w, err)
		return
	}

	messageID, err := parseUUID(chi.URLParam(r, "messageID"))
	if err != nil {
		respondError(w, err)
		return
	}
	if _, err := h.requireChannelMember(r, channelID, userID); err != nil {
		respondError(w, err)
		return
	}

	// Verify message belongs to this channel.
	msg, err := h.messageRepo.GetByID(r.Context(), messageID)
	if err != nil {
		respondError(w, err)
		return
	}
	if msg.ChannelID != channelID {
		respondError(w, apperrors.NotFound("message not found in this channel"))
		return
	}

	if err := h.messageRepo.Pin(r.Context(), channelID, messageID, userID); err != nil {
		respondError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// UnpinMessage unpins a message in a channel.
func (h *MessageHandler) UnpinMessage(w http.ResponseWriter, r *http.Request) {
	channelID, err := parseUUID(chi.URLParam(r, "channelID"))
	if err != nil {
		respondError(w, err)
		return
	}

	messageID, err := parseUUID(chi.URLParam(r, "messageID"))
	if err != nil {
		respondError(w, err)
		return
	}
	if _, err := h.requireChannelMember(r, channelID, middleware.GetUserID(r.Context())); err != nil {
		respondError(w, err)
		return
	}

	if err := h.messageRepo.Unpin(r.Context(), channelID, messageID); err != nil {
		respondError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// AddReaction adds a reaction to a message.
func (h *MessageHandler) AddReaction(w http.ResponseWriter, r *http.Request) {
	var req addReactionRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, err)
		return
	}
	h.addReactionValue(w, r, req.Emoji)
}

// AddReactionByEmoji adds a reaction encoded in the request path.
func (h *MessageHandler) AddReactionByEmoji(w http.ResponseWriter, r *http.Request) {
	h.addReactionValue(w, r, chi.URLParam(r, "emoji"))
}

func (h *MessageHandler) addReactionValue(w http.ResponseWriter, r *http.Request, emoji string) {
	userID := middleware.GetUserID(r.Context())
	channelID, err := parseUUID(chi.URLParam(r, "channelID"))
	if err != nil {
		respondError(w, err)
		return
	}
	messageID, err := parseUUID(chi.URLParam(r, "messageID"))
	if err != nil {
		respondError(w, err)
		return
	}

	if emoji == "" {
		respondError(w, apperrors.Validation("emoji is required", nil))
		return
	}
	if _, err := h.requireChannelMember(r, channelID, userID); err != nil {
		respondError(w, err)
		return
	}

	msg, err := h.messageRepo.GetByID(r.Context(), messageID)
	if err != nil {
		respondError(w, err)
		return
	}
	if msg.ChannelID != channelID {
		respondError(w, apperrors.NotFound("message not found in this channel"))
		return
	}

	if err := h.messageRepo.AddReaction(r.Context(), messageID, userID, nil, emoji); err != nil {
		respondError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// RemoveReaction removes a reaction from a message.
func (h *MessageHandler) RemoveReaction(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	channelID, err := parseUUID(chi.URLParam(r, "channelID"))
	if err != nil {
		respondError(w, err)
		return
	}
	messageID, err := parseUUID(chi.URLParam(r, "messageID"))
	if err != nil {
		respondError(w, err)
		return
	}

	emoji := chi.URLParam(r, "emoji")
	if emoji == "" {
		respondError(w, apperrors.Validation("emoji is required", nil))
		return
	}
	if _, err := h.requireChannelMember(r, channelID, userID); err != nil {
		respondError(w, err)
		return
	}

	msg, err := h.messageRepo.GetByID(r.Context(), messageID)
	if err != nil {
		respondError(w, err)
		return
	}
	if msg.ChannelID != channelID {
		respondError(w, apperrors.NotFound("message not found in this channel"))
		return
	}

	if err := h.messageRepo.RemoveReaction(r.Context(), messageID, userID, nil, emoji); err != nil {
		respondError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
