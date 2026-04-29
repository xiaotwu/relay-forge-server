-- 000003_media_acl.up.sql
-- Track the authorization context that owns an uploaded media object.

ALTER TABLE file_uploads
    ADD COLUMN IF NOT EXISTS owner_type VARCHAR(32) NOT NULL DEFAULT 'pending',
    ADD COLUMN IF NOT EXISTS owner_id UUID,
    ADD COLUMN IF NOT EXISTS completed_at TIMESTAMPTZ;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'file_uploads_owner_type_check'
    ) THEN
        ALTER TABLE file_uploads
            ADD CONSTRAINT file_uploads_owner_type_check
            CHECK (owner_type IN ('pending', 'dm_channel', 'channel', 'guild', 'user_profile'));
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_file_uploads_owner
    ON file_uploads (owner_type, owner_id);
