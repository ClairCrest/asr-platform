package httpapi

import (
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/ClairCrest/asr-platform/api/internal/http/dto"
	"github.com/ClairCrest/asr-platform/api/internal/http/middleware"
	"github.com/ClairCrest/asr-platform/api/internal/job"
)

type JobHandler struct {
	svc *job.Service
}

func NewJobHandler(svc *job.Service) *JobHandler {
	return &JobHandler{svc: svc}
}

func (h *JobHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		WriteError(w, r, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}

	var req dto.CreateJobRequest
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, r, http.StatusBadRequest, "invalid_request", "malformed request body")
		return
	}
	if req.ObjectKey == "" || req.OriginalFilename == "" {
		WriteError(w, r, http.StatusBadRequest, "invalid_request", "object_key and original_filename are required")
		return
	}
	model := req.Model
	if model == "" {
		model = "small.en"
	}

	params := job.CreateParams{
		UserID:           userID,
		ObjectKey:        req.ObjectKey,
		OriginalFilename: req.OriginalFilename,
		Model:            model,
	}
	if key := r.Header.Get("Idempotency-Key"); key != "" {
		params.IdempotencyKey = &key
	}

	j, err := h.svc.Create(r.Context(), params)
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "internal_error", "could not create job")
		return
	}
	WriteJSON(w, http.StatusCreated, toJobResponse(j))
}

func (h *JobHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		WriteError(w, r, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	jobID, err := parseUUIDParam(r, "id")
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "invalid_request", "invalid job id")
		return
	}

	j, events, err := h.svc.Get(r.Context(), userID, jobID)
	if err != nil {
		writeJobError(w, r, err)
		return
	}

	resp := dto.JobDetailResponse{JobResponse: toJobResponse(j)}
	for _, e := range events {
		resp.Events = append(resp.Events, dto.JobEventResponse{
			EventType: string(e.EventType),
			Payload:   e.Payload,
			CreatedAt: e.CreatedAt,
		})
	}
	WriteJSON(w, http.StatusOK, resp)
}

func (h *JobHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		WriteError(w, r, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}

	var status *job.Status
	if raw := r.URL.Query().Get("status"); raw != "" {
		s := job.Status(raw)
		status = &s
	}

	var cursor *time.Time
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		decoded, err := decodeCursor(raw)
		if err != nil {
			WriteError(w, r, http.StatusBadRequest, "invalid_request", "invalid cursor")
			return
		}
		cursor = &decoded
	}

	limit := int32(20)
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = int32(parsed)
		}
	}

	jobs, err := h.svc.List(r.Context(), userID, status, cursor, limit)
	if err != nil {
		WriteError(w, r, http.StatusInternalServerError, "internal_error", "could not list jobs")
		return
	}

	resp := dto.JobListResponse{Jobs: make([]dto.JobResponse, 0, len(jobs))}
	for _, j := range jobs {
		resp.Jobs = append(resp.Jobs, toJobResponse(j))
	}
	if int32(len(jobs)) == limit && len(jobs) > 0 {
		next := encodeCursor(jobs[len(jobs)-1].CreatedAt)
		resp.NextCursor = &next
	}
	WriteJSON(w, http.StatusOK, resp)
}

func (h *JobHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		WriteError(w, r, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	jobID, err := parseUUIDParam(r, "id")
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "invalid_request", "invalid job id")
		return
	}

	j, err := h.svc.Cancel(r.Context(), userID, jobID)
	if err != nil {
		writeJobError(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, toJobResponse(j))
}

func (h *JobHandler) Retry(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		WriteError(w, r, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	jobID, err := parseUUIDParam(r, "id")
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "invalid_request", "invalid job id")
		return
	}

	j, err := h.svc.Retry(r.Context(), userID, jobID)
	if err != nil {
		writeJobError(w, r, err)
		return
	}
	WriteJSON(w, http.StatusOK, toJobResponse(j))
}

func (h *JobHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.UserID(r.Context())
	if !ok {
		WriteError(w, r, http.StatusUnauthorized, "unauthorized", "not authenticated")
		return
	}
	jobID, err := parseUUIDParam(r, "id")
	if err != nil {
		WriteError(w, r, http.StatusBadRequest, "invalid_request", "invalid job id")
		return
	}

	if err := h.svc.Delete(r.Context(), userID, jobID); err != nil {
		writeJobError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJobError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, job.ErrNotFound):
		WriteError(w, r, http.StatusNotFound, "job_not_found", "job not found")
	case errors.Is(err, job.ErrNotCancellable):
		WriteError(w, r, http.StatusConflict, "job_not_cancellable", "job is not in a cancellable state")
	case errors.Is(err, job.ErrNotRetryable):
		WriteError(w, r, http.StatusConflict, "job_not_retryable", "job is not in a retryable state")
	default:
		WriteError(w, r, http.StatusInternalServerError, "internal_error", "an unexpected error occurred")
	}
}

func toJobResponse(j job.Job) dto.JobResponse {
	return dto.JobResponse{
		ID:               j.ID,
		Status:           string(j.Status),
		ObjectKey:        j.ObjectKey,
		OriginalFilename: j.OriginalFilename,
		SizeBytes:        j.SizeBytes,
		DurationSeconds:  j.DurationSeconds,
		Model:            j.Model,
		Attempts:         j.Attempts,
		MaxAttempts:      j.MaxAttempts,
		ErrorCode:        j.ErrorCode,
		ErrorMessage:     j.ErrorMessage,
		CreatedAt:        j.CreatedAt,
		StartedAt:        j.StartedAt,
		FinishedAt:       j.FinishedAt,
	}
}

// Cursors are the RFC3339Nano timestamp of the last row on the page,
// base64-encoded so the wire format has no implied ordering semantics a
// client might rely on.
func encodeCursor(t time.Time) string {
	return base64.RawURLEncoding.EncodeToString([]byte(t.Format(time.RFC3339Nano)))
}

func decodeCursor(raw string) (time.Time, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return time.Time{}, err
	}
	return time.Parse(time.RFC3339Nano, string(decoded))
}
