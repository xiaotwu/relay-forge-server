package acl

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

const (
	permissionViewChannel int64 = 1 << 0
	permissionAdmin       int64 = 1 << 31
)

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) InsertPendingUpload(ctx context.Context, upload Upload) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO file_uploads
			(id, uploader_id, file_name, file_size, content_type, storage_key, bucket, url, status, owner_type, created_at)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, 'pending', 'pending', NOW())`,
		upload.ID,
		upload.UploaderID,
		upload.FileName,
		upload.FileSize,
		upload.ContentType,
		upload.StorageKey,
		upload.Bucket,
		upload.URL,
	)
	return err
}

func (s *PostgresStore) CompleteUpload(
	ctx context.Context,
	fileID uuid.UUID,
	userID uuid.UUID,
	ownerType string,
	ownerID uuid.UUID,
	url string,
) error {
	result, err := s.pool.Exec(ctx, `
		UPDATE file_uploads
		SET status = 'clean',
		    owner_type = $3,
		    owner_id = $4,
		    url = $5,
		    completed_at = NOW()
		WHERE id = $1
		  AND uploader_id = $2
		  AND status = 'pending'`,
		fileID,
		userID,
		ownerType,
		ownerID,
		url,
	)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrForbidden
	}
	return nil
}

func (s *PostgresStore) GetUpload(ctx context.Context, fileID uuid.UUID) (*Upload, error) {
	var upload Upload
	err := s.pool.QueryRow(ctx, `
		SELECT id, uploader_id, file_name, file_size, content_type, storage_key, bucket, url, status, owner_type, owner_id
		FROM file_uploads
		WHERE id = $1`,
		fileID,
	).Scan(
		&upload.ID,
		&upload.UploaderID,
		&upload.FileName,
		&upload.FileSize,
		&upload.ContentType,
		&upload.StorageKey,
		&upload.Bucket,
		&upload.URL,
		&upload.Status,
		&upload.OwnerType,
		&upload.OwnerID,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &upload, nil
}

func (s *PostgresStore) IsUserActive(ctx context.Context, userID uuid.UUID) (bool, error) {
	var active bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM users WHERE id = $1 AND is_disabled = false
		)`,
		userID,
	).Scan(&active)
	return active, err
}

func (s *PostgresStore) IsDMParticipant(ctx context.Context, dmChannelID, userID uuid.UUID) (bool, error) {
	var allowed bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM dm_channel_members m
			INNER JOIN users u ON u.id = m.user_id
			WHERE m.dm_channel_id = $1
			  AND m.user_id = $2
			  AND u.is_disabled = false
		)`,
		dmChannelID,
		userID,
	).Scan(&allowed)
	return allowed, err
}

func (s *PostgresStore) IsChannelMember(ctx context.Context, channelID, userID uuid.UUID) (bool, error) {
	active, err := s.IsUserActive(ctx, userID)
	if err != nil || !active {
		return false, err
	}

	var guildID uuid.UUID
	var ownerID uuid.UUID
	err = s.pool.QueryRow(ctx, `
		SELECT c.guild_id, g.owner_id
		FROM channels c
		INNER JOIN guilds g ON g.id = c.guild_id
		WHERE c.id = $1`,
		channelID,
	).Scan(&guildID, &ownerID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	if ownerID == userID {
		return true, nil
	}

	var member bool
	err = s.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM guild_members WHERE guild_id = $1 AND user_id = $2
		)`,
		guildID,
		userID,
	).Scan(&member)
	if err != nil || !member {
		return false, err
	}

	var permissions int64
	err = s.pool.QueryRow(ctx, `
		SELECT COALESCE(BIT_OR(r.permissions), 0)
		FROM roles r
		INNER JOIN guild_member_roles gmr ON gmr.role_id = r.id
		WHERE gmr.guild_id = $1 AND gmr.user_id = $2`,
		guildID,
		userID,
	).Scan(&permissions)
	if err != nil {
		return false, err
	}
	if permissions&permissionAdmin != 0 {
		return true, nil
	}

	allowed := true

	var roleAllow int64
	var roleDeny int64
	err = s.pool.QueryRow(ctx, `
		SELECT COALESCE(BIT_OR(o.allow), 0), COALESCE(BIT_OR(o.deny), 0)
		FROM channel_permission_overrides o
		INNER JOIN roles r ON r.id = o.target_id AND r.guild_id = $2
		LEFT JOIN guild_member_roles gmr
		  ON gmr.guild_id = r.guild_id
		 AND gmr.role_id = r.id
		 AND gmr.user_id = $3
		WHERE o.channel_id = $1
		  AND o.target_type = 'role'
		  AND (r.is_default = true OR gmr.user_id IS NOT NULL)`,
		channelID,
		guildID,
		userID,
	).Scan(&roleAllow, &roleDeny)
	if err != nil {
		return false, err
	}
	if roleDeny&permissionViewChannel != 0 && roleAllow&permissionViewChannel == 0 {
		allowed = false
	}
	if roleAllow&permissionViewChannel != 0 {
		allowed = true
	}

	var memberAllow int64
	var memberDeny int64
	err = s.pool.QueryRow(ctx, `
		SELECT allow, deny
		FROM channel_permission_overrides
		WHERE channel_id = $1
		  AND target_type = 'member'
		  AND target_id = $2`,
		channelID,
		userID,
	).Scan(&memberAllow, &memberDeny)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}
	if memberDeny&permissionViewChannel != 0 && memberAllow&permissionViewChannel == 0 {
		return false, nil
	}
	if memberAllow&permissionViewChannel != 0 {
		return true, nil
	}

	return allowed, nil
}

func (s *PostgresStore) IsGuildMember(ctx context.Context, guildID, userID uuid.UUID) (bool, error) {
	var allowed bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM guild_members gm
			INNER JOIN users u ON u.id = gm.user_id
			WHERE gm.guild_id = $1
			  AND gm.user_id = $2
			  AND u.is_disabled = false
		)`,
		guildID,
		userID,
	).Scan(&allowed)
	return allowed, err
}
