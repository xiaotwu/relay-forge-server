-- 000003_media_acl.down.sql

DROP INDEX IF EXISTS idx_file_uploads_owner;

ALTER TABLE file_uploads
    DROP CONSTRAINT IF EXISTS file_uploads_owner_type_check,
    DROP COLUMN IF EXISTS completed_at,
    DROP COLUMN IF EXISTS owner_id,
    DROP COLUMN IF EXISTS owner_type;
