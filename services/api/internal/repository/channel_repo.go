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

type ChannelRepository struct {
	pool *pgxpool.Pool
}

func NewChannelRepository(pool *pgxpool.Pool) *ChannelRepository {
	return &ChannelRepository{pool: pool}
}

func (r *ChannelRepository) Create(ctx context.Context, channel *models.Channel) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO channels (id, guild_id, category_id, name, type, topic, position, is_nsfw, slowmode_secs, bitrate, user_limit, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		channel.ID, channel.GuildID, channel.CategoryID, channel.Name, channel.Type, channel.Topic,
		channel.Position, channel.IsNSFW, channel.SlowmodeSecs, channel.Bitrate, channel.UserLimit,
		channel.CreatedAt, channel.UpdatedAt,
	)
	if err != nil {
		return apperrors.Internal("failed to create channel")
	}
	return nil
}

func (r *ChannelRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Channel, error) {
	var c models.Channel
	err := r.pool.QueryRow(ctx, `
		SELECT id, guild_id, category_id, name, type, topic, position, is_nsfw, slowmode_secs, bitrate, user_limit, created_at, updated_at
		FROM channels
		WHERE id = $1`, id,
	).Scan(
		&c.ID, &c.GuildID, &c.CategoryID, &c.Name, &c.Type, &c.Topic,
		&c.Position, &c.IsNSFW, &c.SlowmodeSecs, &c.Bitrate, &c.UserLimit,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("channel not found")
		}
		return nil, apperrors.Internal("failed to get channel")
	}
	return &c, nil
}

func (r *ChannelRepository) Update(ctx context.Context, channel *models.Channel) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE channels
		SET guild_id = $2, category_id = $3, name = $4, type = $5, topic = $6, position = $7,
		    is_nsfw = $8, slowmode_secs = $9, bitrate = $10, user_limit = $11, updated_at = NOW()
		WHERE id = $1`,
		channel.ID, channel.GuildID, channel.CategoryID, channel.Name, channel.Type, channel.Topic,
		channel.Position, channel.IsNSFW, channel.SlowmodeSecs, channel.Bitrate, channel.UserLimit,
	)
	if err != nil {
		return apperrors.Internal("failed to update channel")
	}
	if result.RowsAffected() == 0 {
		return apperrors.NotFound("channel not found")
	}
	return nil
}

func (r *ChannelRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE channels SET updated_at = NOW(), name = name || '_deleted_' || id::text WHERE id = $1`, id,
	)
	if err != nil {
		return apperrors.Internal("failed to soft delete channel")
	}
	if result.RowsAffected() == 0 {
		return apperrors.NotFound("channel not found")
	}
	return nil
}

func (r *ChannelRepository) ListByGuild(ctx context.Context, guildID uuid.UUID) ([]models.Channel, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, guild_id, category_id, name, type, topic, position, is_nsfw, slowmode_secs, bitrate, user_limit, created_at, updated_at
		FROM channels
		WHERE guild_id = $1
		ORDER BY position ASC`, guildID,
	)
	if err != nil {
		return nil, apperrors.Internal("failed to list channels")
	}
	defer rows.Close()

	var channels []models.Channel
	for rows.Next() {
		var c models.Channel
		if err := rows.Scan(
			&c.ID, &c.GuildID, &c.CategoryID, &c.Name, &c.Type, &c.Topic,
			&c.Position, &c.IsNSFW, &c.SlowmodeSecs, &c.Bitrate, &c.UserLimit,
			&c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, apperrors.Internal("failed to scan channel")
		}
		channels = append(channels, c)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.Internal("failed to iterate channels")
	}

	return channels, nil
}

func (r *ChannelRepository) CreateCategory(ctx context.Context, cat *models.Category) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO categories (id, guild_id, name, position, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		cat.ID, cat.GuildID, cat.Name, cat.Position, cat.CreatedAt, cat.UpdatedAt,
	)
	if err != nil {
		return apperrors.Internal("failed to create category")
	}
	return nil
}

func (r *ChannelRepository) ListCategories(ctx context.Context, guildID uuid.UUID) ([]models.Category, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, guild_id, name, position, created_at, updated_at
		FROM categories
		WHERE guild_id = $1
		ORDER BY position ASC`, guildID,
	)
	if err != nil {
		return nil, apperrors.Internal("failed to list categories")
	}
	defer rows.Close()

	var categories []models.Category
	for rows.Next() {
		var c models.Category
		if err := rows.Scan(&c.ID, &c.GuildID, &c.Name, &c.Position, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, apperrors.Internal("failed to scan category")
		}
		categories = append(categories, c)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.Internal("failed to iterate categories")
	}

	return categories, nil
}

func (r *ChannelRepository) SetPermissionOverride(ctx context.Context, override *models.ChannelPermissionOverride) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO channel_permission_overrides (id, channel_id, target_type, target_id, allow, deny)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (channel_id, target_type, target_id) DO UPDATE
		SET allow = EXCLUDED.allow, deny = EXCLUDED.deny`,
		override.ID, override.ChannelID, override.TargetType, override.TargetID, override.Allow, override.Deny,
	)
	if err != nil {
		return apperrors.Internal("failed to set permission override")
	}
	return nil
}

func (r *ChannelRepository) GetPermissionOverrides(ctx context.Context, channelID uuid.UUID) ([]models.ChannelPermissionOverride, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, channel_id, target_type, target_id, allow, deny
		FROM channel_permission_overrides
		WHERE channel_id = $1`, channelID,
	)
	if err != nil {
		return nil, apperrors.Internal("failed to get permission overrides")
	}
	defer rows.Close()

	var overrides []models.ChannelPermissionOverride
	for rows.Next() {
		var o models.ChannelPermissionOverride
		if err := rows.Scan(&o.ID, &o.ChannelID, &o.TargetType, &o.TargetID, &o.Allow, &o.Deny); err != nil {
			return nil, apperrors.Internal("failed to scan permission override")
		}
		overrides = append(overrides, o)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.Internal("failed to iterate permission overrides")
	}

	return overrides, nil
}

func (r *ChannelRepository) DeletePermissionOverride(ctx context.Context, channelID uuid.UUID, targetType string, targetID uuid.UUID) error {
	result, err := r.pool.Exec(ctx, `
		DELETE FROM channel_permission_overrides
		WHERE channel_id = $1 AND target_type = $2 AND target_id = $3`,
		channelID, targetType, targetID,
	)
	if err != nil {
		return apperrors.Internal("failed to delete permission override")
	}
	if result.RowsAffected() == 0 {
		return apperrors.NotFound("permission override not found")
	}
	return nil
}
