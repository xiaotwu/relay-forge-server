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

type GuildRepository struct {
	pool *pgxpool.Pool
}

func NewGuildRepository(pool *pgxpool.Pool) *GuildRepository {
	return &GuildRepository{pool: pool}
}

func (r *GuildRepository) Create(ctx context.Context, guild *models.Guild) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO guilds (id, name, description, icon_url, banner_url, owner_id, is_public, member_count, max_members, vanity_url, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		guild.ID, guild.Name, guild.Description, guild.IconURL, guild.BannerURL, guild.OwnerID,
		guild.IsPublic, guild.MemberCount, guild.MaxMembers, guild.VanityURL,
		guild.CreatedAt, guild.UpdatedAt,
	)
	if err != nil {
		return apperrors.Internal("failed to create guild")
	}
	return nil
}

func (r *GuildRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Guild, error) {
	var g models.Guild
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, description, icon_url, banner_url, owner_id, is_public, member_count, max_members, vanity_url, created_at, updated_at
		FROM guilds
		WHERE id = $1`, id,
	).Scan(
		&g.ID, &g.Name, &g.Description, &g.IconURL, &g.BannerURL, &g.OwnerID,
		&g.IsPublic, &g.MemberCount, &g.MaxMembers, &g.VanityURL,
		&g.CreatedAt, &g.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("guild not found")
		}
		return nil, apperrors.Internal("failed to get guild")
	}
	return &g, nil
}

func (r *GuildRepository) Update(ctx context.Context, guild *models.Guild) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE guilds
		SET name = $2, description = $3, icon_url = $4, banner_url = $5, owner_id = $6,
		    is_public = $7, member_count = $8, max_members = $9, vanity_url = $10, updated_at = NOW()
		WHERE id = $1`,
		guild.ID, guild.Name, guild.Description, guild.IconURL, guild.BannerURL, guild.OwnerID,
		guild.IsPublic, guild.MemberCount, guild.MaxMembers, guild.VanityURL,
	)
	if err != nil {
		return apperrors.Internal("failed to update guild")
	}
	if result.RowsAffected() == 0 {
		return apperrors.NotFound("guild not found")
	}
	return nil
}

func (r *GuildRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE guilds SET updated_at = NOW(), name = name || '_deleted_' || id::text WHERE id = $1`, id,
	)
	if err != nil {
		return apperrors.Internal("failed to soft delete guild")
	}
	if result.RowsAffected() == 0 {
		return apperrors.NotFound("guild not found")
	}
	return nil
}

func (r *GuildRepository) ListForUser(ctx context.Context, userID uuid.UUID) ([]models.Guild, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT g.id, g.name, g.description, g.icon_url, g.banner_url, g.owner_id, g.is_public, g.member_count, g.max_members, g.vanity_url, g.created_at, g.updated_at
		FROM guilds g
		INNER JOIN guild_members gm ON g.id = gm.guild_id
		WHERE gm.user_id = $1
		ORDER BY g.name ASC`, userID,
	)
	if err != nil {
		return nil, apperrors.Internal("failed to list guilds for user")
	}
	defer rows.Close()

	var guilds []models.Guild
	for rows.Next() {
		var g models.Guild
		if err := rows.Scan(
			&g.ID, &g.Name, &g.Description, &g.IconURL, &g.BannerURL, &g.OwnerID,
			&g.IsPublic, &g.MemberCount, &g.MaxMembers, &g.VanityURL,
			&g.CreatedAt, &g.UpdatedAt,
		); err != nil {
			return nil, apperrors.Internal("failed to scan guild")
		}
		guilds = append(guilds, g)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.Internal("failed to iterate guilds")
	}

	return guilds, nil
}

func (r *GuildRepository) AddMember(ctx context.Context, guildID, userID uuid.UUID, nickname string) error {
	var nick *string
	if nickname != "" {
		nick = &nickname
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO guild_members (guild_id, user_id, nickname, joined_at, is_muted, is_deafened)
		VALUES ($1, $2, $3, NOW(), false, false)`,
		guildID, userID, nick,
	)
	if err != nil {
		return apperrors.Internal("failed to add guild member")
	}
	return nil
}

func (r *GuildRepository) RemoveMember(ctx context.Context, guildID, userID uuid.UUID) error {
	result, err := r.pool.Exec(ctx, `
		DELETE FROM guild_members WHERE guild_id = $1 AND user_id = $2`,
		guildID, userID,
	)
	if err != nil {
		return apperrors.Internal("failed to remove guild member")
	}
	if result.RowsAffected() == 0 {
		return apperrors.NotFound("guild member not found")
	}
	return nil
}

func (r *GuildRepository) GetMember(ctx context.Context, guildID, userID uuid.UUID) (*models.GuildMember, error) {
	var m models.GuildMember
	err := r.pool.QueryRow(ctx, `
		SELECT guild_id, user_id, nickname, joined_at, is_muted, is_deafened
		FROM guild_members
		WHERE guild_id = $1 AND user_id = $2`,
		guildID, userID,
	).Scan(&m.GuildID, &m.UserID, &m.Nickname, &m.JoinedAt, &m.IsMuted, &m.IsDeafened)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("guild member not found")
		}
		return nil, apperrors.Internal("failed to get guild member")
	}
	return &m, nil
}

func (r *GuildRepository) ListMembers(ctx context.Context, guildID uuid.UUID, limit, offset int) ([]models.GuildMember, int, error) {
	var total int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM guild_members WHERE guild_id = $1`, guildID).Scan(&total)
	if err != nil {
		return nil, 0, apperrors.Internal("failed to count guild members")
	}

	rows, err := r.pool.Query(ctx, `
		SELECT guild_id, user_id, nickname, joined_at, is_muted, is_deafened
		FROM guild_members
		WHERE guild_id = $1
		ORDER BY joined_at ASC
		LIMIT $2 OFFSET $3`,
		guildID, limit, offset,
	)
	if err != nil {
		return nil, 0, apperrors.Internal("failed to list guild members")
	}
	defer rows.Close()

	var members []models.GuildMember
	for rows.Next() {
		var m models.GuildMember
		if err := rows.Scan(&m.GuildID, &m.UserID, &m.Nickname, &m.JoinedAt, &m.IsMuted, &m.IsDeafened); err != nil {
			return nil, 0, apperrors.Internal("failed to scan guild member")
		}
		members = append(members, m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, apperrors.Internal("failed to iterate guild members")
	}

	return members, total, nil
}

func (r *GuildRepository) ListAll(ctx context.Context, limit, offset int) ([]models.Guild, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, description, icon_url, banner_url, owner_id, is_public, member_count, max_members, vanity_url, created_at, updated_at
		FROM guilds
		WHERE deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`, limit, offset,
	)
	if err != nil {
		return nil, apperrors.Internal("failed to list all guilds")
	}
	defer rows.Close()

	var guilds []models.Guild
	for rows.Next() {
		var g models.Guild
		if err := rows.Scan(
			&g.ID, &g.Name, &g.Description, &g.IconURL, &g.BannerURL, &g.OwnerID,
			&g.IsPublic, &g.MemberCount, &g.MaxMembers, &g.VanityURL,
			&g.CreatedAt, &g.UpdatedAt,
		); err != nil {
			return nil, apperrors.Internal("failed to scan guild")
		}
		guilds = append(guilds, g)
	}
	return guilds, nil
}

func (r *GuildRepository) IsMember(ctx context.Context, guildID, userID uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM guild_members WHERE guild_id = $1 AND user_id = $2)`,
		guildID, userID,
	).Scan(&exists)
	if err != nil {
		return false, apperrors.Internal("failed to check guild membership")
	}
	return exists, nil
}
