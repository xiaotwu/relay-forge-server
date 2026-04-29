package acl

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type fakeStore struct {
	uploads        map[uuid.UUID]Upload
	activeUsers    map[uuid.UUID]bool
	dmMembers      map[[2]uuid.UUID]bool
	channelMembers map[[2]uuid.UUID]bool
	guildMembers   map[[2]uuid.UUID]bool
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		uploads:        map[uuid.UUID]Upload{},
		activeUsers:    map[uuid.UUID]bool{},
		dmMembers:      map[[2]uuid.UUID]bool{},
		channelMembers: map[[2]uuid.UUID]bool{},
		guildMembers:   map[[2]uuid.UUID]bool{},
	}
}

func pair(a, b uuid.UUID) [2]uuid.UUID { return [2]uuid.UUID{a, b} }

func (s *fakeStore) GetUpload(_ context.Context, fileID uuid.UUID) (*Upload, error) {
	upload, ok := s.uploads[fileID]
	if !ok {
		return nil, ErrNotFound
	}
	return &upload, nil
}

func (s *fakeStore) IsUserActive(_ context.Context, userID uuid.UUID) (bool, error) {
	return s.activeUsers[userID], nil
}

func (s *fakeStore) IsDMParticipant(_ context.Context, dmChannelID, userID uuid.UUID) (bool, error) {
	return s.dmMembers[pair(dmChannelID, userID)], nil
}

func (s *fakeStore) IsChannelMember(_ context.Context, channelID, userID uuid.UUID) (bool, error) {
	return s.channelMembers[pair(channelID, userID)], nil
}

func (s *fakeStore) IsGuildMember(_ context.Context, guildID, userID uuid.UUID) (bool, error) {
	return s.guildMembers[pair(guildID, userID)], nil
}

func TestUploaderCanCompleteOwnPendingUpload(t *testing.T) {
	ctx := context.Background()
	uploaderID := uuid.New()
	fileID := uuid.New()
	dmID := uuid.New()
	store := newFakeStore()
	store.activeUsers[uploaderID] = true
	store.dmMembers[pair(dmID, uploaderID)] = true
	store.uploads[fileID] = Upload{ID: fileID, UploaderID: uploaderID, Status: StatusPending}

	if _, err := New(store).AuthorizeComplete(ctx, uploaderID, fileID, OwnerDMChannel, dmID); err != nil {
		t.Fatalf("AuthorizeComplete returned error: %v", err)
	}
}

func TestAnotherUserCannotCompleteSomeoneElsesPendingUpload(t *testing.T) {
	ctx := context.Background()
	uploaderID := uuid.New()
	otherID := uuid.New()
	fileID := uuid.New()
	dmID := uuid.New()
	store := newFakeStore()
	store.activeUsers[otherID] = true
	store.dmMembers[pair(dmID, otherID)] = true
	store.uploads[fileID] = Upload{ID: fileID, UploaderID: uploaderID, Status: StatusPending}

	_, err := New(store).AuthorizeComplete(ctx, otherID, fileID, OwnerDMChannel, dmID)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("AuthorizeComplete error = %v, want ErrForbidden", err)
	}
}

func TestDMParticipantCanAccessDMMedia(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	fileID := uuid.New()
	dmID := uuid.New()
	store := newFakeStore()
	store.activeUsers[userID] = true
	store.dmMembers[pair(dmID, userID)] = true
	store.uploads[fileID] = Upload{
		ID:        fileID,
		Status:    StatusClean,
		OwnerType: OwnerDMChannel,
		OwnerID:   &dmID,
	}

	if _, err := New(store).AuthorizeAccess(ctx, userID, fileID); err != nil {
		t.Fatalf("AuthorizeAccess returned error: %v", err)
	}
}

func TestNonParticipantCannotAccessDMMedia(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	fileID := uuid.New()
	dmID := uuid.New()
	store := newFakeStore()
	store.activeUsers[userID] = true
	store.uploads[fileID] = Upload{
		ID:        fileID,
		Status:    StatusClean,
		OwnerType: OwnerDMChannel,
		OwnerID:   &dmID,
	}

	_, err := New(store).AuthorizeAccess(ctx, userID, fileID)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("AuthorizeAccess error = %v, want ErrForbidden", err)
	}
}

func TestChannelMemberCanAccessChannelMedia(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	fileID := uuid.New()
	channelID := uuid.New()
	store := newFakeStore()
	store.activeUsers[userID] = true
	store.channelMembers[pair(channelID, userID)] = true
	store.uploads[fileID] = Upload{
		ID:        fileID,
		Status:    StatusClean,
		OwnerType: OwnerChannel,
		OwnerID:   &channelID,
	}

	if _, err := New(store).AuthorizeAccess(ctx, userID, fileID); err != nil {
		t.Fatalf("AuthorizeAccess returned error: %v", err)
	}
}

func TestUnrelatedUserCannotAccessChannelMedia(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	fileID := uuid.New()
	channelID := uuid.New()
	store := newFakeStore()
	store.activeUsers[userID] = true
	store.uploads[fileID] = Upload{
		ID:        fileID,
		Status:    StatusClean,
		OwnerType: OwnerChannel,
		OwnerID:   &channelID,
	}

	_, err := New(store).AuthorizeAccess(ctx, userID, fileID)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("AuthorizeAccess error = %v, want ErrForbidden", err)
	}
}

func TestUploaderCanAccessOwnPendingUpload(t *testing.T) {
	ctx := context.Background()
	uploaderID := uuid.New()
	fileID := uuid.New()
	store := newFakeStore()
	store.uploads[fileID] = Upload{
		ID:         fileID,
		UploaderID: uploaderID,
		Status:     StatusPending,
		OwnerType:  OwnerPending,
	}

	if _, err := New(store).AuthorizeAccess(ctx, uploaderID, fileID); err != nil {
		t.Fatalf("AuthorizeAccess returned error: %v", err)
	}
}

func TestDisabledUserCannotAccessGuildMedia(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	fileID := uuid.New()
	guildID := uuid.New()
	store := newFakeStore()
	store.guildMembers[pair(guildID, userID)] = true
	store.uploads[fileID] = Upload{
		ID:        fileID,
		Status:    StatusClean,
		OwnerType: OwnerGuild,
		OwnerID:   &guildID,
	}

	_, err := New(store).AuthorizeAccess(ctx, userID, fileID)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("AuthorizeAccess error = %v, want ErrForbidden", err)
	}
}

func TestUserProfileMediaIsPublic(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.New()
	fileID := uuid.New()
	store := newFakeStore()
	store.uploads[fileID] = Upload{
		ID:        fileID,
		Status:    StatusClean,
		OwnerType: OwnerUserProfile,
		OwnerID:   &ownerID,
	}

	if _, err := New(store).AuthorizeAccess(ctx, uuid.Nil, fileID); err != nil {
		t.Fatalf("AuthorizeAccess returned error: %v", err)
	}
}
