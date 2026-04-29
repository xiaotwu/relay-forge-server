package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"

	"github.com/relay-forge/relay-forge/services/media/internal/acl"
	"github.com/relay-forge/relay-forge/services/media/internal/config"
)

type fakeObjectStorage struct {
	statInfo minio.ObjectInfo
	statErr  error
	getURL   string
	putURL   string
}

func (s *fakeObjectStorage) PresignedPutURL(context.Context, string, string, time.Duration) (string, error) {
	if s.putURL == "" {
		return "https://storage.example/upload", nil
	}
	return s.putURL, nil
}

func (s *fakeObjectStorage) PresignedGetURL(context.Context, string, string, time.Duration) (string, error) {
	if s.getURL == "" {
		return "https://storage.example/download", nil
	}
	return s.getURL, nil
}

func (s *fakeObjectStorage) Stat(context.Context, string, string) (minio.ObjectInfo, error) {
	if s.statErr != nil {
		return minio.ObjectInfo{}, s.statErr
	}
	return s.statInfo, nil
}

func (s *fakeObjectStorage) BucketUploads() string        { return "relay-uploads" }
func (s *fakeObjectStorage) PresignExpiry() time.Duration { return time.Minute }

type fakeUploadACL struct {
	uploads        map[uuid.UUID]acl.Upload
	activeUsers    map[uuid.UUID]bool
	dmMembers      map[[2]uuid.UUID]bool
	channelMembers map[[2]uuid.UUID]bool
	guildMembers   map[[2]uuid.UUID]bool
	completed      uuid.UUID
}

func newFakeUploadACL() *fakeUploadACL {
	return &fakeUploadACL{
		uploads:        map[uuid.UUID]acl.Upload{},
		activeUsers:    map[uuid.UUID]bool{},
		dmMembers:      map[[2]uuid.UUID]bool{},
		channelMembers: map[[2]uuid.UUID]bool{},
		guildMembers:   map[[2]uuid.UUID]bool{},
	}
}

func aclPair(a, b uuid.UUID) [2]uuid.UUID { return [2]uuid.UUID{a, b} }

func (s *fakeUploadACL) InsertPendingUpload(_ context.Context, upload acl.Upload) error {
	s.uploads[upload.ID] = upload
	return nil
}

func (s *fakeUploadACL) CompleteUpload(
	_ context.Context,
	fileID uuid.UUID,
	userID uuid.UUID,
	ownerType string,
	ownerID uuid.UUID,
	url string,
) error {
	upload, ok := s.uploads[fileID]
	if !ok || upload.UploaderID != userID || upload.Status != acl.StatusPending {
		return acl.ErrForbidden
	}
	upload.Status = acl.StatusClean
	upload.OwnerType = ownerType
	upload.OwnerID = &ownerID
	upload.URL = url
	s.uploads[fileID] = upload
	s.completed = fileID
	return nil
}

func (s *fakeUploadACL) GetUpload(_ context.Context, fileID uuid.UUID) (*acl.Upload, error) {
	upload, ok := s.uploads[fileID]
	if !ok {
		return nil, acl.ErrNotFound
	}
	return &upload, nil
}

func (s *fakeUploadACL) IsUserActive(_ context.Context, userID uuid.UUID) (bool, error) {
	return s.activeUsers[userID], nil
}

func (s *fakeUploadACL) IsDMParticipant(_ context.Context, dmChannelID, userID uuid.UUID) (bool, error) {
	return s.dmMembers[aclPair(dmChannelID, userID)], nil
}

func (s *fakeUploadACL) IsChannelMember(_ context.Context, channelID, userID uuid.UUID) (bool, error) {
	return s.channelMembers[aclPair(channelID, userID)], nil
}

func (s *fakeUploadACL) IsGuildMember(_ context.Context, guildID, userID uuid.UUID) (bool, error) {
	return s.guildMembers[aclPair(guildID, userID)], nil
}

func testUploadHandler(store *fakeObjectStorage, aclStore *fakeUploadACL) *UploadHandler {
	return NewUploadHandler(
		store,
		&config.Config{Upload: config.UploadConfig{MaxFileSize: 1024, AllowedMIME: "text/plain,image/png"}},
		aclStore,
		acl.New(aclStore),
	)
}

func requestWithUser(req *http.Request, userID uuid.UUID) *http.Request {
	return req.WithContext(context.WithValue(req.Context(), ctxKeyUserID, userID))
}

func TestCreatePresignedUploadRecordsPendingUploaderACL(t *testing.T) {
	userID := uuid.New()
	aclStore := newFakeUploadACL()
	handler := testUploadHandler(&fakeObjectStorage{}, aclStore)
	body := bytes.NewBufferString(`{"file_name":"note.txt","content_type":"text/plain","file_size":12}`)
	req := requestWithUser(httptest.NewRequest(http.MethodPost, "/upload/presign", body), userID)
	rec := httptest.NewRecorder()

	handler.CreatePresignedUpload(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var res struct {
		FileID string `json:"file_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	fileID, err := uuid.Parse(res.FileID)
	if err != nil {
		t.Fatalf("file_id should be a uuid: %v", err)
	}
	upload := aclStore.uploads[fileID]
	if upload.UploaderID != userID || upload.Status != acl.StatusPending || upload.OwnerType != acl.OwnerPending {
		t.Fatalf("recorded upload = %#v, want pending upload for %s", upload, userID)
	}
}

func TestCompleteUploadRejectsAnotherUsersPendingUpload(t *testing.T) {
	uploaderID := uuid.New()
	otherID := uuid.New()
	dmID := uuid.New()
	fileID := uuid.New()
	aclStore := newFakeUploadACL()
	aclStore.activeUsers[otherID] = true
	aclStore.dmMembers[aclPair(dmID, otherID)] = true
	aclStore.uploads[fileID] = acl.Upload{
		ID:         fileID,
		UploaderID: uploaderID,
		Status:     acl.StatusPending,
		StorageKey: "uploads/" + fileID.String(),
		Bucket:     "relay-uploads",
	}
	handler := testUploadHandler(&fakeObjectStorage{}, aclStore)

	body := bytes.NewBufferString(`{"file_id":"` + fileID.String() + `","key":"uploads/` + fileID.String() + `","file_name":"note.txt","content_type":"text/plain","file_size":12,"owner_type":"dm_channel","owner_id":"` + dmID.String() + `"}`)
	req := requestWithUser(httptest.NewRequest(http.MethodPost, "/upload/complete", body), otherID)
	rec := httptest.NewRecorder()

	handler.CompleteUpload(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	if aclStore.completed != uuid.Nil {
		t.Fatalf("upload should not be completed by unrelated user")
	}
}

func TestCompleteUploadAllowsUploaderForDMContext(t *testing.T) {
	uploaderID := uuid.New()
	dmID := uuid.New()
	fileID := uuid.New()
	aclStore := newFakeUploadACL()
	aclStore.activeUsers[uploaderID] = true
	aclStore.dmMembers[aclPair(dmID, uploaderID)] = true
	aclStore.uploads[fileID] = acl.Upload{
		ID:         fileID,
		UploaderID: uploaderID,
		Status:     acl.StatusPending,
		StorageKey: "uploads/" + fileID.String(),
		Bucket:     "relay-uploads",
	}
	handler := testUploadHandler(&fakeObjectStorage{
		statInfo: minio.ObjectInfo{Size: 12, ContentType: "text/plain"},
	}, aclStore)

	body := bytes.NewBufferString(`{"file_id":"` + fileID.String() + `","key":"uploads/` + fileID.String() + `","file_name":"note.txt","content_type":"text/plain","file_size":12,"owner_type":"dm_channel","owner_id":"` + dmID.String() + `"}`)
	req := requestWithUser(httptest.NewRequest(http.MethodPost, "/upload/complete", body), uploaderID)
	rec := httptest.NewRecorder()

	handler.CompleteUpload(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if aclStore.completed != fileID {
		t.Fatalf("completed upload = %s, want %s", aclStore.completed, fileID)
	}
	var res struct {
		URL      string `json:"url"`
		ProxyURL string `json:"proxy_url"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if res.URL != "/api/v1/media/files/"+fileID.String() || res.ProxyURL != res.URL {
		t.Fatalf("completion returned url=%q proxy_url=%q; want ACL proxy URL only", res.URL, res.ProxyURL)
	}
}

func TestGetFileAllowsDMParticipant(t *testing.T) {
	userID := uuid.New()
	dmID := uuid.New()
	fileID := uuid.New()
	aclStore := newFakeUploadACL()
	aclStore.activeUsers[userID] = true
	aclStore.dmMembers[aclPair(dmID, userID)] = true
	aclStore.uploads[fileID] = acl.Upload{
		ID:         fileID,
		Status:     acl.StatusClean,
		OwnerType:  acl.OwnerDMChannel,
		OwnerID:    &dmID,
		StorageKey: "uploads/" + fileID.String(),
		Bucket:     "relay-uploads",
	}
	handler := testUploadHandler(&fakeObjectStorage{getURL: "https://storage.example/authorized"}, aclStore)

	req := requestWithUser(fileRequest(fileID), userID)
	rec := httptest.NewRecorder()

	handler.GetFile(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusTemporaryRedirect, rec.Body.String())
	}
	if got := rec.Header().Get("Location"); got != "https://storage.example/authorized" {
		t.Fatalf("Location = %q", got)
	}
}

func TestGetFileRejectsNonParticipant(t *testing.T) {
	userID := uuid.New()
	dmID := uuid.New()
	fileID := uuid.New()
	aclStore := newFakeUploadACL()
	aclStore.activeUsers[userID] = true
	aclStore.uploads[fileID] = acl.Upload{
		ID:         fileID,
		Status:     acl.StatusClean,
		OwnerType:  acl.OwnerDMChannel,
		OwnerID:    &dmID,
		StorageKey: "uploads/" + fileID.String(),
		Bucket:     "relay-uploads",
	}
	handler := testUploadHandler(&fakeObjectStorage{}, aclStore)

	req := requestWithUser(fileRequest(fileID), userID)
	rec := httptest.NewRecorder()

	handler.GetFile(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestGetFileRejectsUnknownObjectWithoutListingByUUID(t *testing.T) {
	userID := uuid.New()
	fileID := uuid.New()
	aclStore := newFakeUploadACL()
	handler := testUploadHandler(&fakeObjectStorage{statErr: errors.New("not found")}, aclStore)

	req := requestWithUser(fileRequest(fileID), userID)
	rec := httptest.NewRecorder()

	handler.GetFile(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func fileRequest(fileID uuid.UUID) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/files/"+fileID.String(), nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("fileID", fileID.String())
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
}
