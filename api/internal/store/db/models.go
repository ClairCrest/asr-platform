// Package db is the data access layer for the API. It is written by hand
// in the shape sqlc would generate from internal/store/queries/*.sql
// against internal/store/migrations/ — see docs/adr/0001 for why.
package db

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

type ApiKey struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Name       string
	KeyHash    string
	LastUsedAt *time.Time
	CreatedAt  time.Time
	RevokedAt  *time.Time
}

type Job struct {
	ID                uuid.UUID
	UserID            uuid.UUID
	Status            string
	IdempotencyKey    *string
	ObjectKey         string
	OriginalFilename  string
	SizeBytes         int64
	DurationSeconds   *float64
	Model             string
	Attempts          int32
	MaxAttempts       int32
	ErrorCode         *string
	ErrorMessage      *string
	WorkerID          *string
	LeaseExpiresAt    *time.Time
	CreatedAt         time.Time
	StartedAt         *time.Time
	FinishedAt        *time.Time
	DeletedAt         *time.Time
}

type JobEvent struct {
	ID        uuid.UUID
	JobID     uuid.UUID
	EventType string
	Payload   []byte
	CreatedAt time.Time
}
