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

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) Create(ctx context.Context, user *models.User, passwordHash string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return apperrors.Internal("failed to begin transaction")
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	_, err = tx.Exec(ctx, `
		INSERT INTO users (id, username, display_name, email, avatar_url, banner_url, bio, status, custom_status, is_bot, is_verified, is_disabled, locale, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8::user_status, $9, $10, $11, $12, $13, $14, $15)`,
		user.ID, user.Username, user.DisplayName, user.Email, user.AvatarURL, user.BannerURL, user.Bio,
		user.Status, user.CustomStatus, user.IsBot, user.IsVerified, user.IsDisabled, user.Locale,
		user.CreatedAt, user.UpdatedAt,
	)
	if err != nil {
		return apperrors.Internal("failed to insert user")
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO user_passwords (user_id, password_hash, changed_at)
		VALUES ($1, $2, NOW())`,
		user.ID, passwordHash,
	)
	if err != nil {
		return apperrors.Internal("failed to insert user password")
	}

	return tx.Commit(ctx)
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	var u models.User
	err := r.pool.QueryRow(ctx, `
		SELECT id, username, display_name, email, avatar_url, banner_url, bio, status, custom_status, is_bot, is_verified, is_disabled, locale, created_at, updated_at
		FROM users
		WHERE id = $1 AND is_disabled = false`, id,
	).Scan(
		&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.AvatarURL, &u.BannerURL, &u.Bio,
		&u.Status, &u.CustomStatus, &u.IsBot, &u.IsVerified, &u.IsDisabled, &u.Locale,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("user not found")
		}
		return nil, apperrors.Internal("failed to get user")
	}
	return &u, nil
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	var u models.User
	err := r.pool.QueryRow(ctx, `
		SELECT id, username, display_name, email, avatar_url, banner_url, bio, status, custom_status, is_bot, is_verified, is_disabled, locale, created_at, updated_at
		FROM users
		WHERE username = $1`, username,
	).Scan(
		&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.AvatarURL, &u.BannerURL, &u.Bio,
		&u.Status, &u.CustomStatus, &u.IsBot, &u.IsVerified, &u.IsDisabled, &u.Locale,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("user not found")
		}
		return nil, apperrors.Internal("failed to get user")
	}
	return &u, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	var u models.User
	err := r.pool.QueryRow(ctx, `
		SELECT id, username, display_name, email, avatar_url, banner_url, bio, status, custom_status, is_bot, is_verified, is_disabled, locale, created_at, updated_at
		FROM users
		WHERE email = $1`, email,
	).Scan(
		&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.AvatarURL, &u.BannerURL, &u.Bio,
		&u.Status, &u.CustomStatus, &u.IsBot, &u.IsVerified, &u.IsDisabled, &u.Locale,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("user not found")
		}
		return nil, apperrors.Internal("failed to get user")
	}
	return &u, nil
}

func (r *UserRepository) Update(ctx context.Context, user *models.User) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE users
		SET username = $2, display_name = $3, email = $4, avatar_url = $5, banner_url = $6, bio = $7,
		    status = $8::user_status, custom_status = $9, is_bot = $10, is_verified = $11, is_disabled = $12,
		    locale = $13, updated_at = NOW()
		WHERE id = $1`,
		user.ID, user.Username, user.DisplayName, user.Email, user.AvatarURL, user.BannerURL, user.Bio,
		user.Status, user.CustomStatus, user.IsBot, user.IsVerified, user.IsDisabled, user.Locale,
	)
	if err != nil {
		return apperrors.Internal("failed to update user")
	}
	if result.RowsAffected() == 0 {
		return apperrors.NotFound("user not found")
	}
	return nil
}

func (r *UserRepository) GetPasswordHash(ctx context.Context, userID uuid.UUID) (string, error) {
	var hash string
	err := r.pool.QueryRow(ctx, `
		SELECT password_hash FROM user_passwords WHERE user_id = $1`, userID,
	).Scan(&hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", apperrors.NotFound("password not found")
		}
		return "", apperrors.Internal("failed to get password hash")
	}
	return hash, nil
}

func (r *UserRepository) UpdatePassword(ctx context.Context, userID uuid.UUID, hash string) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE user_passwords SET password_hash = $2, changed_at = NOW() WHERE user_id = $1`,
		userID, hash,
	)
	if err != nil {
		return apperrors.Internal("failed to update password")
	}
	if result.RowsAffected() == 0 {
		return apperrors.NotFound("user not found")
	}
	return nil
}

func (r *UserRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE users SET is_disabled = true, updated_at = NOW() WHERE id = $1`, id,
	)
	if err != nil {
		return apperrors.Internal("failed to soft delete user")
	}
	if result.RowsAffected() == 0 {
		return apperrors.NotFound("user not found")
	}
	return nil
}

func (r *UserRepository) List(ctx context.Context, limit, offset int) ([]models.User, int, error) {
	var total int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE is_disabled = false`).Scan(&total)
	if err != nil {
		return nil, 0, apperrors.Internal("failed to count users")
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, username, display_name, email, avatar_url, banner_url, bio, status, custom_status, is_bot, is_verified, is_disabled, locale, created_at, updated_at
		FROM users
		WHERE is_disabled = false
		ORDER BY created_at ASC
		LIMIT $1 OFFSET $2`, limit, offset,
	)
	if err != nil {
		return nil, 0, apperrors.Internal("failed to list users")
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(
			&u.ID, &u.Username, &u.DisplayName, &u.Email, &u.AvatarURL, &u.BannerURL, &u.Bio,
			&u.Status, &u.CustomStatus, &u.IsBot, &u.IsVerified, &u.IsDisabled, &u.Locale,
			&u.CreatedAt, &u.UpdatedAt,
		); err != nil {
			return nil, 0, apperrors.Internal("failed to scan user")
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, apperrors.Internal("failed to iterate users")
	}

	return users, total, nil
}
