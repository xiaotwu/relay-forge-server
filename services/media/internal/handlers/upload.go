package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"

	"github.com/relay-forge/relay-forge/services/media/internal/acl"
	"github.com/relay-forge/relay-forge/services/media/internal/config"
)

type objectStorage interface {
	PresignedPutURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error)
	PresignedGetURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error)
	Stat(ctx context.Context, bucket, key string) (minio.ObjectInfo, error)
	BucketUploads() string
	PresignExpiry() time.Duration
}

type uploadMetadataStore interface {
	InsertPendingUpload(ctx context.Context, upload acl.Upload) error
	CompleteUpload(ctx context.Context, fileID uuid.UUID, userID uuid.UUID, ownerType string, ownerID uuid.UUID, url string) error
}

type UploadHandler struct {
	store      objectStorage
	cfg        *config.Config
	aclStore   uploadMetadataStore
	aclService *acl.Service
}

func NewUploadHandler(
	store objectStorage,
	cfg *config.Config,
	aclStore uploadMetadataStore,
	aclService *acl.Service,
) *UploadHandler {
	return &UploadHandler{store: store, cfg: cfg, aclStore: aclStore, aclService: aclService}
}

type presignRequest struct {
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
	FileSize    int64  `json:"file_size"`
}

type presignResponse struct {
	UploadURL string `json:"upload_url"`
	FileID    string `json:"file_id"`
	Key       string `json:"key"`
}

func (h *UploadHandler) CreatePresignedUpload(w http.ResponseWriter, r *http.Request) {
	var req presignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	req.FileName = sanitizeFileName(req.FileName)
	if req.FileName == "" || strings.TrimSpace(req.ContentType) == "" || req.FileSize <= 0 {
		http.Error(w, `{"error":"file_name, content_type, and file_size are required"}`, http.StatusBadRequest)
		return
	}

	if req.FileSize > h.cfg.Upload.MaxFileSize {
		http.Error(w, `{"error":"file too large"}`, http.StatusRequestEntityTooLarge)
		return
	}

	if !h.isAllowedMIME(req.ContentType) {
		http.Error(w, `{"error":"file type not allowed"}`, http.StatusUnsupportedMediaType)
		return
	}

	userID := GetUserID(r.Context())
	if userID == uuid.Nil {
		http.Error(w, `{"error":"authentication required"}`, http.StatusUnauthorized)
		return
	}

	fileUUID := uuid.New()
	fileID := fileUUID.String()
	key := "uploads/" + fileID

	url, err := h.store.PresignedPutURL(r.Context(), h.store.BucketUploads(), key, h.store.PresignExpiry())
	if err != nil {
		http.Error(w, `{"error":"failed to generate upload URL"}`, http.StatusInternalServerError)
		return
	}

	proxyURL := "/api/v1/media/files/" + fileID
	if err := h.aclStore.InsertPendingUpload(r.Context(), acl.Upload{
		ID:          fileUUID,
		UploaderID:  userID,
		FileName:    req.FileName,
		FileSize:    req.FileSize,
		ContentType: req.ContentType,
		StorageKey:  key,
		Bucket:      h.store.BucketUploads(),
		URL:         proxyURL,
		Status:      acl.StatusPending,
		OwnerType:   acl.OwnerPending,
	}); err != nil {
		http.Error(w, `{"error":"failed to record upload"}`, http.StatusInternalServerError)
		return
	}

	if err := writeJSON(w, http.StatusOK, presignResponse{
		UploadURL: url,
		FileID:    fileID,
		Key:       key,
	}); err != nil {
		http.Error(w, `{"error":"failed to write response"}`, http.StatusInternalServerError)
	}
}

type completeRequest struct {
	FileID      string `json:"file_id"`
	Key         string `json:"key"`
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
	FileSize    int64  `json:"file_size"`
	OwnerType   string `json:"owner_type"`
	OwnerID     string `json:"owner_id"`
}

func (h *UploadHandler) CompleteUpload(w http.ResponseWriter, r *http.Request) {
	var req completeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	req.FileName = sanitizeFileName(req.FileName)
	if strings.TrimSpace(req.FileID) == "" || strings.TrimSpace(req.Key) == "" {
		http.Error(w, `{"error":"file_id and key are required"}`, http.StatusBadRequest)
		return
	}
	fileUUID, err := uuid.Parse(req.FileID)
	if err != nil {
		http.Error(w, `{"error":"invalid file_id"}`, http.StatusBadRequest)
		return
	}
	if req.Key != "uploads/"+req.FileID {
		http.Error(w, `{"error":"upload key does not match file_id"}`, http.StatusBadRequest)
		return
	}
	if req.FileName == "" || strings.TrimSpace(req.ContentType) == "" || req.FileSize <= 0 {
		http.Error(w, `{"error":"file_name, content_type, and file_size are required"}`, http.StatusBadRequest)
		return
	}
	if req.FileSize > h.cfg.Upload.MaxFileSize {
		http.Error(w, `{"error":"file too large"}`, http.StatusRequestEntityTooLarge)
		return
	}
	if !h.isAllowedMIME(req.ContentType) {
		http.Error(w, `{"error":"file type not allowed"}`, http.StatusUnsupportedMediaType)
		return
	}
	if strings.TrimSpace(req.OwnerType) == "" || strings.TrimSpace(req.OwnerID) == "" {
		http.Error(w, `{"error":"owner_type and owner_id are required"}`, http.StatusBadRequest)
		return
	}
	ownerID, err := uuid.Parse(req.OwnerID)
	if err != nil {
		http.Error(w, `{"error":"invalid owner_id"}`, http.StatusBadRequest)
		return
	}

	userID := GetUserID(r.Context())
	upload, err := h.aclService.AuthorizeComplete(r.Context(), userID, fileUUID, req.OwnerType, ownerID)
	if err != nil {
		http.Error(w, aclErrorBody(err), aclHTTPStatus(err))
		return
	}
	if upload.StorageKey != req.Key {
		http.Error(w, `{"error":"upload key does not match recorded upload"}`, http.StatusBadRequest)
		return
	}

	objectInfo, err := h.store.Stat(r.Context(), h.store.BucketUploads(), req.Key)
	if err != nil {
		http.Error(w, `{"error":"uploaded object not found"}`, http.StatusBadRequest)
		return
	}
	if objectInfo.Size != req.FileSize {
		http.Error(w, `{"error":"uploaded object size mismatch"}`, http.StatusBadRequest)
		return
	}
	if objectInfo.ContentType != "" && objectInfo.ContentType != req.ContentType {
		http.Error(w, `{"error":"uploaded object content type mismatch"}`, http.StatusBadRequest)
		return
	}

	proxyURL := "/api/v1/media/files/" + req.FileID
	if err := h.aclStore.CompleteUpload(r.Context(), fileUUID, userID, req.OwnerType, ownerID, proxyURL); err != nil {
		http.Error(w, aclErrorBody(err), aclHTTPStatus(err))
		return
	}

	if err := writeJSON(w, http.StatusOK, map[string]any{
		"id":           req.FileID,
		"file_id":      req.FileID,
		"url":          proxyURL,
		"proxy_url":    proxyURL,
		"filename":     req.FileName,
		"content_type": req.ContentType,
		"size":         req.FileSize,
		"status":       "completed",
	}); err != nil {
		http.Error(w, `{"error":"failed to write response"}`, http.StatusInternalServerError)
	}
}

func (h *UploadHandler) GetFile(w http.ResponseWriter, r *http.Request) {
	fileID := chi.URLParam(r, "fileID")
	fileUUID, err := uuid.Parse(fileID)
	if strings.TrimSpace(fileID) == "" || err != nil {
		http.Error(w, `{"error":"file not found"}`, http.StatusNotFound)
		return
	}

	upload, err := h.aclService.AuthorizeAccess(r.Context(), GetUserID(r.Context()), fileUUID)
	if err != nil {
		http.Error(w, aclErrorBody(err), aclHTTPStatus(err))
		return
	}
	downloadURL, err := h.store.PresignedGetURL(r.Context(), upload.Bucket, upload.StorageKey, h.store.PresignExpiry())
	if err != nil {
		http.Error(w, `{"error":"file not found"}`, http.StatusNotFound)
		return
	}

	http.Redirect(w, r, downloadURL, http.StatusTemporaryRedirect)
}

func aclHTTPStatus(err error) int {
	switch {
	case errors.Is(err, acl.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, acl.ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, acl.ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, acl.ErrInvalidOwner):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func aclErrorBody(err error) string {
	switch aclHTTPStatus(err) {
	case http.StatusNotFound:
		return `{"error":"file not found"}`
	case http.StatusUnauthorized:
		return `{"error":"authentication required"}`
	case http.StatusForbidden:
		return `{"error":"media access denied"}`
	case http.StatusBadRequest:
		return `{"error":"invalid media owner"}`
	default:
		return `{"error":"media authorization failed"}`
	}
}

func sanitizeFileName(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "." || name == "/" || name == string(filepath.Separator) {
		return ""
	}
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, name)
	return strings.TrimSpace(name)
}

func (h *UploadHandler) isAllowedMIME(mime string) bool {
	allowed := h.cfg.Upload.AllowedMIME
	if allowed == "" {
		return true
	}
	for _, a := range strings.Split(allowed, ",") {
		if strings.TrimSpace(a) == mime {
			return true
		}
	}
	return false
}
