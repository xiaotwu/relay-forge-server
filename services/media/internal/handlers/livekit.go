package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/relay-forge/relay-forge/services/media/internal/config"
)

type LiveKitHandler struct {
	cfg *config.Config
}

func NewLiveKitHandler(cfg *config.Config) *LiveKitHandler {
	return &LiveKitHandler{cfg: cfg}
}

type tokenRequest struct {
	RoomName     string `json:"room_name"`
	Identity     string `json:"identity"`
	DisplayName  string `json:"display_name"`
	CanPublish   bool   `json:"can_publish"`
	CanSubscribe bool   `json:"can_subscribe"`
}

type tokenResponse struct {
	Token     string `json:"token"`
	URL       string `json:"url"`
	ExpiresAt int64  `json:"expires_at"`
}

func (h *LiveKitHandler) GenerateToken(w http.ResponseWriter, r *http.Request) {
	var req tokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.RoomName == "" || req.Identity == "" {
		http.Error(w, `{"error":"room_name and identity are required"}`, http.StatusBadRequest)
		return
	}

	expiresAt := time.Now().Add(24 * time.Hour)
	token, err := buildLiveKitToken(h.cfg.LiveKit.APIKey, h.cfg.LiveKit.APISecret, req.RoomName, req.Identity, req.CanPublish, req.CanSubscribe, expiresAt)
	if err != nil {
		http.Error(w, `{"error":"failed to generate token"}`, http.StatusInternalServerError)
		return
	}

	if err := writeJSON(w, http.StatusOK, tokenResponse{
		Token:     token,
		URL:       h.cfg.LiveKit.URL,
		ExpiresAt: expiresAt.Unix(),
	}); err != nil {
		http.Error(w, `{"error":"failed to write response"}`, http.StatusInternalServerError)
	}
}

type createRoomRequest struct {
	Name            string `json:"name"`
	MaxParticipants uint32 `json:"max_participants"`
}

func (h *LiveKitHandler) CreateRoom(w http.ResponseWriter, r *http.Request) {
	var req createRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if err := writeJSON(w, http.StatusOK, map[string]any{
		"name":             req.Name,
		"max_participants": req.MaxParticipants,
		"status":           "created",
	}); err != nil {
		http.Error(w, `{"error":"failed to write response"}`, http.StatusInternalServerError)
	}
}

func (h *LiveKitHandler) DeleteRoom(w http.ResponseWriter, r *http.Request) {
	roomName := chi.URLParam(r, "roomName")
	if err := writeJSON(w, http.StatusOK, map[string]string{
		"room":   roomName,
		"status": "deleted",
	}); err != nil {
		http.Error(w, `{"error":"failed to write response"}`, http.StatusInternalServerError)
	}
}

func (h *LiveKitHandler) ListRooms(w http.ResponseWriter, r *http.Request) {
	if err := writeJSON(w, http.StatusOK, map[string]any{
		"rooms": []any{},
	}); err != nil {
		http.Error(w, `{"error":"failed to write response"}`, http.StatusInternalServerError)
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(payload)
}

// buildLiveKitToken generates a LiveKit-compatible JWT access token.
// LiveKit tokens are standard JWTs with specific "video" grant claims.
// For production, consider using github.com/livekit/server-sdk-go/v2 AccessToken.
func buildLiveKitToken(apiKey, apiSecret, room, identity string, canPublish, canSubscribe bool, expiresAt time.Time) (string, error) {
	header := map[string]string{"alg": "HS256", "typ": "JWT"}
	headerJSON, _ := json.Marshal(header)
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)

	claims := map[string]any{
		"iss":  apiKey,
		"sub":  identity,
		"iat":  time.Now().Unix(),
		"exp":  expiresAt.Unix(),
		"nbf":  time.Now().Unix(),
		"jti":  fmt.Sprintf("%s-%s-%d", room, identity, time.Now().UnixNano()),
		"name": identity,
		"video": map[string]any{
			"room":         room,
			"roomJoin":     true,
			"canPublish":   canPublish,
			"canSubscribe": canSubscribe,
		},
	}
	claimsJSON, _ := json.Marshal(claims)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	signingInput := headerB64 + "." + claimsB64
	mac := hmac.New(sha256.New, []byte(apiSecret))
	mac.Write([]byte(signingInput))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return signingInput + "." + signature, nil
}
