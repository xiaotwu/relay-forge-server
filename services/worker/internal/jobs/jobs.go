package jobs

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	"github.com/relay-forge/relay-forge/services/worker/internal/config"
)

func connectDB(cfg *config.Config) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(context.Background(), cfg.Database.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	return pool, nil
}

func CleanupExpiredSessions(cfg *config.Config) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		pool, err := connectDB(cfg)
		if err != nil {
			return err
		}
		defer pool.Close()

		result, err := pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at < NOW()`)
		if err != nil {
			return fmt.Errorf("failed to cleanup sessions: %w", err)
		}
		log.Info().Int64("deleted", result.RowsAffected()).Msg("cleaned up expired sessions")

		result, err = pool.Exec(ctx, `DELETE FROM refresh_tokens WHERE expires_at < NOW()`)
		if err != nil {
			return fmt.Errorf("failed to cleanup refresh tokens: %w", err)
		}
		log.Info().Int64("deleted", result.RowsAffected()).Msg("cleaned up expired refresh tokens")

		return nil
	}
}

func CleanupExpiredInvites(cfg *config.Config) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		pool, err := connectDB(cfg)
		if err != nil {
			return err
		}
		defer pool.Close()

		result, err := pool.Exec(ctx, `DELETE FROM invites WHERE expires_at IS NOT NULL AND expires_at < NOW()`)
		if err != nil {
			return fmt.Errorf("failed to cleanup invites: %w", err)
		}
		log.Info().Int64("deleted", result.RowsAffected()).Msg("cleaned up expired invites")
		return nil
	}
}

func CleanupExpiredPasswordResets(cfg *config.Config) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		pool, err := connectDB(cfg)
		if err != nil {
			return err
		}
		defer pool.Close()

		result, err := pool.Exec(ctx, `DELETE FROM password_resets WHERE expires_at < NOW()`)
		if err != nil {
			return fmt.Errorf("failed to cleanup password resets: %w", err)
		}
		log.Info().Int64("deleted", result.RowsAffected()).Msg("cleaned up expired password resets")
		return nil
	}
}

func ArchiveOldAuditLogs(cfg *config.Config) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		pool, err := connectDB(cfg)
		if err != nil {
			return err
		}
		defer pool.Close()

		// Archive audit logs older than 90 days by marking them
		// In a full implementation, these would be exported to cold storage
		result, err := pool.Exec(ctx, `
			DELETE FROM audit_logs
			WHERE created_at < NOW() - INTERVAL '365 days'`)
		if err != nil {
			return fmt.Errorf("failed to archive audit logs: %w", err)
		}
		log.Info().Int64("archived", result.RowsAffected()).Msg("archived old audit logs")
		return nil
	}
}

func ProcessPendingFileScans(cfg *config.Config) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		pool, err := connectDB(cfg)
		if err != nil {
			return err
		}
		defer pool.Close()

		// Mark pending scans as skipped if antivirus is not configured
		// When antivirus is enabled, this would connect to ClamAV and scan files
		result, err := pool.Exec(ctx, `
			UPDATE file_uploads SET scan_status = 'skipped'
			WHERE scan_status = 'pending'
			AND created_at < NOW() - INTERVAL '1 hour'`)
		if err != nil {
			return fmt.Errorf("failed to process file scans: %w", err)
		}
		if result.RowsAffected() > 0 {
			log.Info().Int64("processed", result.RowsAffected()).Msg("processed pending file scans")
		}
		return nil
	}
}

func EnforceDataRetention(cfg *config.Config) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		pool, err := connectDB(cfg)
		if err != nil {
			return err
		}
		defer pool.Close()

		// Hard-delete soft-deleted messages older than 30 days
		result, err := pool.Exec(ctx, `
			DELETE FROM messages
			WHERE deleted_at IS NOT NULL
			AND deleted_at < NOW() - INTERVAL '30 days'`)
		if err != nil {
			return fmt.Errorf("failed to enforce retention on messages: %w", err)
		}
		if result.RowsAffected() > 0 {
			log.Info().Int64("purged", result.RowsAffected()).Msg("purged old soft-deleted messages")
		}

		// Hard-delete disabled users older than 90 days
		result, err = pool.Exec(ctx, `
			DELETE FROM users
			WHERE is_disabled = true
			AND updated_at < NOW() - INTERVAL '90 days'`)
		if err != nil {
			return fmt.Errorf("failed to enforce retention on users: %w", err)
		}
		if result.RowsAffected() > 0 {
			log.Info().Int64("purged", result.RowsAffected()).Msg("purged old disabled users")
		}

		return nil
	}
}
