// Package job owns the job state machine. No other package may mutate a
// job's status directly; every transition goes through Service and writes
// a job_events row.
package job

import (
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusPending    Status = "pending"
	StatusQueued     Status = "queued"
	StatusProcessing Status = "processing"
	StatusRetrying   Status = "retrying"
	StatusSucceeded  Status = "succeeded"
	StatusFailed     Status = "failed"
	StatusCancelled  Status = "cancelled"
)

type EventType string

const (
	EventCreated   EventType = "created"
	EventQueued    EventType = "queued"
	EventLeased    EventType = "leased"
	EventProgress  EventType = "progress"
	EventSucceeded EventType = "succeeded"
	EventFailed    EventType = "failed"
	EventRetrying  EventType = "retrying"
	EventCancelled EventType = "cancelled"
)

type Job struct {
	ID                uuid.UUID
	UserID            uuid.UUID
	Status            Status
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
}

type Event struct {
	ID        uuid.UUID
	JobID     uuid.UUID
	EventType EventType
	Payload   []byte
	CreatedAt time.Time
}

// transitions enumerates every status a job may move to from a given
// status. A transition not listed here is rejected by the service layer
// regardless of which code path attempts it.
var transitions = map[Status][]Status{
	StatusPending:    {StatusQueued},
	StatusQueued:     {StatusProcessing, StatusCancelled},
	StatusProcessing: {StatusSucceeded, StatusFailed, StatusRetrying, StatusCancelled},
	StatusRetrying:   {StatusQueued},
}

// CanTransition reports whether a job may move from from to to.
func CanTransition(from, to Status) bool {
	for _, next := range transitions[from] {
		if next == to {
			return true
		}
	}
	return false
}

// CanCancel reports whether a job in status may be cancelled by the user.
func CanCancel(status Status) bool {
	return status == StatusQueued || status == StatusProcessing
}

// CanRetry reports whether a job in status may be retried by the user.
func CanRetry(status Status) bool {
	return status == StatusFailed
}
