package models

import (
	"time"

	"github.com/google/uuid"
)

// ============================================================================
// Users & Authentication
// ============================================================================

type User struct {
	ID           uuid.UUID  `json:"id"`
	Username     string     `json:"username"`
	DisplayName  *string    `json:"display_name,omitempty"`
	Email        string     `json:"email"`
	AvatarURL    *string    `json:"avatar_url,omitempty"`
	BannerURL    *string    `json:"banner_url,omitempty"`
	Bio          *string    `json:"bio,omitempty"`
	Status       string     `json:"status"`
	CustomStatus *string    `json:"custom_status,omitempty"`
	IsBot        bool       `json:"is_bot"`
	IsVerified   bool       `json:"is_verified"`
	IsDisabled   bool       `json:"is_disabled"`
	Locale       string     `json:"locale"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type UserPassword struct {
	UserID       uuid.UUID `json:"user_id"`
	PasswordHash string    `json:"-"`
	ChangedAt    time.Time `json:"changed_at"`
}

type Session struct {
	ID           uuid.UUID  `json:"id"`
	UserID       uuid.UUID  `json:"user_id"`
	TokenHash    string     `json:"-"`
	IPAddress    *string    `json:"ip_address,omitempty"`
	UserAgent    *string    `json:"user_agent,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	ExpiresAt    time.Time  `json:"expires_at"`
	LastActiveAt time.Time  `json:"last_active_at"`
}

type RefreshToken struct {
	ID         uuid.UUID  `json:"id"`
	UserID     uuid.UUID  `json:"user_id"`
	TokenHash  string     `json:"-"`
	DeviceID   *uuid.UUID `json:"device_id,omitempty"`
	IPAddress  *string    `json:"ip_address,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}

type Device struct {
	ID         uuid.UUID `json:"id"`
	UserID     uuid.UUID `json:"user_id"`
	DeviceName string    `json:"device_name"`
	DeviceType string    `json:"device_type"`
	PushToken  *string   `json:"push_token,omitempty"`
	LastIP     *string   `json:"last_ip,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
}

type TOTPSecret struct {
	UserID      uuid.UUID  `json:"user_id"`
	Secret      string     `json:"-"`
	Verified    bool       `json:"verified"`
	BackupCodes []string   `json:"-"`
	EnabledAt   *time.Time `json:"enabled_at,omitempty"`
}

type PasswordReset struct {
	ID        uuid.UUID  `json:"id"`
	UserID    uuid.UUID  `json:"user_id"`
	TokenHash string     `json:"-"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt time.Time  `json:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
}

type EmailVerification struct {
	ID         uuid.UUID  `json:"id"`
	UserID     uuid.UUID  `json:"user_id"`
	Email      string     `json:"email"`
	TokenHash  string     `json:"-"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
	VerifiedAt *time.Time `json:"verified_at,omitempty"`
}

// ============================================================================
// Guilds & Membership
// ============================================================================

type Guild struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	Description *string    `json:"description,omitempty"`
	IconURL     *string    `json:"icon_url,omitempty"`
	BannerURL   *string    `json:"banner_url,omitempty"`
	OwnerID     uuid.UUID  `json:"owner_id"`
	IsPublic    bool       `json:"is_public"`
	MemberCount int        `json:"member_count"`
	MaxMembers  int        `json:"max_members"`
	VanityURL   *string    `json:"vanity_url,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type GuildMember struct {
	GuildID    uuid.UUID `json:"guild_id"`
	UserID     uuid.UUID `json:"user_id"`
	Nickname   *string   `json:"nickname,omitempty"`
	JoinedAt   time.Time `json:"joined_at"`
	IsMuted    bool      `json:"is_muted"`
	IsDeafened bool      `json:"is_deafened"`
}

// ============================================================================
// Roles & Permissions
// ============================================================================

type Role struct {
	ID            uuid.UUID `json:"id"`
	GuildID       uuid.UUID `json:"guild_id"`
	Name          string    `json:"name"`
	Color         string    `json:"color"`
	Position      int       `json:"position"`
	Permissions   int64     `json:"permissions"`
	IsDefault     bool      `json:"is_default"`
	IsMentionable bool      `json:"is_mentionable"`
	IconURL       *string   `json:"icon_url,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type GuildMemberRole struct {
	GuildID    uuid.UUID `json:"guild_id"`
	UserID     uuid.UUID `json:"user_id"`
	RoleID     uuid.UUID `json:"role_id"`
	AssignedAt time.Time `json:"assigned_at"`
}

// ============================================================================
// Categories & Channels
// ============================================================================

type Category struct {
	ID        uuid.UUID `json:"id"`
	GuildID   uuid.UUID `json:"guild_id"`
	Name      string    `json:"name"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Channel struct {
	ID           uuid.UUID  `json:"id"`
	GuildID      uuid.UUID  `json:"guild_id"`
	CategoryID   *uuid.UUID `json:"category_id,omitempty"`
	Name         string     `json:"name"`
	Type         string     `json:"type"`
	Topic        *string    `json:"topic,omitempty"`
	Position     int        `json:"position"`
	IsNSFW       bool       `json:"is_nsfw"`
	SlowmodeSecs int        `json:"slowmode_secs"`
	Bitrate      *int       `json:"bitrate,omitempty"`
	UserLimit    *int       `json:"user_limit,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type ChannelPermissionOverride struct {
	ID         uuid.UUID `json:"id"`
	ChannelID  uuid.UUID `json:"channel_id"`
	TargetType string    `json:"target_type"`
	TargetID   uuid.UUID `json:"target_id"`
	Allow      int64     `json:"allow"`
	Deny       int64     `json:"deny"`
}

// ============================================================================
// Messages
// ============================================================================

type Message struct {
	ID        uuid.UUID  `json:"id"`
	ChannelID uuid.UUID  `json:"channel_id"`
	AuthorID  uuid.UUID  `json:"author_id"`
	Content   *string    `json:"content,omitempty"`
	Type      string     `json:"type"`
	ReplyToID *uuid.UUID `json:"reply_to_id,omitempty"`
	IsPinned  bool       `json:"is_pinned"`
	IsEdited  bool       `json:"is_edited"`
	IsDeleted bool       `json:"is_deleted"`
	EditedAt  *time.Time `json:"edited_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

type MessageAttachment struct {
	ID          uuid.UUID `json:"id"`
	MessageID   uuid.UUID `json:"message_id"`
	FileName    string    `json:"file_name"`
	FileSize    int64     `json:"file_size"`
	ContentType string    `json:"content_type"`
	URL         string    `json:"url"`
	Width       *int      `json:"width,omitempty"`
	Height      *int      `json:"height,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type MessageReaction struct {
	MessageID uuid.UUID `json:"message_id"`
	UserID    uuid.UUID `json:"user_id"`
	Emoji     string    `json:"emoji"`
	CreatedAt time.Time `json:"created_at"`
}

type MessageMention struct {
	MessageID uuid.UUID `json:"message_id"`
	UserID    uuid.UUID `json:"user_id"`
}

type ReadState struct {
	ChannelID    uuid.UUID  `json:"channel_id"`
	UserID       uuid.UUID  `json:"user_id"`
	LastMessageID *uuid.UUID `json:"last_message_id,omitempty"`
	MentionCount int        `json:"mention_count"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type Pin struct {
	ID        uuid.UUID `json:"id"`
	ChannelID uuid.UUID `json:"channel_id"`
	MessageID uuid.UUID `json:"message_id"`
	PinnedBy  uuid.UUID `json:"pinned_by"`
	PinnedAt  time.Time `json:"pinned_at"`
}

// ============================================================================
// Threads & Forums
// ============================================================================

type Thread struct {
	ID                 uuid.UUID  `json:"id"`
	ChannelID          uuid.UUID  `json:"channel_id"`
	CreatorID          uuid.UUID  `json:"creator_id"`
	Name               string     `json:"name"`
	IsLocked           bool       `json:"is_locked"`
	IsArchived         bool       `json:"is_archived"`
	AutoArchiveMinutes int        `json:"auto_archive_minutes"`
	MessageCount       int        `json:"message_count"`
	CreatedAt          time.Time  `json:"created_at"`
	ArchivedAt         *time.Time `json:"archived_at,omitempty"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type ThreadMember struct {
	ThreadID uuid.UUID `json:"thread_id"`
	UserID   uuid.UUID `json:"user_id"`
	JoinedAt time.Time `json:"joined_at"`
}

type ForumTag struct {
	ID          uuid.UUID `json:"id"`
	ChannelID   uuid.UUID `json:"channel_id"`
	Name        string    `json:"name"`
	Emoji       *string   `json:"emoji,omitempty"`
	IsModerated bool      `json:"is_moderated"`
	CreatedAt   time.Time `json:"created_at"`
}

type ForumPostTag struct {
	ThreadID uuid.UUID `json:"thread_id"`
	TagID    uuid.UUID `json:"tag_id"`
}

// ============================================================================
// Direct Messages
// ============================================================================

type DMChannel struct {
	ID        uuid.UUID  `json:"id"`
	Type      string     `json:"type"`
	Name      *string    `json:"name,omitempty"`
	IconURL   *string    `json:"icon_url,omitempty"`
	OwnerID   *uuid.UUID `json:"owner_id,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type DMChannelMember struct {
	DMChannelID uuid.UUID  `json:"dm_channel_id"`
	UserID      uuid.UUID  `json:"user_id"`
	JoinedAt    time.Time  `json:"joined_at"`
	LastReadAt  *time.Time `json:"last_read_at,omitempty"`
	IsMuted     bool       `json:"is_muted"`
}

type DMMessage struct {
	ID          uuid.UUID  `json:"id"`
	DMChannelID uuid.UUID  `json:"dm_channel_id"`
	AuthorID    uuid.UUID  `json:"author_id"`
	Content     *string    `json:"content,omitempty"`
	ReplyToID   *uuid.UUID `json:"reply_to_id,omitempty"`
	IsEdited    bool       `json:"is_edited"`
	IsDeleted   bool       `json:"is_deleted"`
	EditedAt    *time.Time `json:"edited_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// ============================================================================
// End-to-End Encryption
// ============================================================================

type E2EEIdentityKey struct {
	UserID            uuid.UUID `json:"user_id"`
	IdentityPublicKey string    `json:"identity_public_key"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type E2EESignedPrekey struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	KeyID     int       `json:"key_id"`
	PublicKey string    `json:"public_key"`
	Signature string    `json:"signature"`
	CreatedAt time.Time `json:"created_at"`
}

type E2EEOneTimePrekey struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	KeyID     int       `json:"key_id"`
	PublicKey string    `json:"public_key"`
	Used      bool      `json:"used"`
	CreatedAt time.Time `json:"created_at"`
}

// ============================================================================
// Invites
// ============================================================================

type Invite struct {
	ID          uuid.UUID  `json:"id"`
	Code        string     `json:"code"`
	Type        string     `json:"type"`
	GuildID     *uuid.UUID `json:"guild_id,omitempty"`
	ChannelID   *uuid.UUID `json:"channel_id,omitempty"`
	InviterID   uuid.UUID  `json:"inviter_id"`
	MaxUses     *int       `json:"max_uses,omitempty"`
	UseCount    int        `json:"use_count"`
	MaxAgeSecs  *int       `json:"max_age_secs,omitempty"`
	IsTemporary bool       `json:"is_temporary"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

// ============================================================================
// Custom Emoji & Stickers
// ============================================================================

type CustomEmoji struct {
	ID         uuid.UUID `json:"id"`
	GuildID    uuid.UUID `json:"guild_id"`
	Name       string    `json:"name"`
	ImageURL   string    `json:"image_url"`
	CreatorID  uuid.UUID `json:"creator_id"`
	IsAnimated bool      `json:"is_animated"`
	CreatedAt  time.Time `json:"created_at"`
}

type StickerPack struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	Description *string    `json:"description,omitempty"`
	CoverURL    *string    `json:"cover_url,omitempty"`
	GuildID     *uuid.UUID `json:"guild_id,omitempty"`
	CreatorID   uuid.UUID  `json:"creator_id"`
	CreatedAt   time.Time  `json:"created_at"`
}

type Sticker struct {
	ID          uuid.UUID `json:"id"`
	PackID      uuid.UUID `json:"pack_id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	ImageURL    string    `json:"image_url"`
	Tags        []string  `json:"tags,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// ============================================================================
// Polls
// ============================================================================

type Poll struct {
	ID             uuid.UUID  `json:"id"`
	MessageID      uuid.UUID  `json:"message_id"`
	Question       string     `json:"question"`
	AllowsMultiple bool       `json:"allows_multiple"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type PollOption struct {
	ID        uuid.UUID `json:"id"`
	PollID    uuid.UUID `json:"poll_id"`
	Text      string    `json:"text"`
	Position  int       `json:"position"`
	VoteCount int       `json:"vote_count"`
}

type PollVote struct {
	PollID    uuid.UUID `json:"poll_id"`
	OptionID  uuid.UUID `json:"option_id"`
	UserID    uuid.UUID `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

// ============================================================================
// Moderation
// ============================================================================

type AuditLog struct {
	ID         uuid.UUID  `json:"id"`
	GuildID    uuid.UUID  `json:"guild_id"`
	ActorID    uuid.UUID  `json:"actor_id"`
	Action     string     `json:"action"`
	TargetType *string    `json:"target_type,omitempty"`
	TargetID   *uuid.UUID `json:"target_id,omitempty"`
	Changes    any        `json:"changes,omitempty"`
	Reason     *string    `json:"reason,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type Ban struct {
	ID        uuid.UUID  `json:"id"`
	GuildID   uuid.UUID  `json:"guild_id"`
	UserID    uuid.UUID  `json:"user_id"`
	BannedBy  uuid.UUID  `json:"banned_by"`
	Reason    *string    `json:"reason,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

type Report struct {
	ID             uuid.UUID  `json:"id"`
	GuildID        *uuid.UUID `json:"guild_id,omitempty"`
	ReporterID     uuid.UUID  `json:"reporter_id"`
	TargetType     string     `json:"target_type"`
	TargetID       uuid.UUID  `json:"target_id"`
	Reason         string     `json:"reason"`
	Status         string     `json:"status"`
	ModeratorID    *uuid.UUID `json:"moderator_id,omitempty"`
	ResolutionNote *string    `json:"resolution_note,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
}

type WordFilter struct {
	ID        uuid.UUID `json:"id"`
	GuildID   uuid.UUID `json:"guild_id"`
	Pattern   string    `json:"pattern"`
	IsRegex   bool      `json:"is_regex"`
	Action    string    `json:"action"`
	CreatedBy uuid.UUID `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

// ============================================================================
// File Uploads
// ============================================================================

type FileUpload struct {
	ID             uuid.UUID  `json:"id"`
	UploaderID     uuid.UUID  `json:"uploader_id"`
	FileName       string     `json:"file_name"`
	FileSize       int64      `json:"file_size"`
	ContentType    string     `json:"content_type"`
	StorageKey     string     `json:"storage_key"`
	Bucket         string     `json:"bucket"`
	URL            string     `json:"url"`
	Status         string     `json:"status"`
	ChecksumSHA256 *string    `json:"checksum_sha256,omitempty"`
	Width          *int       `json:"width,omitempty"`
	Height         *int       `json:"height,omitempty"`
	DurationSecs   *float64   `json:"duration_secs,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// ============================================================================
// System Settings
// ============================================================================

type SystemSetting struct {
	Key       string     `json:"key"`
	Value     any        `json:"value"`
	UpdatedBy *uuid.UUID `json:"updated_by,omitempty"`
	UpdatedAt time.Time  `json:"updated_at"`
}
