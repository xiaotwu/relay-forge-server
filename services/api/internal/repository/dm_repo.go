package repository

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "github.com/relay-forge/relay-forge/services/api/internal/errors"
)

type DMPublicUser struct {
	ID           uuid.UUID `json:"id"`
	Username     string    `json:"username"`
	DisplayName  string    `json:"display_name"`
	AvatarURL    *string   `json:"avatar_url,omitempty"`
	BannerURL    *string   `json:"banner_url,omitempty"`
	Bio          *string   `json:"bio,omitempty"`
	Status       string    `json:"status"`
	CustomStatus *string   `json:"custom_status,omitempty"`
}

type DMChannelView struct {
	ID            uuid.UUID      `json:"id"`
	Type          string         `json:"type"`
	Participants  []DMPublicUser `json:"participants"`
	Name          *string        `json:"name,omitempty"`
	IconURL       *string        `json:"icon_url,omitempty"`
	OwnerID       *uuid.UUID     `json:"owner_id,omitempty"`
	LastMessage   *string        `json:"last_message,omitempty"`
	LastMessageAt *time.Time     `json:"last_message_at,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

type DMMessageView struct {
	ID        uuid.UUID    `json:"id"`
	ChannelID uuid.UUID    `json:"channel_id"`
	AuthorID  uuid.UUID    `json:"author_id"`
	Author    DMPublicUser `json:"author"`
	Content   string       `json:"content"`
	ReplyToID *uuid.UUID   `json:"reply_to_id,omitempty"`
	Edited    bool         `json:"edited"`
	EditedAt  *time.Time   `json:"edited_at,omitempty"`
	CreatedAt time.Time    `json:"created_at"`
}

type DMRepository struct {
	pool *pgxpool.Pool
}

func NewDMRepository(pool *pgxpool.Pool) *DMRepository {
	return &DMRepository{pool: pool}
}

func (r *DMRepository) ListChannelsForUser(ctx context.Context, userID uuid.UUID) ([]DMChannelView, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT c.id, c.type, c.name, c.icon_url, c.owner_id, c.created_at, c.updated_at, lm.content, lm.created_at
		FROM dm_channels c
		INNER JOIN dm_channel_members self_member ON self_member.dm_channel_id = c.id AND self_member.user_id = $1
		LEFT JOIN LATERAL (
			SELECT content, created_at
			FROM dm_messages
			WHERE dm_channel_id = c.id AND is_deleted = false
			ORDER BY created_at DESC
			LIMIT 1
		) lm ON true
		ORDER BY COALESCE(lm.created_at, c.updated_at) DESC, c.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, apperrors.Internal("failed to list direct messages")
	}
	defer rows.Close()

	var channels []DMChannelView
	for rows.Next() {
		var view DMChannelView
		if err := rows.Scan(
			&view.ID,
			&view.Type,
			&view.Name,
			&view.IconURL,
			&view.OwnerID,
			&view.CreatedAt,
			&view.UpdatedAt,
			&view.LastMessage,
			&view.LastMessageAt,
		); err != nil {
			return nil, apperrors.Internal("failed to scan direct messages")
		}

		view.Participants, err = r.listParticipants(ctx, view.ID)
		if err != nil {
			return nil, err
		}
		channels = append(channels, view)
	}

	if err := rows.Err(); err != nil {
		return nil, apperrors.Internal("failed to iterate direct messages")
	}

	return channels, nil
}

func (r *DMRepository) CreateOrGetDirectChannel(
	ctx context.Context,
	requesterID uuid.UUID,
	participantIDs []uuid.UUID,
	name *string,
) (*DMChannelView, error) {
	uniqueMembers := []uuid.UUID{requesterID}
	seen := map[uuid.UUID]struct{}{requesterID: {}}
	for _, participantID := range participantIDs {
		if _, exists := seen[participantID]; exists {
			continue
		}
		seen[participantID] = struct{}{}
		uniqueMembers = append(uniqueMembers, participantID)
	}

	if len(uniqueMembers) < 2 {
		return nil, apperrors.Validation("choose at least one other participant", nil)
	}

	channelType := "group_dm"
	if len(uniqueMembers) == 2 {
		channelType = "dm"
	}

	if channelType == "dm" {
		otherUserID := uniqueMembers[1]
		var existingID uuid.UUID
		err := r.pool.QueryRow(ctx, `
			SELECT c.id
			FROM dm_channels c
			INNER JOIN dm_channel_members m1 ON m1.dm_channel_id = c.id AND m1.user_id = $1
			INNER JOIN dm_channel_members m2 ON m2.dm_channel_id = c.id AND m2.user_id = $2
			WHERE c.type = 'dm'
			LIMIT 1`,
			requesterID, otherUserID,
		).Scan(&existingID)
		if err == nil {
			return r.GetChannelForUser(ctx, existingID, requesterID)
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.Internal("failed to look up direct message")
		}
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, apperrors.Internal("failed to begin direct message transaction")
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	channelID := uuid.New()
	var createdAt time.Time
	var updatedAt time.Time
	if err := tx.QueryRow(ctx, `
		INSERT INTO dm_channels (id, type, name, owner_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		RETURNING created_at, updated_at`,
		channelID, channelType, name, requesterID,
	).Scan(&createdAt, &updatedAt); err != nil {
		return nil, apperrors.Internal("failed to create direct message")
	}

	sort.Slice(uniqueMembers, func(i, j int) bool {
		return uniqueMembers[i].String() < uniqueMembers[j].String()
	})
	for _, memberID := range uniqueMembers {
		if _, err := tx.Exec(ctx, `
			INSERT INTO dm_channel_members (dm_channel_id, user_id, joined_at, is_muted)
			VALUES ($1, $2, NOW(), false)`,
			channelID, memberID,
		); err != nil {
			return nil, apperrors.Internal("failed to add direct message participant")
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, apperrors.Internal("failed to finalize direct message")
	}

	return &DMChannelView{
		ID:        channelID,
		Type:      channelType,
		Name:      name,
		OwnerID:   &requesterID,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}, nil
}

func (r *DMRepository) UpdateChannel(
	ctx context.Context,
	channelID uuid.UUID,
	userID uuid.UUID,
	name *string,
) (*DMChannelView, error) {
	member, err := r.IsMember(ctx, channelID, userID)
	if err != nil {
		return nil, err
	}
	if !member {
		return nil, apperrors.Forbidden("cannot edit this conversation")
	}

	result, err := r.pool.Exec(ctx, `
		UPDATE dm_channels
		SET name = $3, updated_at = NOW()
		WHERE id = $1 AND owner_id = $2 AND type = 'group_dm'`,
		channelID, userID, name,
	)
	if err != nil {
		return nil, apperrors.Internal("failed to update conversation")
	}
	if result.RowsAffected() == 0 {
		return nil, apperrors.Forbidden("only the group creator can edit this conversation")
	}

	return r.GetChannelForUser(ctx, channelID, userID)
}

func (r *DMRepository) GetChannelForUser(
	ctx context.Context,
	channelID uuid.UUID,
	userID uuid.UUID,
) (*DMChannelView, error) {
	member, err := r.IsMember(ctx, channelID, userID)
	if err != nil {
		return nil, err
	}
	if !member {
		return nil, apperrors.Forbidden("cannot access this direct message")
	}

	var view DMChannelView
	if err := r.pool.QueryRow(ctx, `
		SELECT c.id, c.type, c.name, c.icon_url, c.owner_id, c.created_at, c.updated_at, lm.content, lm.created_at
		FROM dm_channels c
		LEFT JOIN LATERAL (
			SELECT content, created_at
			FROM dm_messages
			WHERE dm_channel_id = c.id AND is_deleted = false
			ORDER BY created_at DESC
			LIMIT 1
		) lm ON true
		WHERE c.id = $1`,
		channelID,
	).Scan(
		&view.ID,
		&view.Type,
		&view.Name,
		&view.IconURL,
		&view.OwnerID,
		&view.CreatedAt,
		&view.UpdatedAt,
		&view.LastMessage,
		&view.LastMessageAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("direct message not found")
		}
		return nil, apperrors.Internal("failed to load direct message")
	}

	view.Participants, err = r.listParticipants(ctx, channelID)
	if err != nil {
		return nil, err
	}

	return &view, nil
}

func (r *DMRepository) IsMember(ctx context.Context, channelID, userID uuid.UUID) (bool, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM dm_channel_members
			WHERE dm_channel_id = $1 AND user_id = $2
		)`,
		channelID, userID,
	).Scan(&exists); err != nil {
		return false, apperrors.Internal("failed to validate direct message membership")
	}
	return exists, nil
}

func (r *DMRepository) ListMemberIDs(ctx context.Context, channelID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT m.user_id
		FROM dm_channel_members m
		INNER JOIN users u ON u.id = m.user_id
		WHERE m.dm_channel_id = $1 AND u.is_disabled = false`,
		channelID,
	)
	if err != nil {
		return nil, apperrors.Internal("failed to load direct message recipients")
	}
	defer rows.Close()

	userIDs := make([]uuid.UUID, 0)
	for rows.Next() {
		var userID uuid.UUID
		if err := rows.Scan(&userID); err != nil {
			return nil, apperrors.Internal("failed to scan direct message recipient")
		}
		userIDs = append(userIDs, userID)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.Internal("failed to iterate direct message recipients")
	}

	return userIDs, nil
}

func (r *DMRepository) ListMessages(
	ctx context.Context,
	channelID uuid.UUID,
	userID uuid.UUID,
) ([]DMMessageView, error) {
	member, err := r.IsMember(ctx, channelID, userID)
	if err != nil {
		return nil, err
	}
	if !member {
		return nil, apperrors.Forbidden("cannot access messages in this direct message")
	}

	rows, err := r.pool.Query(ctx, `
		SELECT
			m.id,
			m.dm_channel_id,
			m.author_id,
			COALESCE(m.content, ''),
			m.reply_to_id,
			m.is_edited,
			m.edited_at,
			m.created_at,
			u.id,
			u.username,
			COALESCE(u.display_name, u.username),
			u.avatar_url,
			u.banner_url,
			u.bio,
			u.status,
			u.custom_status
		FROM dm_messages m
		INNER JOIN users u ON u.id = m.author_id
		WHERE m.dm_channel_id = $1 AND m.is_deleted = false
		ORDER BY m.created_at ASC`,
		channelID,
	)
	if err != nil {
		return nil, apperrors.Internal("failed to list direct messages")
	}
	defer rows.Close()

	var messages []DMMessageView
	for rows.Next() {
		var message DMMessageView
		if err := rows.Scan(
			&message.ID,
			&message.ChannelID,
			&message.AuthorID,
			&message.Content,
			&message.ReplyToID,
			&message.Edited,
			&message.EditedAt,
			&message.CreatedAt,
			&message.Author.ID,
			&message.Author.Username,
			&message.Author.DisplayName,
			&message.Author.AvatarURL,
			&message.Author.BannerURL,
			&message.Author.Bio,
			&message.Author.Status,
			&message.Author.CustomStatus,
		); err != nil {
			return nil, apperrors.Internal("failed to scan direct message")
		}
		messages = append(messages, message)
	}

	if err := rows.Err(); err != nil {
		return nil, apperrors.Internal("failed to iterate direct messages")
	}

	return messages, nil
}

func (r *DMRepository) CreateMessage(
	ctx context.Context,
	channelID uuid.UUID,
	userID uuid.UUID,
	content string,
	replyToID *uuid.UUID,
) (*DMMessageView, error) {
	member, err := r.IsMember(ctx, channelID, userID)
	if err != nil {
		return nil, err
	}
	if !member {
		return nil, apperrors.Forbidden("cannot send messages in this direct message")
	}

	messageID := uuid.New()
	if _, err := r.pool.Exec(ctx, `
		INSERT INTO dm_messages (id, dm_channel_id, author_id, content, reply_to_id, is_edited, is_deleted, edited_at, created_at)
		VALUES ($1, $2, $3, $4, $5, false, false, NULL, NOW())`,
		messageID, channelID, userID, content, replyToID,
	); err != nil {
		return nil, apperrors.Internal("failed to create direct message")
	}

	if _, err := r.pool.Exec(ctx, `
		UPDATE dm_channels
		SET updated_at = NOW()
		WHERE id = $1`,
		channelID,
	); err != nil {
		return nil, apperrors.Internal("failed to update direct message activity")
	}

	var message DMMessageView
	if err := r.pool.QueryRow(ctx, `
		SELECT
			m.id,
			m.dm_channel_id,
			m.author_id,
			COALESCE(m.content, ''),
			m.reply_to_id,
			m.is_edited,
			m.edited_at,
			m.created_at,
			u.id,
			u.username,
			COALESCE(u.display_name, u.username),
			u.avatar_url,
			u.banner_url,
			u.bio,
			u.status,
			u.custom_status
		FROM dm_messages m
		INNER JOIN users u ON u.id = m.author_id
		WHERE m.id = $1`,
		messageID,
	).Scan(
		&message.ID,
		&message.ChannelID,
		&message.AuthorID,
		&message.Content,
		&message.ReplyToID,
		&message.Edited,
		&message.EditedAt,
		&message.CreatedAt,
		&message.Author.ID,
		&message.Author.Username,
		&message.Author.DisplayName,
		&message.Author.AvatarURL,
		&message.Author.BannerURL,
		&message.Author.Bio,
		&message.Author.Status,
		&message.Author.CustomStatus,
	); err != nil {
		return nil, apperrors.Internal("failed to load direct message")
	}

	return &message, nil
}

func (r *DMRepository) DeleteMessage(
	ctx context.Context,
	channelID uuid.UUID,
	messageID uuid.UUID,
	userID uuid.UUID,
) error {
	member, err := r.IsMember(ctx, channelID, userID)
	if err != nil {
		return err
	}
	if !member {
		return apperrors.Forbidden("cannot access messages in this direct message")
	}

	result, err := r.pool.Exec(ctx, `
		UPDATE dm_messages
		SET is_deleted = true, deleted_at = NOW()
		WHERE id = $1 AND dm_channel_id = $2 AND author_id = $3 AND is_deleted = false`,
		messageID, channelID, userID,
	)
	if err != nil {
		return apperrors.Internal("failed to delete direct message")
	}
	if result.RowsAffected() == 0 {
		return apperrors.Forbidden("only the sender can recall this message")
	}

	if _, err := r.pool.Exec(ctx, `
		UPDATE dm_channels
		SET updated_at = NOW()
		WHERE id = $1`,
		channelID,
	); err != nil {
		return apperrors.Internal("failed to update direct message activity")
	}

	return nil
}

func (r *DMRepository) listParticipants(ctx context.Context, channelID uuid.UUID) ([]DMPublicUser, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			u.id,
			u.username,
			COALESCE(u.display_name, u.username),
			u.avatar_url,
			u.banner_url,
			u.bio,
			u.status,
			u.custom_status
		FROM dm_channel_members m
		INNER JOIN users u ON u.id = m.user_id
		WHERE m.dm_channel_id = $1 AND u.is_disabled = false
		ORDER BY m.joined_at ASC`,
		channelID,
	)
	if err != nil {
		return nil, apperrors.Internal("failed to load direct message participants")
	}
	defer rows.Close()

	var participants []DMPublicUser
	for rows.Next() {
		var user DMPublicUser
		if err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.DisplayName,
			&user.AvatarURL,
			&user.BannerURL,
			&user.Bio,
			&user.Status,
			&user.CustomStatus,
		); err != nil {
			return nil, apperrors.Internal("failed to scan direct message participant")
		}
		participants = append(participants, user)
	}

	if err := rows.Err(); err != nil {
		return nil, apperrors.Internal("failed to iterate direct message participants")
	}

	return participants, nil
}
