package job

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Store is the persistence boundary the job service depends on. It speaks
// in domain types (Job, Event) so this package never imports the sqlc or
// pgx packages directly, which keeps Service testable with an in-memory
// fake instead of a real database.
//
// Every method that returns a single Job or Event returns ErrNotFound when
// no row matches, so Service never has to know about sql.ErrNoRows.
type Store interface {
	CreateJob(ctx context.Context, j Job) (Job, error)
	GetJob(ctx context.Context, id, userID uuid.UUID) (Job, error)
	GetJobByIdempotencyKey(ctx context.Context, userID uuid.UUID, key string) (Job, error)
	ListJobs(ctx context.Context, userID uuid.UUID, status *Status, cursor *time.Time, limit int32) ([]Job, error)
	UpdateJobStatus(ctx context.Context, id uuid.UUID, status Status) (Job, error)
	CancelJob(ctx context.Context, id, userID uuid.UUID) (Job, error)
	RetryJob(ctx context.Context, id, userID uuid.UUID) (Job, error)
	SoftDeleteJob(ctx context.Context, id, userID uuid.UUID) error

	CreateEvent(ctx context.Context, jobID uuid.UUID, eventType EventType, payload []byte) (Event, error)
	ListEvents(ctx context.Context, jobID uuid.UUID) ([]Event, error)
}

// Queue is the outbound boundary to the job stream. Enqueue is called once
// per job, right after it is persisted as queued.
type Queue interface {
	Enqueue(ctx context.Context, jobID uuid.UUID) error
}

// ObjectStore is the outbound boundary to the audio object store, used only
// to remove the source object when a job is deleted.
type ObjectStore interface {
	DeleteObject(ctx context.Context, key string) error
}

type Service struct {
	store   Store
	queue   Queue
	objects ObjectStore
}

func NewService(store Store, queue Queue, objects ObjectStore) *Service {
	return &Service{store: store, queue: queue, objects: objects}
}

type CreateParams struct {
	UserID           uuid.UUID
	IdempotencyKey   *string
	ObjectKey        string
	OriginalFilename string
	SizeBytes        int64
	Model            string
}

// Create inserts a new job, enqueues it, and returns it in status queued.
// If IdempotencyKey is set and a job already exists for this user with that
// key, the existing job is returned unchanged rather than creating a
// duplicate — this is what makes job creation safe to retry.
func (s *Service) Create(ctx context.Context, p CreateParams) (Job, error) {
	if p.IdempotencyKey != nil {
		existing, err := s.store.GetJobByIdempotencyKey(ctx, p.UserID, *p.IdempotencyKey)
		if err == nil {
			return existing, nil
		}
		if err != ErrNotFound {
			return Job{}, fmt.Errorf("job: check idempotency key: %w", err)
		}
	}

	j := Job{
		ID:               uuid.New(),
		UserID:           p.UserID,
		Status:           StatusPending,
		IdempotencyKey:   p.IdempotencyKey,
		ObjectKey:        p.ObjectKey,
		OriginalFilename: p.OriginalFilename,
		SizeBytes:        p.SizeBytes,
		Model:            p.Model,
		MaxAttempts:      3,
	}

	created, err := s.store.CreateJob(ctx, j)
	if err != nil {
		return Job{}, fmt.Errorf("job: create: %w", err)
	}
	if err := s.recordEvent(ctx, created.ID, EventCreated, nil); err != nil {
		return Job{}, err
	}

	if err := s.queue.Enqueue(ctx, created.ID); err != nil {
		return Job{}, fmt.Errorf("job: enqueue: %w", err)
	}

	queued, err := s.transition(ctx, created.ID, StatusQueued, EventQueued)
	if err != nil {
		return Job{}, err
	}
	return queued, nil
}

// Get returns a job and its full event history, scoped to userID so a user
// can never read another user's job.
func (s *Service) Get(ctx context.Context, userID, jobID uuid.UUID) (Job, []Event, error) {
	j, err := s.store.GetJob(ctx, jobID, userID)
	if err != nil {
		return Job{}, nil, err
	}
	events, err := s.store.ListEvents(ctx, jobID)
	if err != nil {
		return Job{}, nil, fmt.Errorf("job: list events: %w", err)
	}
	return j, events, nil
}

// List returns a page of jobs for userID, newest first, optionally filtered
// by status and paginated by a created_at keyset cursor.
func (s *Service) List(ctx context.Context, userID uuid.UUID, status *Status, cursor *time.Time, limit int32) ([]Job, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	jobs, err := s.store.ListJobs(ctx, userID, status, cursor, limit)
	if err != nil {
		return nil, fmt.Errorf("job: list: %w", err)
	}
	return jobs, nil
}

// Cancel moves a queued or processing job to cancelled. It returns
// ErrNotCancellable if the job exists but is in a terminal or otherwise
// non-cancellable state.
func (s *Service) Cancel(ctx context.Context, userID, jobID uuid.UUID) (Job, error) {
	current, err := s.store.GetJob(ctx, jobID, userID)
	if err != nil {
		return Job{}, err
	}
	if !CanCancel(current.Status) {
		return Job{}, ErrNotCancellable
	}

	updated, err := s.store.CancelJob(ctx, jobID, userID)
	if err != nil {
		return Job{}, fmt.Errorf("job: cancel: %w", err)
	}
	if err := s.recordEvent(ctx, jobID, EventCancelled, nil); err != nil {
		return Job{}, err
	}
	return updated, nil
}

// Retry resets a failed job back to queued with a fresh attempt counter. It
// returns ErrNotRetryable if the job exists but is not in status failed.
func (s *Service) Retry(ctx context.Context, userID, jobID uuid.UUID) (Job, error) {
	current, err := s.store.GetJob(ctx, jobID, userID)
	if err != nil {
		return Job{}, err
	}
	if !CanRetry(current.Status) {
		return Job{}, ErrNotRetryable
	}

	updated, err := s.store.RetryJob(ctx, jobID, userID)
	if err != nil {
		return Job{}, fmt.Errorf("job: retry: %w", err)
	}
	if err := s.recordEvent(ctx, jobID, EventRetrying, nil); err != nil {
		return Job{}, err
	}

	if err := s.queue.Enqueue(ctx, jobID); err != nil {
		return Job{}, fmt.Errorf("job: enqueue retry: %w", err)
	}
	if err := s.recordEvent(ctx, jobID, EventQueued, nil); err != nil {
		return Job{}, err
	}
	return updated, nil
}

// Delete soft-deletes a job and best-effort removes its source audio
// object. Object removal failure is not fatal: the job is already gone
// from the user's perspective, and an orphaned object is a cleanup
// concern, not a correctness one.
func (s *Service) Delete(ctx context.Context, userID, jobID uuid.UUID) error {
	current, err := s.store.GetJob(ctx, jobID, userID)
	if err != nil {
		return err
	}
	if err := s.store.SoftDeleteJob(ctx, jobID, userID); err != nil {
		return fmt.Errorf("job: delete: %w", err)
	}
	_ = s.objects.DeleteObject(ctx, current.ObjectKey)
	return nil
}

func (s *Service) transition(ctx context.Context, jobID uuid.UUID, to Status, event EventType) (Job, error) {
	updated, err := s.store.UpdateJobStatus(ctx, jobID, to)
	if err != nil {
		return Job{}, fmt.Errorf("job: transition to %s: %w", to, err)
	}
	if err := s.recordEvent(ctx, jobID, event, nil); err != nil {
		return Job{}, err
	}
	return updated, nil
}

func (s *Service) recordEvent(ctx context.Context, jobID uuid.UUID, eventType EventType, payload map[string]any) error {
	body := []byte("{}")
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("job: encode event payload: %w", err)
		}
		body = encoded
	}
	if _, err := s.store.CreateEvent(ctx, jobID, eventType, body); err != nil {
		return fmt.Errorf("job: record event %s: %w", eventType, err)
	}
	return nil
}
