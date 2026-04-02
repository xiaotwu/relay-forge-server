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

type RoleRepository struct {
	pool *pgxpool.Pool
}

func NewRoleRepository(pool *pgxpool.Pool) *RoleRepository {
	return &RoleRepository{pool: pool}
}

func (r *RoleRepository) Create(ctx context.Context, role *models.Role) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO roles (id, guild_id, name, color, position, permissions, is_default, is_mentionable, icon_url, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		role.ID, role.GuildID, role.Name, role.Color, role.Position, role.Permissions,
		role.IsDefault, role.IsMentionable, role.IconURL, role.CreatedAt, role.UpdatedAt,
	)
	if err != nil {
		return apperrors.Internal("failed to create role")
	}
	return nil
}

func (r *RoleRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Role, error) {
	var role models.Role
	err := r.pool.QueryRow(ctx, `
		SELECT id, guild_id, name, color, position, permissions, is_default, is_mentionable, icon_url, created_at, updated_at
		FROM roles
		WHERE id = $1`, id,
	).Scan(
		&role.ID, &role.GuildID, &role.Name, &role.Color, &role.Position, &role.Permissions,
		&role.IsDefault, &role.IsMentionable, &role.IconURL, &role.CreatedAt, &role.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("role not found")
		}
		return nil, apperrors.Internal("failed to get role")
	}
	return &role, nil
}

func (r *RoleRepository) Update(ctx context.Context, role *models.Role) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE roles
		SET name = $2, color = $3, position = $4, permissions = $5, is_default = $6,
		    is_mentionable = $7, icon_url = $8, updated_at = NOW()
		WHERE id = $1`,
		role.ID, role.Name, role.Color, role.Position, role.Permissions,
		role.IsDefault, role.IsMentionable, role.IconURL,
	)
	if err != nil {
		return apperrors.Internal("failed to update role")
	}
	if result.RowsAffected() == 0 {
		return apperrors.NotFound("role not found")
	}
	return nil
}

func (r *RoleRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.pool.Exec(ctx, `DELETE FROM roles WHERE id = $1`, id)
	if err != nil {
		return apperrors.Internal("failed to delete role")
	}
	if result.RowsAffected() == 0 {
		return apperrors.NotFound("role not found")
	}
	return nil
}

func (r *RoleRepository) ListByGuild(ctx context.Context, guildID uuid.UUID) ([]models.Role, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, guild_id, name, color, position, permissions, is_default, is_mentionable, icon_url, created_at, updated_at
		FROM roles
		WHERE guild_id = $1
		ORDER BY position ASC`, guildID,
	)
	if err != nil {
		return nil, apperrors.Internal("failed to list roles")
	}
	defer rows.Close()

	var roles []models.Role
	for rows.Next() {
		var role models.Role
		if err := rows.Scan(
			&role.ID, &role.GuildID, &role.Name, &role.Color, &role.Position, &role.Permissions,
			&role.IsDefault, &role.IsMentionable, &role.IconURL, &role.CreatedAt, &role.UpdatedAt,
		); err != nil {
			return nil, apperrors.Internal("failed to scan role")
		}
		roles = append(roles, role)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.Internal("failed to iterate roles")
	}

	return roles, nil
}

func (r *RoleRepository) AssignRole(ctx context.Context, guildID, userID, roleID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO guild_member_roles (guild_id, user_id, role_id, assigned_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (guild_id, user_id, role_id) DO NOTHING`,
		guildID, userID, roleID,
	)
	if err != nil {
		return apperrors.Internal("failed to assign role")
	}
	return nil
}

func (r *RoleRepository) RemoveRole(ctx context.Context, guildID, userID, roleID uuid.UUID) error {
	result, err := r.pool.Exec(ctx, `
		DELETE FROM guild_member_roles WHERE guild_id = $1 AND user_id = $2 AND role_id = $3`,
		guildID, userID, roleID,
	)
	if err != nil {
		return apperrors.Internal("failed to remove role")
	}
	if result.RowsAffected() == 0 {
		return apperrors.NotFound("role assignment not found")
	}
	return nil
}

func (r *RoleRepository) GetUserRoles(ctx context.Context, guildID, userID uuid.UUID) ([]models.Role, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT r.id, r.guild_id, r.name, r.color, r.position, r.permissions, r.is_default, r.is_mentionable, r.icon_url, r.created_at, r.updated_at
		FROM roles r
		INNER JOIN guild_member_roles gmr ON r.id = gmr.role_id
		WHERE gmr.guild_id = $1 AND gmr.user_id = $2
		ORDER BY r.position ASC`, guildID, userID,
	)
	if err != nil {
		return nil, apperrors.Internal("failed to get user roles")
	}
	defer rows.Close()

	var roles []models.Role
	for rows.Next() {
		var role models.Role
		if err := rows.Scan(
			&role.ID, &role.GuildID, &role.Name, &role.Color, &role.Position, &role.Permissions,
			&role.IsDefault, &role.IsMentionable, &role.IconURL, &role.CreatedAt, &role.UpdatedAt,
		); err != nil {
			return nil, apperrors.Internal("failed to scan role")
		}
		roles = append(roles, role)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.Internal("failed to iterate roles")
	}

	return roles, nil
}

func (r *RoleRepository) GetUserPermissions(ctx context.Context, guildID, userID uuid.UUID) (int64, error) {
	var permissions *int64
	err := r.pool.QueryRow(ctx, `
		SELECT BIT_OR(r.permissions)
		FROM roles r
		INNER JOIN guild_member_roles gmr ON r.id = gmr.role_id
		WHERE gmr.guild_id = $1 AND gmr.user_id = $2`,
		guildID, userID,
	).Scan(&permissions)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, apperrors.Internal("failed to get user permissions")
	}
	if permissions == nil {
		return 0, nil
	}
	return *permissions, nil
}
