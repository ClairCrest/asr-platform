package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/ClairCrest/asr-platform/api/internal/http/dto"
	"github.com/ClairCrest/asr-platform/api/internal/http/middleware"
)

const maxUploadSizeBytes = 200 * 1024 * 1024 // 200 MB, per the plan's upload constraints

var acceptedContentTypes = map[string]bool{
	"audio/wav":   true,
	"audio/mpeg":  true,
	"audio/mp4":   true,
	"audio/x-m4a": true,
	"audio/flac":  true,
	"audio/ogg":   true,
}

// ObjectPresigner is the subset of objectstore.Client the upload handler
// needs, declared here so this package does not import objectstore
// directly.
type ObjectPresigner interface {
	PresignPutURL(ctx context.Context, objectKey string) (string, error)
}

type UploadHandler struct {
	objects ObjectPresigner
}

func NewUploadHandler(objects ObjectPresigner) *UploadHandler {
	return &UploadHandler{objects: objects}
}

func (h *UploadHandler) CreateUpload(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		WriteError(w, r, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}

	var req dto.CreateUploadRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, r, http.StatusBadRequest, "invalid_request", "malformed request body")
		return
	}
	if req.Filename == "" {
		WriteError(w, r, http.StatusBadRequest, "invalid_request", "filename is required")
		return
	}
	if req.SizeBytes <= 0 || req.SizeBytes > maxUploadSizeBytes {
		WriteError(w, r, http.StatusBadRequest, "file_too_large", "file must be between 1 byte and 200 MB")
		return
	}
	if !acceptedContentTypes[req.ContentType] {
		WriteError(w, r, http.StatusUnsupportedMediaType, "unsupported_media_type", "unsupported audio content type")
		return
	}

	objectKey := fmt.Sprintf("%s/%s%s", userID, uuid.NewString(), filepath.Ext(req.Filename))

	uploadURL, err := h.objects.PresignPutURL(r.Context(), objectKey)
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "internal_error", "could not create upload url")
		return
	}

	WriteJSON(w, http.StatusOK, dto.CreateUploadResponse{
		UploadURL: uploadURL,
		ObjectKey: objectKey,
	})
}
