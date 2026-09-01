package dto

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type CreateUploadRequest struct {
	Filename    string `json:"filename"`
	SizeBytes   int64  `json:"size_bytes"`
	ContentType string `json:"content_type"`
}

type CreateUploadResponse struct {
	UploadURL string `json:"upload_url"`
	ObjectKey string `json:"object_key"`
}

type CreateJobRequest struct {
	ObjectKey        string `json:"object_key"`
	OriginalFilename string `json:"original_filename"`
	Model            string `json:"model,omitempty"`
}

type JobResponse struct {
	ID               uuid.UUID  `json:"id"`
	Status           string     `json:"status"`
	ObjectKey        string     `json:"object_key"`
	OriginalFilename string     `json:"original_filename"`
	SizeBytes        int64      `json:"size_bytes"`
	DurationSeconds  *float64   `json:"duration_seconds,omitempty"`
	Model            string     `json:"model"`
	Attempts         int32      `json:"attempts"`
	MaxAttempts      int32      `json:"max_attempts"`
	ErrorCode        *string    `json:"error_code,omitempty"`
	ErrorMessage     *string    `json:"error_message,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	StartedAt        *time.Time `json:"started_at,omitempty"`
	FinishedAt       *time.Time `json:"finished_at,omitempty"`
}

type JobEventResponse struct {
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt time.Time       `json:"created_at"`
}

type JobDetailResponse struct {
	JobResponse
	Events []JobEventResponse `json:"events"`
}

type JobListResponse struct {
	Jobs       []JobResponse `json:"jobs"`
	NextCursor *string       `json:"next_cursor,omitempty"`
}
