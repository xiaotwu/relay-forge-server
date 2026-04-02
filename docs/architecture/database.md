# Database Model Overview

## Core Tables

### Users and Authentication

- `users` -- user accounts (id, username, display_name, email, avatar_url, status, created_at, updated_at, deleted_at)
- `user_passwords` -- hashed passwords (user_id, hash, algorithm, updated_at)
- `sessions` -- active sessions (id, user_id, device_id, token_hash, ip, user_agent, expires_at, created_at)
- `refresh_tokens` -- refresh token rotation (id, user_id, session_id, token_hash, family, counter, expires_at, revoked_at)
- `devices` -- registered devices for E2EE (id, user_id, name, platform, push_token, last_seen_at, created_at)
- `totp_secrets` -- 2FA TOTP secrets (user_id, secret_encrypted, enabled, verified_at)
- `password_resets` -- password reset tokens (id, user_id, token_hash, expires_at, used_at)
- `email_verifications` -- email verification tokens (id, user_id, email, token_hash, expires_at, verified_at)

### Guilds and Channels

- `guilds` -- guild/server (id, name, description, icon_url, banner_url, owner_id, visibility, created_at, updated_at, deleted_at)
- `guild_members` -- membership (guild_id, user_id, nickname, joined_at, muted_until)
- `categories` -- channel groups (id, guild_id, name, position)
- `channels` -- channels (id, guild_id, category_id, name, topic, type, position, slowmode_seconds, nsfw, archived, created_at, updated_at, deleted_at)
  - type enum: text, voice, forum, announcement
- `channel_permission_overrides` -- per-channel overrides (channel_id, target_type, target_id, allow, deny)

### Roles and Permissions

- `roles` -- guild roles (id, guild_id, name, color, position, permissions_bitfield, mentionable, hoisted, created_at)
- `guild_member_roles` -- role assignments (guild_id, user_id, role_id)

### Messages

- `messages` -- messages (id, channel_id, author_id, content, type, reply_to_id, edited_at, deleted_at, pinned, created_at)
  - type enum: default, system, reply, thread_starter
- `message_attachments` -- file attachments (id, message_id, file_name, file_size, mime_type, storage_key, width, height, created_at)
- `message_reactions` -- reactions (message_id, user_id, emoji_id, emoji_native, created_at)
- `message_mentions` -- indexed mentions (message_id, user_id, role_id, mention_everyone)
- `read_states` -- per-user per-channel read position (user_id, channel_id, last_read_message_id, mention_count, updated_at)
- `pins` -- pinned messages (channel_id, message_id, pinned_by, pinned_at)

### Threads and Forums

- `threads` -- threads (id, channel_id, starter_message_id, name, archived, locked, auto_archive_minutes, created_at, archived_at)
- `thread_members` -- thread subscriptions (thread_id, user_id, joined_at, last_read_message_id)
- `forum_tags` -- forum channel tags (id, channel_id, name, emoji_id, moderated)
- `forum_post_tags` -- tags applied to forum posts (thread_id, tag_id)

### Direct Messages

- `dm_channels` -- DM conversations (id, type, created_at)
  - type enum: dm, group_dm
- `dm_channel_members` -- participants (dm_channel_id, user_id, joined_at)
- `dm_messages` -- encrypted DM messages (id, dm_channel_id, sender_id, sender_device_id, recipient_device_id, ciphertext, iv, header, created_at, deleted_at)

### E2EE Key Management

- `e2ee_identity_keys` -- device identity keys (device_id, identity_public_key, created_at)
- `e2ee_signed_prekeys` -- signed prekeys (id, device_id, public_key, signature, created_at, deprecated_at)
- `e2ee_one_time_prekeys` -- one-time prekeys (id, device_id, public_key, used)

### Invites

- `invites` -- guild invites (code, guild_id, channel_id, inviter_id, max_uses, use_count, expires_at, created_at)

### Emoji and Stickers

- `custom_emoji` -- guild or global emoji (id, guild_id, name, image_url, animated, creator_id, created_at, disabled)
- `sticker_packs` -- sticker collections (id, guild_id, name, description, created_at)
- `stickers` -- individual stickers (id, pack_id, name, image_url, format, created_at)

### Polls

- `polls` -- message polls (id, message_id, question, allow_multiple, expires_at, created_at)
- `poll_options` -- poll choices (id, poll_id, text, position)
- `poll_votes` -- votes (poll_id, option_id, user_id, voted_at)

### Moderation

- `audit_logs` -- audit trail (id, guild_id, actor_id, action, target_type, target_id, metadata, created_at)
- `bans` -- guild bans (guild_id, user_id, reason, banned_by, created_at)
- `reports` -- abuse reports (id, reporter_id, guild_id, target_type, target_id, reason, status, resolved_by, created_at, resolved_at)
- `word_filters` -- sensitive word patterns (id, guild_id, pattern, action, created_at)

### Files and Storage

- `file_uploads` -- upload tracking (id, uploader_id, file_name, file_size, mime_type, storage_key, bucket, scan_status, created_at)
  - scan_status enum: pending, clean, infected, error, skipped

### System

- `system_settings` -- key-value system configuration (key, value, updated_at, updated_by)

## Indexing Strategy

- Primary keys: UUIDv7 (time-ordered) for most entities, sequential for bridge tables
- Foreign key indexes on all references
- Composite indexes on (guild_id, position) for ordered listings
- Composite indexes on (channel_id, created_at) for message pagination
- GIN index on messages.content for full-text search
- Partial indexes on deleted_at IS NULL for soft-deleted tables
- Index on (user_id, channel_id) for read states
- Index on invites.code for fast lookup

## Pagination Strategy

- Cursor-based pagination using message IDs (UUIDv7 are time-ordered)
- Keyset pagination for channel lists, member lists
- Offset pagination only for admin dashboard pages with known-small datasets
