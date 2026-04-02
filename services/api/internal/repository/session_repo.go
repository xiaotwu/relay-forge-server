package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/relay-forge/relay-forge/services/api/internal/errors"
	"github.com/relay-forge/relay-forge/services/api/internal/models"
)

type SessionRepository struct {
	pool *pgxpool.Pool
}

func NewSessionRepository(pool *pgxpool.Pool) *SessionRepository {
	return &SessionRepository{pool: pool}
}

func (r *SessionRepository) Create(ctx context.Context, session *models.Session) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO sessions (id, user_id, token_hash, ip_address, user_agent, created_at, expires_at, last_active_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		session.ID, session.UserID, session.TokenHash, session.IPAddress, session.UserAgent,
		session.CreatedAt, session.ExpiresAt, session.LastActiveAt,
	)
	if err != nil {
		return apperrors.Internal("failed to create session")
	}
	return nil
}

func (r *SessionRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Session, error) {
	var s models.Session
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, token_hash, ip_address, user_agent, created_at, expires_at, last_active_at
		FROM sessions
		WHERE id = $1`, id,
	).Scan(
		&s.ID, &s.UserID, &s.TokenHash, &s.IPAddress, &s.UserAgent,
		&s.CreatedAt, &s.ExpiresAt, &s.LastActiveAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("session not found")
		}
		return nil, apperrors.Internal("failed to get session")
	}
	return &s, nil
}

func (r *SessionRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]models.Session, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, user_id, token_hash, ip_address, user_agent, created_at, expires_at, last_active_at
		FROM sessions
		WHERE user_id = $1
		ORDER BY last_active_at DESC`, userID,
	)
	if err != nil {
		return nil, apperrors.Internal("failed to list sessions")
	}
	defer rows.Close()

	var sessions []models.Session
	for rows.Next() {
		var s models.Session
		if err := rows.Scan(
			&s.ID, &s.UserID, &s.TokenHash, &s.IPAddress, &s.UserAgent,
			&s.CreatedAt, &s.ExpiresAt, &s.LastActiveAt,
		); err != nil {
			return nil, apperrors.Internal("failed to scan session")
		}
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.Internal("failed to iterate sessions")
	}

	return sessions, nil
}

func (r *SessionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.pool.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, id)
	if err != nil {
		return apperrors.Internal("failed to delete session")
	}
	if result.RowsAffected() == 0 {
		return apperrors.NotFound("session not found")
	}
	return nil
}

func (r *SessionRepository) DeleteAllForUser(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1`, userID)
	if err != nil {
		return apperrors.Internal("failed to delete sessions for user")
	}
	return nil
}

func (r *SessionRepository) CreateRefreshToken(ctx context.Context, rt *models.RefreshToken) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO refresh_tokens (id, user_id, token_hash, device_id, ip_address, created_at, expires_at, revoked_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		rt.ID, rt.UserID, rt.TokenHash, rt.DeviceID, rt.IPAddress,
		rt.CreatedAt, rt.ExpiresAt, rt.RevokedAt,
	)
	if err != nil {
		return apperrors.Internal("failed to create refresh token")
	}
	return nil
}

func (r *SessionRepository) GetRefreshTokenByHash(ctx context.Context, hash string) (*models.RefreshToken, error) {
	var rt models.RefreshToken
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, token_hash, device_id, ip_address, created_at, expires_at, revoked_at
		FROM refresh_tokens
		WHERE token_hash = $1`, hash,
	).Scan(
		&rt.ID, &rt.UserID, &rt.TokenHash, &rt.DeviceID, &rt.IPAddress,
		&rt.CreatedAt, &rt.ExpiresAt, &rt.RevokedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("refresh token not found")
		}
		return nil, apperrors.Internal("failed to get refresh token")
	}
	return &rt, nil
}

func (r *SessionRepository) RevokeRefreshToken(ctx context.Context, id uuid.UUID) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = NOW() WHERE id = $1 AND revoked_at IS NULL`,
		id,
	)
	if err != nil {
		return apperrors.Internal("failed to revoke refresh token")
	}
	if result.RowsAffected() == 0 {
		return apperrors.NotFound("refresh token not found or already revoked")
	}
	return nil
}

func (r *SessionRepository) RevokeRefreshTokenFamily(ctx context.Context, family string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = NOW() WHERE family = $1 AND revoked_at IS NULL`,
		family,
	)
	if err != nil {
		return apperrors.Internal("failed to revoke refresh token family")
	}
	return nil
}
