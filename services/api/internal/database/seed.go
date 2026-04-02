package database

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/argon2"
)

// Argon2id parameters following OWASP recommendations.
const (
	argonTime    = 1
	argonMemory  = 64 * 1024 // 64 MB
	argonThreads = 4
	argonKeyLen  = 32
	argonSaltLen = 16
)

// hashPassword produces an Argon2id hash with an embedded random salt.
func hashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generating salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	encodedSalt := base64.RawStdEncoding.EncodeToString(salt)
	encodedHash := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		argonMemory, argonTime, argonThreads, encodedSalt, encodedHash), nil
}

// Seed populates the database with test data for development.
func Seed(pool *pgxpool.Pool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("beginning seed transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// --- Users ---
	adminPassHash, err := hashPassword("admin123")
	if err != nil {
		return fmt.Errorf("hashing admin password: %w", err)
	}
	user1PassHash, err := hashPassword("user123")
	if err != nil {
		return fmt.Errorf("hashing user1 password: %w", err)
	}

	var adminID, user1ID string

	err = tx.QueryRow(ctx, `
		INSERT INTO users (username, display_name, email, status, is_bot)
		VALUES ('admin', 'Admin', 'admin@relayforge.local', 'online', false)
		ON CONFLICT (username) DO UPDATE SET username = EXCLUDED.username
		RETURNING id::text
	`).Scan(&adminID)
	if err != nil {
		return fmt.Errorf("inserting admin user: %w", err)
	}

	err = tx.QueryRow(ctx, `
		INSERT INTO users (username, display_name, email, status, is_bot)
		VALUES ('user1', 'User One', 'user1@relayforge.local', 'online', false)
		ON CONFLICT (username) DO UPDATE SET username = EXCLUDED.username
		RETURNING id::text
	`).Scan(&user1ID)
	if err != nil {
		return fmt.Errorf("inserting user1: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO user_passwords (user_id, password_hash)
		VALUES ($1::uuid, $2), ($3::uuid, $4)
		ON CONFLICT (user_id) DO UPDATE SET password_hash = EXCLUDED.password_hash
	`, adminID, adminPassHash, user1ID, user1PassHash)
	if err != nil {
		return fmt.Errorf("inserting passwords: %w", err)
	}

	log.Info().Str("admin_id", adminID).Str("user1_id", user1ID).Msg("seeded users")

	// --- Guild ---
	var guildID string
	err = tx.QueryRow(ctx, `
		INSERT INTO guilds (name, owner_id, description)
		VALUES ('General', $1::uuid, 'The default guild for RelayForge')
		ON CONFLICT DO NOTHING
		RETURNING id::text
	`, adminID).Scan(&guildID)
	if err != nil {
		return fmt.Errorf("inserting guild: %w", err)
	}

	log.Info().Str("guild_id", guildID).Msg("seeded guild")

	// --- Roles ---
	var everyoneRoleID, adminRoleID string

	err = tx.QueryRow(ctx, `
		INSERT INTO roles (guild_id, name, color, position, permissions, is_default)
		VALUES ($1::uuid, '@everyone', '#99AAB5', 0, 104324673, true)
		RETURNING id::text
	`, guildID).Scan(&everyoneRoleID)
	if err != nil {
		return fmt.Errorf("inserting @everyone role: %w", err)
	}

	err = tx.QueryRow(ctx, `
		INSERT INTO roles (guild_id, name, color, position, permissions, is_default)
		VALUES ($1::uuid, 'Admin', '#E74C3C', 1, 2147483647, false)
		RETURNING id::text
	`, guildID).Scan(&adminRoleID)
	if err != nil {
		return fmt.Errorf("inserting Admin role: %w", err)
	}

	log.Info().Str("everyone_role", everyoneRoleID).Str("admin_role", adminRoleID).Msg("seeded roles")

	// --- Guild Members ---
	_, err = tx.Exec(ctx, `
		INSERT INTO guild_members (guild_id, user_id, nickname)
		VALUES ($1::uuid, $2::uuid, 'Admin'), ($1::uuid, $3::uuid, 'User One')
		ON CONFLICT DO NOTHING
	`, guildID, adminID, user1ID)
	if err != nil {
		return fmt.Errorf("inserting guild members: %w", err)
	}

	// Assign Admin role to admin user
	_, err = tx.Exec(ctx, `
		INSERT INTO guild_member_roles (guild_id, user_id, role_id)
		VALUES ($1::uuid, $2::uuid, $3::uuid)
		ON CONFLICT DO NOTHING
	`, guildID, adminID, adminRoleID)
	if err != nil {
		return fmt.Errorf("assigning admin role: %w", err)
	}

	// --- Channels ---
	var generalChID, voiceChID string

	err = tx.QueryRow(ctx, `
		INSERT INTO channels (guild_id, name, type, topic, position)
		VALUES ($1::uuid, 'general', 'text', 'General discussion', 0)
		RETURNING id::text
	`, guildID).Scan(&generalChID)
	if err != nil {
		return fmt.Errorf("inserting #general channel: %w", err)
	}

	err = tx.QueryRow(ctx, `
		INSERT INTO channels (guild_id, name, type, topic, position)
		VALUES ($1::uuid, 'voice', 'voice', 'Voice chat', 1)
		RETURNING id::text
	`, guildID).Scan(&voiceChID)
	if err != nil {
		return fmt.Errorf("inserting #voice channel: %w", err)
	}

	log.Info().Str("general", generalChID).Str("voice", voiceChID).Msg("seeded channels")

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("committing seed transaction: %w", err)
	}

	log.Info().Msg("database seeding completed")
	return nil
}
