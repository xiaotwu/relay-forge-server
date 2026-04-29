-- 000002_retention_and_upload_status.up.sql
-- Add explicit soft-delete timestamps and a skipped upload scan status.

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_enum e
        JOIN pg_type t ON t.oid = e.enumtypid
        WHERE t.typname = 'upload_status'
          AND e.enumlabel = 'skipped'
    ) THEN
        ALTER TYPE upload_status ADD VALUE 'skipped';
    END IF;
END $$;

ALTER TABLE messages
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

ALTER TABLE dm_messages
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_messages_deleted_at
    ON messages (deleted_at)
    WHERE is_deleted = true;

CREATE INDEX IF NOT EXISTS idx_dm_messages_deleted_at
    ON dm_messages (deleted_at)
    WHERE is_deleted = true;
