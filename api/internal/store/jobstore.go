package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ClairCrest/asr-platform/api/internal/job"
	"github.com/ClairCrest/asr-platform/api/internal/store/db"
)

// JobStore implements job.Store against Postgres via internal/store/db.
type JobStore struct {
	q *db.Queries
}

func NewJobStore(pool db.DBTX) *JobStore {
	return &JobStore{q: db.New(pool)}
}

func (s *JobStore) CreateJob(ctx context.Context, j job.Job) (job.Job, error) {
	row, err := s.q.CreateJob(ctx, db.CreateJobParams{
		ID:               j.ID,
		UserID:           j.UserID,
		Status:           string(j.Status),
		IdempotencyKey:   j.IdempotencyKey,
		ObjectKey:        j.ObjectKey,
		OriginalFilename: j.OriginalFilename,
		SizeBytes:        j.SizeBytes,
		Model:            j.Model,
	})
	if err != nil {
		return job.Job{}, fmt.Errorf("jobstore: create: %w", err)
	}
	return fromDBJob(row), nil
}

func (s *JobStore) GetJob(ctx context.Context, id, userID uuid.UUID) (job.Job, error) {
	row, err := s.q.GetJobByID(ctx, id, userID)
	if err != nil {
		return job.Job{}, mapNotFound(err)
	}
	return fromDBJob(row), nil
}

func (s *JobStore) GetJobByIdempotencyKey(ctx context.Context, userID uuid.UUID, key string) (job.Job, error) {
	row, err := s.q.GetJobByIdempotencyKey(ctx, userID, key)
	if err != nil {
		return job.Job{}, mapNotFound(err)
	}
	return fromDBJob(row), nil
}

func (s *JobStore) ListJobs(ctx context.Context, userID uuid.UUID, status *job.Status, cursor *time.Time, limit int32) ([]job.Job, error) {
	var statusStr *string
	if status != nil {
		s := string(*status)
		statusStr = &s
	}
	rows, err := s.q.ListJobsByUser(ctx, userID, statusStr, cursor, limit)
	if err != nil {
		return nil, fmt.Errorf("jobstore: list: %w", err)
	}
	out := make([]job.Job, 0, len(rows))
	for _, r := range rows {
		out = append(out, fromDBJob(r))
	}
	return out, nil
}

func (s *JobStore) UpdateJobStatus(ctx context.Context, id uuid.UUID, status job.Status) (job.Job, error) {
	row, err := s.q.UpdateJobStatus(ctx, id, string(status))
	if err != nil {
		return job.Job{}, mapNotFound(err)
	}
	return fromDBJob(row), nil
}

func (s *JobStore) CancelJob(ctx context.Context, id, userID uuid.UUID) (job.Job, error) {
	row, err := s.q.CancelJob(ctx, id, userID)
	if err != nil {
		return job.Job{}, mapNotFound(err)
	}
	return fromDBJob(row), nil
}

func (s *JobStore) RetryJob(ctx context.Context, id, userID uuid.UUID) (job.Job, error) {
	row, err := s.q.RetryJob(ctx, id, userID)
	if err != nil {
		return job.Job{}, mapNotFound(err)
	}
	return fromDBJob(row), nil
}

func (s *JobStore) SoftDeleteJob(ctx context.Context, id, userID uuid.UUID) error {
	affected, err := s.q.SoftDeleteJob(ctx, id, userID)
	if err != nil {
		return fmt.Errorf("jobstore: soft delete: %w", err)
	}
	if affected == 0 {
		return job.ErrNotFound
	}
	return nil
}

func (s *JobStore) CreateEvent(ctx context.Context, jobID uuid.UUID, eventType job.EventType, payload []byte) (job.Event, error) {
	row, err := s.q.CreateJobEvent(ctx, uuid.New(), jobID, string(eventType), payload)
	if err != nil {
		return job.Event{}, fmt.Errorf("jobstore: create event: %w", err)
	}
	return fromDBEvent(row), nil
}

func (s *JobStore) ListEvents(ctx context.Context, jobID uuid.UUID) ([]job.Event, error) {
	rows, err := s.q.ListJobEventsByJob(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("jobstore: list events: %w", err)
	}
	out := make([]job.Event, 0, len(rows))
	for _, r := range rows {
		out = append(out, fromDBEvent(r))
	}
	return out, nil
}

func mapNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return job.ErrNotFound
	}
	return fmt.Errorf("jobstore: %w", err)
}

func fromDBJob(r db.Job) job.Job {
	return job.Job{
		ID:               r.ID,
		UserID:           r.UserID,
		Status:           job.Status(r.Status),
		IdempotencyKey:   r.IdempotencyKey,
		ObjectKey:        r.ObjectKey,
		OriginalFilename: r.OriginalFilename,
		SizeBytes:        r.SizeBytes,
		DurationSeconds:  r.DurationSeconds,
		Model:            r.Model,
		Attempts:         r.Attempts,
		MaxAttempts:      r.MaxAttempts,
		ErrorCode:        r.ErrorCode,
		ErrorMessage:     r.ErrorMessage,
		WorkerID:         r.WorkerID,
		LeaseExpiresAt:   r.LeaseExpiresAt,
		CreatedAt:        r.CreatedAt,
		StartedAt:        r.StartedAt,
		FinishedAt:       r.FinishedAt,
	}
}

func fromDBEvent(r db.JobEvent) job.Event {
	return job.Event{
		ID:        r.ID,
		JobID:     r.JobID,
		EventType: job.EventType(r.EventType),
		Payload:   r.Payload,
		CreatedAt: r.CreatedAt,
	}
}
