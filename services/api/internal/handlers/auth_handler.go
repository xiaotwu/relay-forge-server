package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/relay-forge/relay-forge/services/api/internal/auth"
	"github.com/relay-forge/relay-forge/services/api/internal/config"
	apperrors "github.com/relay-forge/relay-forge/services/api/internal/errors"
	"github.com/relay-forge/relay-forge/services/api/internal/models"
	"github.com/relay-forge/relay-forge/services/api/internal/repository"
)

// AuthHandler handles authentication-related endpoints.
type AuthHandler struct {
	userRepo    *repository.UserRepository
	sessionRepo *repository.SessionRepository
	jwtSvc      *auth.JWTService
	cfg         *config.Config
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(
	userRepo *repository.UserRepository,
	sessionRepo *repository.SessionRepository,
	jwtSvc *auth.JWTService,
	cfg *config.Config,
) *AuthHandler {
	return &AuthHandler{
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
		jwtSvc:      jwtSvc,
		cfg:         cfg,
	}
}

// -- Request / Response types ------------------------------------------------

type registerRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type passwordResetRequestReq struct {
	Email string `json:"email"`
}

type passwordResetConfirmReq struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

// -- Handlers ----------------------------------------------------------------

// Register creates a new user account and returns tokens.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, err)
		return
	}

	if req.Username == "" || req.Email == "" || req.Password == "" {
		respondError(w, apperrors.Validation("username, email, and password are required", nil))
		return
	}

	if err := auth.ValidatePasswordStrength(req.Password, h.cfg.Auth.PasswordMinLen); err != nil {
		respondError(w, apperrors.Validation(err.Error(), nil))
		return
	}

	// Check for existing user by email.
	if _, err := h.userRepo.GetByEmail(r.Context(), req.Email); err == nil {
		respondError(w, apperrors.Conflict("email already registered"))
		return
	}

	// Check for existing user by username.
	if _, err := h.userRepo.GetByUsername(r.Context(), req.Username); err == nil {
		respondError(w, apperrors.Conflict("username already taken"))
		return
	}

	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		respondError(w, apperrors.Internal("failed to hash password"))
		return
	}

	now := time.Now()
	user := &models.User{
		ID:        uuid.New(),
		Username:  req.Username,
		Email:     req.Email,
		Status:    "online",
		Locale:    "en-US",
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := h.userRepo.Create(r.Context(), user, passwordHash); err != nil {
		respondError(w, err)
		return
	}

	tokens, err := h.createSession(r, user.ID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, tokens)
}

// Login authenticates a user and returns tokens.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, err)
		return
	}

	if req.Email == "" || req.Password == "" {
		respondError(w, apperrors.Validation("email and password are required", nil))
		return
	}

	user, err := h.userRepo.GetByEmail(r.Context(), req.Email)
	if err != nil {
		respondError(w, apperrors.Unauthorized("invalid credentials"))
		return
	}

	if user.IsDisabled {
		respondError(w, apperrors.Forbidden("account is disabled"))
		return
	}

	hash, err := h.userRepo.GetPasswordHash(r.Context(), user.ID)
	if err != nil {
		respondError(w, apperrors.Unauthorized("invalid credentials"))
		return
	}

	ok, err := auth.VerifyPassword(req.Password, hash)
	if err != nil || !ok {
		respondError(w, apperrors.Unauthorized("invalid credentials"))
		return
	}

	tokens, err := h.createSession(r, user.ID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, tokens)
}

// Refresh rotates the refresh token and returns a new token pair.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, err)
		return
	}

	if req.RefreshToken == "" {
		respondError(w, apperrors.Validation("refresh_token is required", nil))
		return
	}

	claims, err := h.jwtSvc.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		respondError(w, apperrors.Unauthorized("invalid refresh token"))
		return
	}

	// Hash the token to look up in DB.
	tokenHash := hashToken(req.RefreshToken)
	rt, err := h.sessionRepo.GetRefreshTokenByHash(r.Context(), tokenHash)
	if err != nil {
		respondError(w, apperrors.Unauthorized("refresh token not found"))
		return
	}

	if rt.RevokedAt != nil {
		respondError(w, apperrors.Unauthorized("refresh token has been revoked"))
		return
	}

	// Revoke the old refresh token.
	if err := h.sessionRepo.RevokeRefreshToken(r.Context(), rt.ID); err != nil {
		respondError(w, err)
		return
	}

	// Create new token pair.
	accessToken, err := h.jwtSvc.GenerateAccessToken(claims.UserID, nil)
	if err != nil {
		respondError(w, apperrors.Internal("failed to generate access token"))
		return
	}

	newSessionID := uuid.New()
	refreshToken, err := h.jwtSvc.GenerateRefreshToken(claims.UserID, newSessionID)
	if err != nil {
		respondError(w, apperrors.Internal("failed to generate refresh token"))
		return
	}

	newHash := hashToken(refreshToken)
	now := time.Now()
	newRT := &models.RefreshToken{
		ID:        uuid.New(),
		UserID:    claims.UserID,
		TokenHash: newHash,
		CreatedAt: now,
		ExpiresAt: now.Add(h.cfg.Auth.RefreshTTL),
	}
	if err := h.sessionRepo.CreateRefreshToken(r.Context(), newRT); err != nil {
		respondError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, tokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(h.cfg.Auth.AccessTTL.Seconds()),
	})
}

// Logout revokes the refresh token / session.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req logoutRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, err)
		return
	}

	if req.RefreshToken != "" {
		tokenHash := hashToken(req.RefreshToken)
		rt, err := h.sessionRepo.GetRefreshTokenByHash(r.Context(), tokenHash)
		if err == nil {
			_ = h.sessionRepo.RevokeRefreshToken(r.Context(), rt.ID)
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// PasswordResetRequest sends a password reset email (stub).
func (h *AuthHandler) PasswordResetRequest(w http.ResponseWriter, r *http.Request) {
	var req passwordResetRequestReq
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, err)
		return
	}

	if req.Email == "" {
		respondError(w, apperrors.Validation("email is required", nil))
		return
	}

	// Always return success to avoid email enumeration.
	// In production, this would send an email with a reset link.
	respondJSON(w, http.StatusOK, map[string]string{
		"message": "if an account with that email exists, a password reset link has been sent",
	})
}

// PasswordResetConfirm resets the password using a reset token.
func (h *AuthHandler) PasswordResetConfirm(w http.ResponseWriter, r *http.Request) {
	var req passwordResetConfirmReq
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, err)
		return
	}

	if req.Token == "" || req.NewPassword == "" {
		respondError(w, apperrors.Validation("token and new_password are required", nil))
		return
	}

	if err := auth.ValidatePasswordStrength(req.NewPassword, h.cfg.Auth.PasswordMinLen); err != nil {
		respondError(w, apperrors.Validation(err.Error(), nil))
		return
	}

	// Stub: In production, validate the reset token from the DB,
	// look up the user, and update the password.
	// For now, return an error since token validation is not wired.
	respondError(w, apperrors.Validation("password reset token validation not yet implemented", nil))
}

// -- Helpers -----------------------------------------------------------------

// createSession creates a session and refresh token, returning the token pair.
func (h *AuthHandler) createSession(r *http.Request, userID uuid.UUID) (*tokenResponse, error) {
	now := time.Now()
	sessionID := uuid.New()

	ipAddr := r.RemoteAddr
	userAgent := r.UserAgent()

	session := &models.Session{
		ID:           sessionID,
		UserID:       userID,
		TokenHash:    "", // Populated below via refresh token hash.
		IPAddress:    &ipAddr,
		UserAgent:    &userAgent,
		CreatedAt:    now,
		ExpiresAt:    now.Add(h.cfg.Auth.RefreshTTL),
		LastActiveAt: now,
	}

	accessToken, err := h.jwtSvc.GenerateAccessToken(userID, nil)
	if err != nil {
		return nil, apperrors.Internal("failed to generate access token")
	}

	refreshToken, err := h.jwtSvc.GenerateRefreshToken(userID, sessionID)
	if err != nil {
		return nil, apperrors.Internal("failed to generate refresh token")
	}

	tokenHash := hashToken(refreshToken)
	session.TokenHash = tokenHash

	if err := h.sessionRepo.Create(r.Context(), session); err != nil {
		return nil, err
	}

	rt := &models.RefreshToken{
		ID:        uuid.New(),
		UserID:    userID,
		TokenHash: tokenHash,
		CreatedAt: now,
		ExpiresAt: now.Add(h.cfg.Auth.RefreshTTL),
	}
	if err := h.sessionRepo.CreateRefreshToken(r.Context(), rt); err != nil {
		return nil, err
	}

	return &tokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(h.cfg.Auth.AccessTTL.Seconds()),
	}, nil
}

// hashToken returns the hex-encoded SHA-256 hash of a token string.
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
