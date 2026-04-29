-- 000002_retention_and_upload_status.down.sql
-- The upload_status enum value is intentionally left in place because PostgreSQL
-- cannot safely remove enum values without recreating dependent objects.

DROP INDEX IF EXISTS idx_dm_messages_deleted_at;
DROP INDEX IF EXISTS idx_messages_deleted_at;

ALTER TABLE dm_messages
    DROP COLUMN IF EXISTS deleted_at;

ALTER TABLE messages
    DROP COLUMN IF EXISTS deleted_at;
