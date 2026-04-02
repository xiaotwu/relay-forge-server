package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/relay-forge/relay-forge/services/media/internal/config"
	"github.com/relay-forge/relay-forge/services/media/internal/storage"
)

type UploadHandler struct {
	store *storage.S3Store
	cfg   *config.Config
}

func NewUploadHandler(store *storage.S3Store, cfg *config.Config) *UploadHandler {
	return &UploadHandler{store: store, cfg: cfg}
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

	if req.FileSize > h.cfg.Upload.MaxFileSize {
		http.Error(w, `{"error":"file too large"}`, http.StatusRequestEntityTooLarge)
		return
	}

	if !h.isAllowedMIME(req.ContentType) {
		http.Error(w, `{"error":"file type not allowed"}`, http.StatusUnsupportedMediaType)
		return
	}

	fileID := uuid.New().String()
	ext := ""
	if idx := strings.LastIndex(req.FileName, "."); idx >= 0 {
		ext = req.FileName[idx:]
	}
	key := "uploads/" + fileID + ext

	url, err := h.store.PresignedPutURL(r.Context(), h.store.BucketUploads(), key, h.store.PresignExpiry())
	if err != nil {
		http.Error(w, `{"error":"failed to generate upload URL"}`, http.StatusInternalServerError)
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
}

func (h *UploadHandler) CompleteUpload(w http.ResponseWriter, r *http.Request) {
	var req completeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	downloadURL, err := h.store.PresignedGetURL(r.Context(), h.store.BucketUploads(), req.Key, h.store.PresignExpiry())
	if err != nil {
		http.Error(w, `{"error":"failed to generate download URL"}`, http.StatusInternalServerError)
		return
	}

	if err := writeJSON(w, http.StatusOK, map[string]string{
		"file_id": req.FileID,
		"url":     downloadURL,
		"status":  "completed",
	}); err != nil {
		http.Error(w, `{"error":"failed to write response"}`, http.StatusInternalServerError)
	}
}

func (h *UploadHandler) GetFile(w http.ResponseWriter, r *http.Request) {
	fileID := chi.URLParam(r, "fileID")

	key := "uploads/" + fileID
	downloadURL, err := h.store.PresignedGetURL(r.Context(), h.store.BucketUploads(), key, h.store.PresignExpiry())
	if err != nil {
		http.Error(w, `{"error":"file not found"}`, http.StatusNotFound)
		return
	}

	http.Redirect(w, r, downloadURL, http.StatusTemporaryRedirect)
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
