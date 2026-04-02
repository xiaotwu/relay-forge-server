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

type MessageRepository struct {
	pool *pgxpool.Pool
}

func NewMessageRepository(pool *pgxpool.Pool) *MessageRepository {
	return &MessageRepository{pool: pool}
}

func (r *MessageRepository) Create(ctx context.Context, msg *models.Message) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO messages (id, channel_id, author_id, content, type, reply_to_id, is_pinned, is_edited, is_deleted, edited_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		msg.ID, msg.ChannelID, msg.AuthorID, msg.Content, msg.Type, msg.ReplyToID,
		msg.IsPinned, msg.IsEdited, msg.IsDeleted, msg.EditedAt, msg.CreatedAt,
	)
	if err != nil {
		return apperrors.Internal("failed to create message")
	}
	return nil
}

func (r *MessageRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Message, error) {
	var m models.Message
	err := r.pool.QueryRow(ctx, `
		SELECT id, channel_id, author_id, content, type, reply_to_id, is_pinned, is_edited, is_deleted, edited_at, created_at
		FROM messages
		WHERE id = $1 AND is_deleted = false`, id,
	).Scan(
		&m.ID, &m.ChannelID, &m.AuthorID, &m.Content, &m.Type, &m.ReplyToID,
		&m.IsPinned, &m.IsEdited, &m.IsDeleted, &m.EditedAt, &m.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("message not found")
		}
		return nil, apperrors.Internal("failed to get message")
	}
	return &m, nil
}

func (r *MessageRepository) Update(ctx context.Context, msg *models.Message) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE messages
		SET content = $2, is_edited = true, edited_at = NOW()
		WHERE id = $1 AND is_deleted = false`,
		msg.ID, msg.Content,
	)
	if err != nil {
		return apperrors.Internal("failed to update message")
	}
	if result.RowsAffected() == 0 {
		return apperrors.NotFound("message not found")
	}
	return nil
}

func (r *MessageRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	result, err := r.pool.Exec(ctx, `
		UPDATE messages SET is_deleted = true WHERE id = $1 AND is_deleted = false`, id,
	)
	if err != nil {
		return apperrors.Internal("failed to soft delete message")
	}
	if result.RowsAffected() == 0 {
		return apperrors.NotFound("message not found")
	}
	return nil
}

func (r *MessageRepository) HardDelete(ctx context.Context, id uuid.UUID) error {
	result, err := r.pool.Exec(ctx, `DELETE FROM messages WHERE id = $1`, id)
	if err != nil {
		return apperrors.Internal("failed to hard delete message")
	}
	if result.RowsAffected() == 0 {
		return apperrors.NotFound("message not found")
	}
	return nil
}

func (r *MessageRepository) ListByChannel(ctx context.Context, channelID uuid.UUID, before *uuid.UUID, limit int) ([]models.Message, error) {
	var rows pgx.Rows
	var err error

	if before != nil {
		rows, err = r.pool.Query(ctx, `
			SELECT id, channel_id, author_id, content, type, reply_to_id, is_pinned, is_edited, is_deleted, edited_at, created_at
			FROM messages
			WHERE channel_id = $1 AND is_deleted = false AND created_at < (SELECT created_at FROM messages WHERE id = $2)
			ORDER BY created_at DESC
			LIMIT $3`,
			channelID, *before, limit,
		)
	} else {
		rows, err = r.pool.Query(ctx, `
			SELECT id, channel_id, author_id, content, type, reply_to_id, is_pinned, is_edited, is_deleted, edited_at, created_at
			FROM messages
			WHERE channel_id = $1 AND is_deleted = false
			ORDER BY created_at DESC
			LIMIT $2`,
			channelID, limit,
		)
	}
	if err != nil {
		return nil, apperrors.Internal("failed to list messages")
	}
	defer rows.Close()

	var messages []models.Message
	for rows.Next() {
		var m models.Message
		if err := rows.Scan(
			&m.ID, &m.ChannelID, &m.AuthorID, &m.Content, &m.Type, &m.ReplyToID,
			&m.IsPinned, &m.IsEdited, &m.IsDeleted, &m.EditedAt, &m.CreatedAt,
		); err != nil {
			return nil, apperrors.Internal("failed to scan message")
		}
		messages = append(messages, m)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.Internal("failed to iterate messages")
	}

	return messages, nil
}

func (r *MessageRepository) Search(ctx context.Context, channelID uuid.UUID, query string, limit, offset int) ([]models.Message, int, error) {
	var total int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM messages
		WHERE channel_id = $1 AND is_deleted = false AND content ILIKE '%' || $2 || '%'`,
		channelID, query,
	).Scan(&total)
	if err != nil {
		return nil, 0, apperrors.Internal("failed to count search results")
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, channel_id, author_id, content, type, reply_to_id, is_pinned, is_edited, is_deleted, edited_at, created_at
		FROM messages
		WHERE channel_id = $1 AND is_deleted = false AND content ILIKE '%' || $2 || '%'
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4`,
		channelID, query, limit, offset,
	)
	if err != nil {
		return nil, 0, apperrors.Internal("failed to search messages")
	}
	defer rows.Close()

	var messages []models.Message
	for rows.Next() {
		var m models.Message
		if err := rows.Scan(
			&m.ID, &m.ChannelID, &m.AuthorID, &m.Content, &m.Type, &m.ReplyToID,
			&m.IsPinned, &m.IsEdited, &m.IsDeleted, &m.EditedAt, &m.CreatedAt,
		); err != nil {
			return nil, 0, apperrors.Internal("failed to scan message")
		}
		messages = append(messages, m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, apperrors.Internal("failed to iterate messages")
	}

	return messages, total, nil
}

func (r *MessageRepository) Pin(ctx context.Context, channelID, messageID, pinnedBy uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return apperrors.Internal("failed to begin transaction")
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	result, err := tx.Exec(ctx, `
		UPDATE messages SET is_pinned = true WHERE id = $1 AND channel_id = $2 AND is_deleted = false`,
		messageID, channelID,
	)
	if err != nil {
		return apperrors.Internal("failed to pin message")
	}
	if result.RowsAffected() == 0 {
		return apperrors.NotFound("message not found")
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO pins (id, channel_id, message_id, pinned_by, pinned_at)
		VALUES ($1, $2, $3, $4, NOW())`,
		uuid.New(), channelID, messageID, pinnedBy,
	)
	if err != nil {
		return apperrors.Internal("failed to create pin record")
	}

	return tx.Commit(ctx)
}

func (r *MessageRepository) Unpin(ctx context.Context, channelID, messageID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return apperrors.Internal("failed to begin transaction")
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	result, err := tx.Exec(ctx, `
		UPDATE messages SET is_pinned = false WHERE id = $1 AND channel_id = $2`,
		messageID, channelID,
	)
	if err != nil {
		return apperrors.Internal("failed to unpin message")
	}
	if result.RowsAffected() == 0 {
		return apperrors.NotFound("message not found")
	}

	_, err = tx.Exec(ctx, `
		DELETE FROM pins WHERE channel_id = $1 AND message_id = $2`,
		channelID, messageID,
	)
	if err != nil {
		return apperrors.Internal("failed to delete pin record")
	}

	return tx.Commit(ctx)
}

func (r *MessageRepository) ListPins(ctx context.Context, channelID uuid.UUID) ([]models.Message, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT m.id, m.channel_id, m.author_id, m.content, m.type, m.reply_to_id, m.is_pinned, m.is_edited, m.is_deleted, m.edited_at, m.created_at
		FROM messages m
		INNER JOIN pins p ON m.id = p.message_id AND m.channel_id = p.channel_id
		WHERE p.channel_id = $1 AND m.is_deleted = false
		ORDER BY p.pinned_at DESC`, channelID,
	)
	if err != nil {
		return nil, apperrors.Internal("failed to list pins")
	}
	defer rows.Close()

	var messages []models.Message
	for rows.Next() {
		var m models.Message
		if err := rows.Scan(
			&m.ID, &m.ChannelID, &m.AuthorID, &m.Content, &m.Type, &m.ReplyToID,
			&m.IsPinned, &m.IsEdited, &m.IsDeleted, &m.EditedAt, &m.CreatedAt,
		); err != nil {
			return nil, apperrors.Internal("failed to scan pinned message")
		}
		messages = append(messages, m)
	}
	if err := rows.Err(); err != nil {
		return nil, apperrors.Internal("failed to iterate pinned messages")
	}

	return messages, nil
}

func (r *MessageRepository) AddReaction(ctx context.Context, messageID, userID uuid.UUID, emojiID *uuid.UUID, emojiNative string) error {
	emoji := emojiNative
	if emojiID != nil {
		emoji = emojiID.String()
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO message_reactions (message_id, user_id, emoji, created_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (message_id, user_id, emoji) DO NOTHING`,
		messageID, userID, emoji,
	)
	if err != nil {
		return apperrors.Internal("failed to add reaction")
	}
	return nil
}

func (r *MessageRepository) RemoveReaction(ctx context.Context, messageID, userID uuid.UUID, emojiID *uuid.UUID, emojiNative string) error {
	emoji := emojiNative
	if emojiID != nil {
		emoji = emojiID.String()
	}
	result, err := r.pool.Exec(ctx, `
		DELETE FROM message_reactions WHERE message_id = $1 AND user_id = $2 AND emoji = $3`,
		messageID, userID, emoji,
	)
	if err != nil {
		return apperrors.Internal("failed to remove reaction")
	}
	if result.RowsAffected() == 0 {
		return apperrors.NotFound("reaction not found")
	}
	return nil
}

func (r *MessageRepository) GetReadState(ctx context.Context, userID, channelID uuid.UUID) (*models.ReadState, error) {
	var rs models.ReadState
	err := r.pool.QueryRow(ctx, `
		SELECT channel_id, user_id, last_message_id, mention_count, updated_at
		FROM read_states
		WHERE user_id = $1 AND channel_id = $2`,
		userID, channelID,
	).Scan(&rs.ChannelID, &rs.UserID, &rs.LastMessageID, &rs.MentionCount, &rs.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NotFound("read state not found")
		}
		return nil, apperrors.Internal("failed to get read state")
	}
	return &rs, nil
}

func (r *MessageRepository) UpdateReadState(ctx context.Context, userID, channelID, lastMessageID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO read_states (channel_id, user_id, last_message_id, mention_count, updated_at)
		VALUES ($1, $2, $3, 0, NOW())
		ON CONFLICT (channel_id, user_id) DO UPDATE
		SET last_message_id = EXCLUDED.last_message_id, mention_count = 0, updated_at = NOW()`,
		channelID, userID, lastMessageID,
	)
	if err != nil {
		return apperrors.Internal("failed to update read state")
	}
	return nil
}
