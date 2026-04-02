-- 000001_initial_schema.up.sql
-- RelayForge complete database schema

-- ============================================================================
-- EXTENSIONS
-- ============================================================================
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ============================================================================
-- ENUM TYPES
-- ============================================================================
CREATE TYPE user_status AS ENUM ('online', 'idle', 'dnd', 'invisible', 'offline');
CREATE TYPE channel_type AS ENUM ('text', 'voice', 'announcement', 'stage', 'forum');
CREATE TYPE message_type AS ENUM ('default', 'system', 'reply', 'thread_starter', 'pin_notice', 'member_join', 'member_leave', 'channel_name_change');
CREATE TYPE dm_channel_type AS ENUM ('dm', 'group_dm');
CREATE TYPE upload_status AS ENUM ('pending', 'scanning', 'clean', 'infected', 'error');
CREATE TYPE report_status AS ENUM ('open', 'investigating', 'resolved', 'dismissed');
CREATE TYPE invite_type AS ENUM ('guild', 'channel', 'dm');
CREATE TYPE audit_action AS ENUM (
    'guild_create', 'guild_update', 'guild_delete',
    'channel_create', 'channel_update', 'channel_delete',
    'role_create', 'role_update', 'role_delete',
    'member_kick', 'member_ban', 'member_unban', 'member_role_update',
    'message_delete', 'message_bulk_delete',
    'invite_create', 'invite_delete',
    'emoji_create', 'emoji_update', 'emoji_delete',
    'sticker_create', 'sticker_update', 'sticker_delete',
    'webhook_create', 'webhook_update', 'webhook_delete'
);

-- ============================================================================
-- USERS & AUTHENTICATION
-- ============================================================================

CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username        VARCHAR(32) NOT NULL UNIQUE,
    display_name    VARCHAR(64),
    email           VARCHAR(255) NOT NULL UNIQUE,
    avatar_url      TEXT,
    banner_url      TEXT,
    bio             VARCHAR(512),
    status          user_status NOT NULL DEFAULT 'offline',
    custom_status   VARCHAR(128),
    is_bot          BOOLEAN NOT NULL DEFAULT false,
    is_verified     BOOLEAN NOT NULL DEFAULT false,
    is_disabled     BOOLEAN NOT NULL DEFAULT false,
    locale          VARCHAR(10) NOT NULL DEFAULT 'en-US',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_email ON users (email);
CREATE INDEX idx_users_username ON users (username);
CREATE INDEX idx_users_created_at ON users (created_at);

CREATE TABLE user_passwords (
    user_id         UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    password_hash   TEXT NOT NULL,
    changed_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE sessions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash      TEXT NOT NULL UNIQUE,
    ip_address      INET,
    user_agent      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ NOT NULL,
    last_active_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sessions_user_id ON sessions (user_id);
CREATE INDEX idx_sessions_expires_at ON sessions (expires_at);

CREATE TABLE refresh_tokens (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash      TEXT NOT NULL UNIQUE,
    device_id       UUID,
    ip_address      INET,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ NOT NULL,
    revoked_at      TIMESTAMPTZ
);

CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens (user_id);
CREATE INDEX idx_refresh_tokens_token_hash ON refresh_tokens (token_hash);

CREATE TABLE devices (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_name     VARCHAR(128) NOT NULL,
    device_type     VARCHAR(32) NOT NULL,
    push_token      TEXT,
    last_ip         INET,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_devices_user_id ON devices (user_id);

CREATE TABLE totp_secrets (
    user_id         UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    secret          TEXT NOT NULL,
    verified        BOOLEAN NOT NULL DEFAULT false,
    backup_codes    TEXT[],
    enabled_at      TIMESTAMPTZ
);

CREATE TABLE password_resets (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash      TEXT NOT NULL UNIQUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ NOT NULL,
    used_at         TIMESTAMPTZ
);

CREATE INDEX idx_password_resets_user_id ON password_resets (user_id);
CREATE INDEX idx_password_resets_token_hash ON password_resets (token_hash);

CREATE TABLE email_verifications (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    email           VARCHAR(255) NOT NULL,
    token_hash      TEXT NOT NULL UNIQUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ NOT NULL,
    verified_at     TIMESTAMPTZ
);

CREATE INDEX idx_email_verifications_user_id ON email_verifications (user_id);
CREATE INDEX idx_email_verifications_token_hash ON email_verifications (token_hash);

-- ============================================================================
-- GUILDS & MEMBERSHIP
-- ============================================================================

CREATE TABLE guilds (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(100) NOT NULL,
    description     VARCHAR(1024),
    icon_url        TEXT,
    banner_url      TEXT,
    owner_id        UUID NOT NULL REFERENCES users(id),
    is_public       BOOLEAN NOT NULL DEFAULT false,
    member_count    INT NOT NULL DEFAULT 0,
    max_members     INT NOT NULL DEFAULT 5000,
    vanity_url      VARCHAR(32) UNIQUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_guilds_owner_id ON guilds (owner_id);

CREATE TABLE guild_members (
    guild_id        UUID NOT NULL REFERENCES guilds(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    nickname        VARCHAR(64),
    joined_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    is_muted        BOOLEAN NOT NULL DEFAULT false,
    is_deafened     BOOLEAN NOT NULL DEFAULT false,
    PRIMARY KEY (guild_id, user_id)
);

CREATE INDEX idx_guild_members_user_id ON guild_members (user_id);

-- ============================================================================
-- ROLES & PERMISSIONS
-- ============================================================================

CREATE TABLE roles (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    guild_id        UUID NOT NULL REFERENCES guilds(id) ON DELETE CASCADE,
    name            VARCHAR(64) NOT NULL,
    color           VARCHAR(7) NOT NULL DEFAULT '#99AAB5',
    position        INT NOT NULL DEFAULT 0,
    permissions     BIGINT NOT NULL DEFAULT 0,
    is_default      BOOLEAN NOT NULL DEFAULT false,
    is_mentionable  BOOLEAN NOT NULL DEFAULT true,
    icon_url        TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_roles_guild_id ON roles (guild_id);
CREATE UNIQUE INDEX idx_roles_guild_default ON roles (guild_id) WHERE is_default = true;

CREATE TABLE guild_member_roles (
    guild_id        UUID NOT NULL,
    user_id         UUID NOT NULL,
    role_id         UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    assigned_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (guild_id, user_id, role_id),
    FOREIGN KEY (guild_id, user_id) REFERENCES guild_members(guild_id, user_id) ON DELETE CASCADE
);

-- ============================================================================
-- CATEGORIES & CHANNELS
-- ============================================================================

CREATE TABLE categories (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    guild_id        UUID NOT NULL REFERENCES guilds(id) ON DELETE CASCADE,
    name            VARCHAR(100) NOT NULL,
    position        INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_categories_guild_id ON categories (guild_id);

CREATE TABLE channels (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    guild_id        UUID NOT NULL REFERENCES guilds(id) ON DELETE CASCADE,
    category_id     UUID REFERENCES categories(id) ON DELETE SET NULL,
    name            VARCHAR(100) NOT NULL,
    type            channel_type NOT NULL DEFAULT 'text',
    topic           VARCHAR(1024),
    position        INT NOT NULL DEFAULT 0,
    is_nsfw         BOOLEAN NOT NULL DEFAULT false,
    slowmode_secs   INT NOT NULL DEFAULT 0,
    bitrate         INT,
    user_limit      INT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_channels_guild_id ON channels (guild_id);
CREATE INDEX idx_channels_category_id ON channels (category_id);

CREATE TABLE channel_permission_overrides (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    channel_id      UUID NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    target_type     VARCHAR(10) NOT NULL CHECK (target_type IN ('role', 'member')),
    target_id       UUID NOT NULL,
    allow           BIGINT NOT NULL DEFAULT 0,
    deny            BIGINT NOT NULL DEFAULT 0,
    UNIQUE (channel_id, target_type, target_id)
);

CREATE INDEX idx_channel_perm_overrides_channel ON channel_permission_overrides (channel_id);

-- ============================================================================
-- MESSAGES
-- ============================================================================

CREATE TABLE messages (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    channel_id      UUID NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    author_id       UUID NOT NULL REFERENCES users(id),
    content         TEXT,
    type            message_type NOT NULL DEFAULT 'default',
    reply_to_id     UUID REFERENCES messages(id) ON DELETE SET NULL,
    is_pinned       BOOLEAN NOT NULL DEFAULT false,
    is_edited       BOOLEAN NOT NULL DEFAULT false,
    is_deleted      BOOLEAN NOT NULL DEFAULT false,
    edited_at       TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_messages_channel_id ON messages (channel_id, created_at DESC);
CREATE INDEX idx_messages_author_id ON messages (author_id);
CREATE INDEX idx_messages_reply_to ON messages (reply_to_id) WHERE reply_to_id IS NOT NULL;

CREATE TABLE message_attachments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id      UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    file_name       VARCHAR(255) NOT NULL,
    file_size       BIGINT NOT NULL,
    content_type    VARCHAR(128) NOT NULL,
    url             TEXT NOT NULL,
    width           INT,
    height          INT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_message_attachments_message ON message_attachments (message_id);

CREATE TABLE message_reactions (
    message_id      UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    emoji           VARCHAR(64) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (message_id, user_id, emoji)
);

CREATE TABLE message_mentions (
    message_id      UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (message_id, user_id)
);

CREATE TABLE read_states (
    channel_id      UUID NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    last_message_id UUID REFERENCES messages(id) ON DELETE SET NULL,
    mention_count   INT NOT NULL DEFAULT 0,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (channel_id, user_id)
);

CREATE TABLE pins (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    channel_id      UUID NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    message_id      UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    pinned_by       UUID NOT NULL REFERENCES users(id),
    pinned_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (channel_id, message_id)
);

CREATE INDEX idx_pins_channel ON pins (channel_id);

-- ============================================================================
-- THREADS & FORUMS
-- ============================================================================

CREATE TABLE threads (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    channel_id      UUID NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    creator_id      UUID NOT NULL REFERENCES users(id),
    name            VARCHAR(100) NOT NULL,
    is_locked       BOOLEAN NOT NULL DEFAULT false,
    is_archived     BOOLEAN NOT NULL DEFAULT false,
    auto_archive_minutes INT NOT NULL DEFAULT 1440,
    message_count   INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    archived_at     TIMESTAMPTZ,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_threads_channel_id ON threads (channel_id);

CREATE TABLE thread_members (
    thread_id       UUID NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    joined_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (thread_id, user_id)
);

CREATE TABLE forum_tags (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    channel_id      UUID NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    name            VARCHAR(32) NOT NULL,
    emoji           VARCHAR(64),
    is_moderated    BOOLEAN NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_forum_tags_channel ON forum_tags (channel_id);

CREATE TABLE forum_post_tags (
    thread_id       UUID NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
    tag_id          UUID NOT NULL REFERENCES forum_tags(id) ON DELETE CASCADE,
    PRIMARY KEY (thread_id, tag_id)
);

-- ============================================================================
-- DIRECT MESSAGES
-- ============================================================================

CREATE TABLE dm_channels (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type            dm_channel_type NOT NULL DEFAULT 'dm',
    name            VARCHAR(100),
    icon_url        TEXT,
    owner_id        UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE dm_channel_members (
    dm_channel_id   UUID NOT NULL REFERENCES dm_channels(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    joined_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_read_at    TIMESTAMPTZ,
    is_muted        BOOLEAN NOT NULL DEFAULT false,
    PRIMARY KEY (dm_channel_id, user_id)
);

CREATE INDEX idx_dm_channel_members_user ON dm_channel_members (user_id);

CREATE TABLE dm_messages (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dm_channel_id   UUID NOT NULL REFERENCES dm_channels(id) ON DELETE CASCADE,
    author_id       UUID NOT NULL REFERENCES users(id),
    content         TEXT,
    reply_to_id     UUID REFERENCES dm_messages(id) ON DELETE SET NULL,
    is_edited       BOOLEAN NOT NULL DEFAULT false,
    is_deleted      BOOLEAN NOT NULL DEFAULT false,
    edited_at       TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_dm_messages_channel ON dm_messages (dm_channel_id, created_at DESC);
CREATE INDEX idx_dm_messages_author ON dm_messages (author_id);

-- ============================================================================
-- END-TO-END ENCRYPTION
-- ============================================================================

CREATE TABLE e2ee_identity_keys (
    user_id             UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    identity_public_key TEXT NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE e2ee_signed_prekeys (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key_id              INT NOT NULL,
    public_key          TEXT NOT NULL,
    signature           TEXT NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, key_id)
);

CREATE INDEX idx_e2ee_signed_prekeys_user ON e2ee_signed_prekeys (user_id);

CREATE TABLE e2ee_one_time_prekeys (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    key_id              INT NOT NULL,
    public_key          TEXT NOT NULL,
    used                BOOLEAN NOT NULL DEFAULT false,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, key_id)
);

CREATE INDEX idx_e2ee_otp_user ON e2ee_one_time_prekeys (user_id, used);

-- ============================================================================
-- INVITES
-- ============================================================================

CREATE TABLE invites (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code            VARCHAR(16) NOT NULL UNIQUE,
    type            invite_type NOT NULL DEFAULT 'guild',
    guild_id        UUID REFERENCES guilds(id) ON DELETE CASCADE,
    channel_id      UUID REFERENCES channels(id) ON DELETE CASCADE,
    inviter_id      UUID NOT NULL REFERENCES users(id),
    max_uses        INT,
    use_count       INT NOT NULL DEFAULT 0,
    max_age_secs    INT,
    is_temporary    BOOLEAN NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ
);

CREATE INDEX idx_invites_code ON invites (code);
CREATE INDEX idx_invites_guild ON invites (guild_id);

-- ============================================================================
-- CUSTOM EMOJI & STICKERS
-- ============================================================================

CREATE TABLE custom_emoji (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    guild_id        UUID NOT NULL REFERENCES guilds(id) ON DELETE CASCADE,
    name            VARCHAR(32) NOT NULL,
    image_url       TEXT NOT NULL,
    creator_id      UUID NOT NULL REFERENCES users(id),
    is_animated     BOOLEAN NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_custom_emoji_guild ON custom_emoji (guild_id);

CREATE TABLE sticker_packs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(64) NOT NULL,
    description     VARCHAR(256),
    cover_url       TEXT,
    guild_id        UUID REFERENCES guilds(id) ON DELETE CASCADE,
    creator_id      UUID NOT NULL REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sticker_packs_guild ON sticker_packs (guild_id);

CREATE TABLE stickers (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pack_id         UUID NOT NULL REFERENCES sticker_packs(id) ON DELETE CASCADE,
    name            VARCHAR(32) NOT NULL,
    description     VARCHAR(128),
    image_url       TEXT NOT NULL,
    tags            TEXT[],
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_stickers_pack ON stickers (pack_id);

-- ============================================================================
-- POLLS
-- ============================================================================

CREATE TABLE polls (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    message_id      UUID NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    question        VARCHAR(300) NOT NULL,
    allows_multiple BOOLEAN NOT NULL DEFAULT false,
    expires_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_polls_message ON polls (message_id);

CREATE TABLE poll_options (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    poll_id         UUID NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
    text            VARCHAR(100) NOT NULL,
    position        INT NOT NULL DEFAULT 0,
    vote_count      INT NOT NULL DEFAULT 0
);

CREATE INDEX idx_poll_options_poll ON poll_options (poll_id);

CREATE TABLE poll_votes (
    poll_id         UUID NOT NULL REFERENCES polls(id) ON DELETE CASCADE,
    option_id       UUID NOT NULL REFERENCES poll_options(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (poll_id, option_id, user_id)
);

-- ============================================================================
-- MODERATION
-- ============================================================================

CREATE TABLE audit_logs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    guild_id        UUID NOT NULL REFERENCES guilds(id) ON DELETE CASCADE,
    actor_id        UUID NOT NULL REFERENCES users(id),
    action          audit_action NOT NULL,
    target_type     VARCHAR(32),
    target_id       UUID,
    changes         JSONB,
    reason          VARCHAR(512),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_logs_guild ON audit_logs (guild_id, created_at DESC);
CREATE INDEX idx_audit_logs_actor ON audit_logs (actor_id);

CREATE TABLE bans (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    guild_id        UUID NOT NULL REFERENCES guilds(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES users(id),
    banned_by       UUID NOT NULL REFERENCES users(id),
    reason          VARCHAR(512),
    expires_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (guild_id, user_id)
);

CREATE INDEX idx_bans_guild ON bans (guild_id);
CREATE INDEX idx_bans_user ON bans (user_id);

CREATE TABLE reports (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    guild_id        UUID REFERENCES guilds(id) ON DELETE SET NULL,
    reporter_id     UUID NOT NULL REFERENCES users(id),
    target_type     VARCHAR(32) NOT NULL,
    target_id       UUID NOT NULL,
    reason          VARCHAR(1024) NOT NULL,
    status          report_status NOT NULL DEFAULT 'open',
    moderator_id    UUID REFERENCES users(id),
    resolution_note VARCHAR(1024),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at     TIMESTAMPTZ
);

CREATE INDEX idx_reports_guild ON reports (guild_id);
CREATE INDEX idx_reports_status ON reports (status);

CREATE TABLE word_filters (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    guild_id        UUID NOT NULL REFERENCES guilds(id) ON DELETE CASCADE,
    pattern         VARCHAR(256) NOT NULL,
    is_regex        BOOLEAN NOT NULL DEFAULT false,
    action          VARCHAR(16) NOT NULL DEFAULT 'block' CHECK (action IN ('block', 'warn', 'mute')),
    created_by      UUID NOT NULL REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_word_filters_guild ON word_filters (guild_id);

-- ============================================================================
-- FILE UPLOADS
-- ============================================================================

CREATE TABLE file_uploads (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    uploader_id     UUID NOT NULL REFERENCES users(id),
    file_name       VARCHAR(255) NOT NULL,
    file_size       BIGINT NOT NULL,
    content_type    VARCHAR(128) NOT NULL,
    storage_key     TEXT NOT NULL UNIQUE,
    bucket          VARCHAR(64) NOT NULL,
    url             TEXT NOT NULL,
    status          upload_status NOT NULL DEFAULT 'pending',
    checksum_sha256 VARCHAR(64),
    width           INT,
    height          INT,
    duration_secs   FLOAT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_file_uploads_uploader ON file_uploads (uploader_id);
CREATE INDEX idx_file_uploads_status ON file_uploads (status);

-- ============================================================================
-- SYSTEM SETTINGS
-- ============================================================================

CREATE TABLE system_settings (
    key             VARCHAR(128) PRIMARY KEY,
    value           JSONB NOT NULL,
    updated_by      UUID REFERENCES users(id),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
