package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/relay-forge/relay-forge/services/api/internal/auth"
	"github.com/relay-forge/relay-forge/services/api/internal/config"
	apperrors "github.com/relay-forge/relay-forge/services/api/internal/errors"
	"github.com/relay-forge/relay-forge/services/api/internal/middleware"
	"github.com/relay-forge/relay-forge/services/api/internal/repository"

	"github.com/jackc/pgx/v5/pgxpool"
)

// UserHandler handles user profile and account endpoints.
type UserHandler struct {
	userRepo    *repository.UserRepository
	sessionRepo *repository.SessionRepository
	jwtSvc      *auth.JWTService
	cfg         *config.Config
	pool        *pgxpool.Pool
}

// NewUserHandler creates a new UserHandler.
func NewUserHandler(
	userRepo *repository.UserRepository,
	sessionRepo *repository.SessionRepository,
	jwtSvc *auth.JWTService,
	cfg *config.Config,
	pool *pgxpool.Pool,
) *UserHandler {
	return &UserHandler{
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
		jwtSvc:      jwtSvc,
		cfg:         cfg,
		pool:        pool,
	}
}

// -- Request / Response types ------------------------------------------------

type updateProfileRequest struct {
	DisplayName  *string `json:"display_name,omitempty"`
	AvatarURL    *string `json:"avatar_url,omitempty"`
	BannerURL    *string `json:"banner_url,omitempty"`
	Bio          *string `json:"bio,omitempty"`
	Status       *string `json:"status,omitempty"`
	CustomStatus *string `json:"custom_status,omitempty"`
	Locale       *string `json:"locale,omitempty"`
}

type enable2FAResponse struct {
	Secret string `json:"secret"`
	URL    string `json:"url"`
}

type verify2FARequest struct {
	Code string `json:"code"`
}

type disable2FARequest struct {
	Code string `json:"code"`
}

// -- Handlers ----------------------------------------------------------------

// GetMe returns the current user's profile.
func (h *UserHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == uuid.Nil {
		respondError(w, apperrors.Unauthorized("not authenticated"))
		return
	}

	user, err := h.userRepo.GetByID(r.Context(), userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, user)
}

// UpdateMe updates the current user's profile.
func (h *UserHandler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == uuid.Nil {
		respondError(w, apperrors.Unauthorized("not authenticated"))
		return
	}

	var req updateProfileRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, err)
		return
	}

	user, err := h.userRepo.GetByID(r.Context(), userID)
	if err != nil {
		respondError(w, err)
		return
	}

	if req.DisplayName != nil {
		user.DisplayName = req.DisplayName
	}
	if req.AvatarURL != nil {
		user.AvatarURL = req.AvatarURL
	}
	if req.BannerURL != nil {
		user.BannerURL = req.BannerURL
	}
	if req.Bio != nil {
		user.Bio = req.Bio
	}
	if req.Status != nil {
		user.Status = *req.Status
	}
	if req.CustomStatus != nil {
		user.CustomStatus = req.CustomStatus
	}
	if req.Locale != nil {
		user.Locale = *req.Locale
	}

	if err := h.userRepo.Update(r.Context(), user); err != nil {
		respondError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, user)
}

// ListSessions returns the current user's active sessions.
func (h *UserHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == uuid.Nil {
		respondError(w, apperrors.Unauthorized("not authenticated"))
		return
	}

	sessions, err := h.sessionRepo.ListByUser(r.Context(), userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, sessions)
}

// RevokeSession revokes a specific session.
func (h *UserHandler) RevokeSession(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == uuid.Nil {
		respondError(w, apperrors.Unauthorized("not authenticated"))
		return
	}

	sessionID, err := parseUUID(chi.URLParam(r, "sessionID"))
	if err != nil {
		respondError(w, err)
		return
	}

	// Verify the session belongs to the current user.
	session, err := h.sessionRepo.GetByID(r.Context(), sessionID)
	if err != nil {
		respondError(w, err)
		return
	}
	if session.UserID != userID {
		respondError(w, apperrors.Forbidden("cannot revoke another user's session"))
		return
	}

	if err := h.sessionRepo.Delete(r.Context(), sessionID); err != nil {
		respondError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Enable2FA generates a TOTP secret for the current user.
func (h *UserHandler) Enable2FA(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == uuid.Nil {
		respondError(w, apperrors.Unauthorized("not authenticated"))
		return
	}

	user, err := h.userRepo.GetByID(r.Context(), userID)
	if err != nil {
		respondError(w, err)
		return
	}

	secret, url, err := auth.GenerateTOTPSecret(h.cfg.Auth.TOTPIssuer, user.Email)
	if err != nil {
		respondError(w, apperrors.Internal("failed to generate TOTP secret"))
		return
	}

	// Store the unverified secret.
	_, err = h.pool.Exec(r.Context(), `
		INSERT INTO totp_secrets (user_id, secret, verified, created_at)
		VALUES ($1, $2, false, NOW())
		ON CONFLICT (user_id) DO UPDATE SET secret = $2, verified = false`,
		userID, secret,
	)
	if err != nil {
		respondError(w, apperrors.Internal("failed to store TOTP secret"))
		return
	}

	respondJSON(w, http.StatusOK, enable2FAResponse{
		Secret: secret,
		URL:    url,
	})
}

// Verify2FA verifies the TOTP code and enables 2FA.
func (h *UserHandler) Verify2FA(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == uuid.Nil {
		respondError(w, apperrors.Unauthorized("not authenticated"))
		return
	}

	var req verify2FARequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, err)
		return
	}

	if req.Code == "" {
		respondError(w, apperrors.Validation("code is required", nil))
		return
	}

	// Retrieve the stored secret.
	var secret string
	err := h.pool.QueryRow(r.Context(), `
		SELECT secret FROM totp_secrets WHERE user_id = $1`, userID,
	).Scan(&secret)
	if err != nil {
		respondError(w, apperrors.NotFound("no 2FA setup in progress"))
		return
	}

	if !auth.ValidateTOTP(secret, req.Code) {
		respondError(w, apperrors.Validation("invalid TOTP code", nil))
		return
	}

	// Mark as verified.
	_, err = h.pool.Exec(r.Context(), `
		UPDATE totp_secrets SET verified = true, enabled_at = NOW() WHERE user_id = $1`, userID,
	)
	if err != nil {
		respondError(w, apperrors.Internal("failed to enable 2FA"))
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "2FA enabled successfully"})
}

// Disable2FA disables 2FA for the current user.
func (h *UserHandler) Disable2FA(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == uuid.Nil {
		respondError(w, apperrors.Unauthorized("not authenticated"))
		return
	}

	var req disable2FARequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, err)
		return
	}

	if req.Code == "" {
		respondError(w, apperrors.Validation("code is required", nil))
		return
	}

	// Verify the code before disabling.
	var secret string
	var verified bool
	err := h.pool.QueryRow(r.Context(), `
		SELECT secret, verified FROM totp_secrets WHERE user_id = $1`, userID,
	).Scan(&secret, &verified)
	if err != nil {
		respondError(w, apperrors.NotFound("2FA is not enabled"))
		return
	}

	if !verified {
		respondError(w, apperrors.Validation("2FA is not verified", nil))
		return
	}

	if !auth.ValidateTOTP(secret, req.Code) {
		respondError(w, apperrors.Validation("invalid TOTP code", nil))
		return
	}

	_, err = h.pool.Exec(r.Context(), `DELETE FROM totp_secrets WHERE user_id = $1`, userID)
	if err != nil {
		respondError(w, apperrors.Internal("failed to disable 2FA"))
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": "2FA disabled successfully"})
}
