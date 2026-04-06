package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

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
	pool        *pgxpool.Pool
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(
	userRepo *repository.UserRepository,
	sessionRepo *repository.SessionRepository,
	jwtSvc *auth.JWTService,
	cfg *config.Config,
	pool *pgxpool.Pool,
) *AuthHandler {
	return &AuthHandler{
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
		jwtSvc:      jwtSvc,
		cfg:         cfg,
		pool:        pool,
	}
}

// -- Request / Response types ------------------------------------------------

type registerRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email         string `json:"email"`
	Password      string `json:"password"`
	TwoFactorCode string `json:"two_factor_code,omitempty"`
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

func rolesForUser(user *models.User, cfg *config.Config) []string {
	if user == nil {
		return nil
	}

	for _, username := range cfg.Auth.AdminUsernames {
		if strings.EqualFold(user.Username, username) {
			return []string{"admin"}
		}
	}

	for _, email := range cfg.Auth.AdminEmails {
		if strings.EqualFold(user.Email, email) {
			return []string{"admin"}
		}
	}

	return nil
}

func shouldConcealPasswordResetLookupError(err error) bool {
	if err == nil {
		return false
	}

	appErr, ok := apperrors.IsAppError(err)
	return ok && appErr.Code == "not_found"
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

	twoFactorEnabled, secret, backupCodes, err := h.getTwoFactorState(r.Context(), user.ID)
	if err != nil {
		respondError(w, err)
		return
	}

	if twoFactorEnabled {
		if req.TwoFactorCode == "" {
			respondError(w, apperrors.Unauthorized("two-factor code required"))
			return
		}

		if auth.ValidateTOTP(secret, req.TwoFactorCode) {
			// Authenticator code is valid.
		} else if consumed, err := h.consumeBackupCode(r.Context(), user.ID, backupCodes, req.TwoFactorCode); err != nil {
			respondError(w, apperrors.Internal("failed to validate backup code"))
			return
		} else if !consumed {
			respondError(w, apperrors.Unauthorized("invalid two-factor code"))
			return
		}
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
	user, err := h.userRepo.GetByID(r.Context(), claims.UserID)
	if err != nil {
		respondError(w, err)
		return
	}

	accessToken, err := h.jwtSvc.GenerateAccessToken(claims.UserID, rolesForUser(user, h.cfg))
	if err != nil {
		respondError(w, apperrors.Internal("failed to generate access token"))
		return
	}

	refreshToken, err := h.jwtSvc.GenerateRefreshToken(claims.UserID, claims.SessionID)
	if err != nil {
		respondError(w, apperrors.Internal("failed to generate refresh token"))
		return
	}

	newHash := hashToken(refreshToken)
	now := time.Now()
	if err := h.sessionRepo.UpdateSessionTokenHash(r.Context(), claims.SessionID, newHash, now); err != nil {
		respondError(w, err)
		return
	}
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
		if claims, err := h.jwtSvc.ValidateRefreshToken(req.RefreshToken); err == nil {
			_ = h.sessionRepo.Delete(r.Context(), claims.SessionID)
		}
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

	user, err := h.userRepo.GetByEmail(r.Context(), req.Email)
	if err != nil {
		if shouldConcealPasswordResetLookupError(err) {
			respondJSON(w, http.StatusOK, map[string]string{
				"message": "if an account with that email exists, a password reset link has been sent",
			})
			return
		}

		respondError(w, err)
		return
	}

	resetToken, err := auth.GenerateOpaqueToken(32)
	if err != nil {
		respondError(w, apperrors.Internal("failed to create password reset token"))
		return
	}

	expiresAt := time.Now().Add(time.Hour)
	_, err = h.pool.Exec(r.Context(), `
		INSERT INTO password_resets (id, user_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)`,
		uuid.New(), user.ID, hashToken(resetToken), expiresAt,
	)
	if err != nil {
		respondError(w, apperrors.Internal("failed to store password reset token"))
		return
	}

	response := map[string]string{
		"message": "if an account with that email exists, a password reset link has been sent",
	}
	if h.cfg.Env != "production" {
		response["reset_token"] = resetToken
	}

	respondJSON(w, http.StatusOK, response)
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

	var reset models.PasswordReset
	err := h.pool.QueryRow(r.Context(), `
		SELECT id, user_id, token_hash, created_at, expires_at, used_at
		FROM password_resets
		WHERE token_hash = $1`,
		hashToken(req.Token),
	).Scan(
		&reset.ID,
		&reset.UserID,
		&reset.TokenHash,
		&reset.CreatedAt,
		&reset.ExpiresAt,
		&reset.UsedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			respondError(w, apperrors.Validation("password reset token is invalid or expired", nil))
			return
		}
		respondError(w, apperrors.Internal("failed to validate password reset token"))
		return
	}

	if reset.UsedAt != nil || time.Now().After(reset.ExpiresAt) {
		respondError(w, apperrors.Validation("password reset token is invalid or expired", nil))
		return
	}

	passwordHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		respondError(w, apperrors.Internal("failed to hash password"))
		return
	}

	if err := h.userRepo.UpdatePassword(r.Context(), reset.UserID, passwordHash); err != nil {
		respondError(w, err)
		return
	}

	if _, err := h.pool.Exec(r.Context(), `
		UPDATE password_resets SET used_at = NOW() WHERE id = $1`,
		reset.ID,
	); err != nil {
		respondError(w, apperrors.Internal("failed to mark password reset token as used"))
		return
	}

	_ = h.sessionRepo.DeleteAllForUser(r.Context(), reset.UserID)
	_ = h.sessionRepo.RevokeAllRefreshTokensForUser(r.Context(), reset.UserID)

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "password updated successfully",
	})
}

// -- Helpers -----------------------------------------------------------------

// createSession creates a session and refresh token, returning the token pair.
func (h *AuthHandler) createSession(r *http.Request, userID uuid.UUID) (*tokenResponse, error) {
	now := time.Now()
	sessionID := uuid.New()

	user, err := h.userRepo.GetByID(r.Context(), userID)
	if err != nil {
		return nil, err
	}

	userAgent := r.UserAgent()

	session := &models.Session{
		ID:           sessionID,
		UserID:       userID,
		TokenHash:    "", // Populated below via refresh token hash.
		IPAddress:    extractClientIP(r),
		UserAgent:    &userAgent,
		CreatedAt:    now,
		ExpiresAt:    now.Add(h.cfg.Auth.RefreshTTL),
		LastActiveAt: now,
	}

	accessToken, err := h.jwtSvc.GenerateAccessToken(userID, rolesForUser(user, h.cfg))
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

func (h *AuthHandler) getTwoFactorState(
	ctx context.Context,
	userID uuid.UUID,
) (bool, string, []string, error) {
	var secret string
	var verified bool
	var backupCodes []string
	err := h.pool.QueryRow(ctx, `
		SELECT secret, verified, backup_codes
		FROM totp_secrets
		WHERE user_id = $1`,
		userID,
	).Scan(&secret, &verified, &backupCodes)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, "", nil, nil
		}
		return false, "", nil, apperrors.Internal("failed to load two-factor settings")
	}

	if !verified {
		return false, "", nil, nil
	}

	return true, secret, backupCodes, nil
}

func (h *AuthHandler) consumeBackupCode(
	ctx context.Context,
	userID uuid.UUID,
	backupCodes []string,
	providedCode string,
) (bool, error) {
	var matchedCode string
	for _, code := range backupCodes {
		if auth.BackupCodeMatches(code, providedCode) {
			matchedCode = code
			break
		}
	}

	if matchedCode == "" {
		return false, nil
	}

	_, err := h.pool.Exec(ctx, `
		UPDATE totp_secrets
		SET backup_codes = array_remove(COALESCE(backup_codes, '{}'::text[]), $2)
		WHERE user_id = $1`,
		userID, matchedCode,
	)
	if err != nil {
		return false, err
	}

	return true, nil
}
