package acl

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

const (
	OwnerPending     = "pending"
	OwnerDMChannel   = "dm_channel"
	OwnerChannel     = "channel"
	OwnerGuild       = "guild"
	OwnerUserProfile = "user_profile"

	StatusPending = "pending"
	StatusClean   = "clean"
)

var (
	ErrNotFound     = errors.New("media not found")
	ErrUnauthorized = errors.New("authentication required")
	ErrForbidden    = errors.New("media access denied")
	ErrInvalidOwner = errors.New("invalid media owner")
)

type Upload struct {
	ID          uuid.UUID
	UploaderID  uuid.UUID
	FileName    string
	FileSize    int64
	ContentType string
	StorageKey  string
	Bucket      string
	URL         string
	Status      string
	OwnerType   string
	OwnerID     *uuid.UUID
}

type Store interface {
	GetUpload(ctx context.Context, fileID uuid.UUID) (*Upload, error)
	IsUserActive(ctx context.Context, userID uuid.UUID) (bool, error)
	IsDMParticipant(ctx context.Context, dmChannelID, userID uuid.UUID) (bool, error)
	IsChannelMember(ctx context.Context, channelID, userID uuid.UUID) (bool, error)
	IsGuildMember(ctx context.Context, guildID, userID uuid.UUID) (bool, error)
}

type Service struct {
	store Store
}

func New(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) AuthorizeComplete(
	ctx context.Context,
	userID uuid.UUID,
	fileID uuid.UUID,
	ownerType string,
	ownerID uuid.UUID,
) (*Upload, error) {
	if userID == uuid.Nil {
		return nil, ErrUnauthorized
	}

	upload, err := s.store.GetUpload(ctx, fileID)
	if err != nil {
		return nil, err
	}
	if upload.UploaderID != userID || upload.Status != StatusPending {
		return nil, ErrForbidden
	}
	if ownerID == uuid.Nil {
		return nil, ErrInvalidOwner
	}
	if err := s.authorizeOwner(ctx, userID, ownerType, ownerID); err != nil {
		return nil, err
	}

	return upload, nil
}

func (s *Service) AuthorizeAccess(ctx context.Context, userID uuid.UUID, fileID uuid.UUID) (*Upload, error) {
	upload, err := s.store.GetUpload(ctx, fileID)
	if err != nil {
		return nil, err
	}

	if upload.OwnerType == OwnerUserProfile {
		return upload, nil
	}
	if userID == uuid.Nil {
		return nil, ErrUnauthorized
	}
	if upload.Status == StatusPending {
		if upload.UploaderID == userID {
			return upload, nil
		}
		return nil, ErrForbidden
	}
	if upload.OwnerID == nil {
		return nil, ErrForbidden
	}
	if err := s.authorizeOwner(ctx, userID, upload.OwnerType, *upload.OwnerID); err != nil {
		return nil, err
	}

	return upload, nil
}

func (s *Service) authorizeOwner(ctx context.Context, userID uuid.UUID, ownerType string, ownerID uuid.UUID) error {
	active, err := s.store.IsUserActive(ctx, userID)
	if err != nil {
		return err
	}
	if !active {
		return ErrForbidden
	}

	var allowed bool
	switch ownerType {
	case OwnerDMChannel:
		allowed, err = s.store.IsDMParticipant(ctx, ownerID, userID)
	case OwnerChannel:
		allowed, err = s.store.IsChannelMember(ctx, ownerID, userID)
	case OwnerGuild:
		allowed, err = s.store.IsGuildMember(ctx, ownerID, userID)
	case OwnerUserProfile:
		allowed = ownerID == userID
	default:
		return ErrInvalidOwner
	}
	if err != nil {
		return err
	}
	if !allowed {
		return ErrForbidden
	}

	return nil
}
