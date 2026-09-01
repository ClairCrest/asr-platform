package db

import (
	"context"
	"time"

	"github.com/google/uuid"
)

const createJob = `
INSERT INTO jobs (
    id, user_id, status, idempotency_key, object_key,
    original_filename, size_bytes, model
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING id, user_id, status, idempotency_key, object_key, original_filename,
          size_bytes, duration_seconds, model, attempts, max_attempts,
          error_code, error_message, worker_id, lease_expires_at,
          created_at, started_at, finished_at, deleted_at
`

type CreateJobParams struct {
	ID               uuid.UUID
	UserID           uuid.UUID
	Status           string
	IdempotencyKey   *string
	ObjectKey        string
	OriginalFilename string
	SizeBytes        int64
	Model            string
}

func (q *Queries) CreateJob(ctx context.Context, p CreateJobParams) (Job, error) {
	row := q.db.QueryRow(ctx, createJob,
		p.ID, p.UserID, p.Status, p.IdempotencyKey, p.ObjectKey,
		p.OriginalFilename, p.SizeBytes, p.Model,
	)
	return scanJob(row)
}

const getJobByID = `
SELECT id, user_id, status, idempotency_key, object_key, original_filename,
       size_bytes, duration_seconds, model, attempts, max_attempts,
       error_code, error_message, worker_id, lease_expires_at,
       created_at, started_at, finished_at, deleted_at
FROM jobs
WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
`

func (q *Queries) GetJobByID(ctx context.Context, id, userID uuid.UUID) (Job, error) {
	row := q.db.QueryRow(ctx, getJobByID, id, userID)
	return scanJob(row)
}

const getJobByIdempotencyKey = `
SELECT id, user_id, status, idempotency_key, object_key, original_filename,
       size_bytes, duration_seconds, model, attempts, max_attempts,
       error_code, error_message, worker_id, lease_expires_at,
       created_at, started_at, finished_at, deleted_at
FROM jobs
WHERE user_id = $1 AND idempotency_key = $2 AND deleted_at IS NULL
`

func (q *Queries) GetJobByIdempotencyKey(ctx context.Context, userID uuid.UUID, key string) (Job, error) {
	row := q.db.QueryRow(ctx, getJobByIdempotencyKey, userID, key)
	return scanJob(row)
}

const listJobsByUser = `
SELECT id, user_id, status, idempotency_key, object_key, original_filename,
       size_bytes, duration_seconds, model, attempts, max_attempts,
       error_code, error_message, worker_id, lease_expires_at,
       created_at, started_at, finished_at, deleted_at
FROM jobs
WHERE user_id = $1
  AND deleted_at IS NULL
  AND ($2::text IS NULL OR status = $2)
  AND ($3::timestamptz IS NULL OR created_at < $3)
ORDER BY created_at DESC
LIMIT $4
`

func (q *Queries) ListJobsByUser(ctx context.Context, userID uuid.UUID, status *string, cursor *time.Time, limit int32) ([]Job, error) {
	rows, err := q.db.Query(ctx, listJobsByUser, userID, status, cursor, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := []Job{}
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

const updateJobStatus = `
UPDATE jobs
SET status = $2,
    started_at = CASE WHEN $2 = 'processing' AND started_at IS NULL THEN now() ELSE started_at END,
    finished_at = CASE WHEN $2 IN ('succeeded', 'failed', 'cancelled') THEN now() ELSE finished_at END
WHERE id = $1 AND deleted_at IS NULL
RETURNING id, user_id, status, idempotency_key, object_key, original_filename,
          size_bytes, duration_seconds, model, attempts, max_attempts,
          error_code, error_message, worker_id, lease_expires_at,
          created_at, started_at, finished_at, deleted_at
`

func (q *Queries) UpdateJobStatus(ctx context.Context, id uuid.UUID, status string) (Job, error) {
	row := q.db.QueryRow(ctx, updateJobStatus, id, status)
	return scanJob(row)
}

const retryJob = `
UPDATE jobs
SET status = 'queued',
    attempts = 0,
    error_code = NULL,
    error_message = NULL,
    worker_id = NULL,
    lease_expires_at = NULL,
    started_at = NULL,
    finished_at = NULL
WHERE id = $1 AND user_id = $2 AND status = 'failed' AND deleted_at IS NULL
RETURNING id, user_id, status, idempotency_key, object_key, original_filename,
          size_bytes, duration_seconds, model, attempts, max_attempts,
          error_code, error_message, worker_id, lease_expires_at,
          created_at, started_at, finished_at, deleted_at
`

func (q *Queries) RetryJob(ctx context.Context, id, userID uuid.UUID) (Job, error) {
	row := q.db.QueryRow(ctx, retryJob, id, userID)
	return scanJob(row)
}

const cancelJob = `
UPDATE jobs
SET status = 'cancelled', finished_at = now()
WHERE id = $1 AND user_id = $2 AND status IN ('queued', 'processing') AND deleted_at IS NULL
RETURNING id, user_id, status, idempotency_key, object_key, original_filename,
          size_bytes, duration_seconds, model, attempts, max_attempts,
          error_code, error_message, worker_id, lease_expires_at,
          created_at, started_at, finished_at, deleted_at
`

func (q *Queries) CancelJob(ctx context.Context, id, userID uuid.UUID) (Job, error) {
	row := q.db.QueryRow(ctx, cancelJob, id, userID)
	return scanJob(row)
}

const listExpiredLeases = `
SELECT id, user_id, status, idempotency_key, object_key, original_filename,
       size_bytes, duration_seconds, model, attempts, max_attempts,
       error_code, error_message, worker_id, lease_expires_at,
       created_at, started_at, finished_at, deleted_at
FROM jobs
WHERE status = 'processing' AND lease_expires_at < now() AND deleted_at IS NULL
`

// ListExpiredLeases returns every processing job whose lease has expired,
// for the reaper to requeue or fail.
func (q *Queries) ListExpiredLeases(ctx context.Context) ([]Job, error) {
	rows, err := q.db.Query(ctx, listExpiredLeases)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := []Job{}
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

const requeueExpiredJob = `
UPDATE jobs
SET status = 'queued', attempts = attempts + 1, worker_id = NULL, lease_expires_at = NULL
WHERE id = $1 AND status = 'processing'
RETURNING id, user_id, status, idempotency_key, object_key, original_filename,
          size_bytes, duration_seconds, model, attempts, max_attempts,
          error_code, error_message, worker_id, lease_expires_at,
          created_at, started_at, finished_at, deleted_at
`

// RequeueExpiredJob increments attempts and moves a job whose lease
// expired back to queued, for redelivery.
func (q *Queries) RequeueExpiredJob(ctx context.Context, id uuid.UUID) (Job, error) {
	row := q.db.QueryRow(ctx, requeueExpiredJob, id)
	return scanJob(row)
}

const failExpiredJob = `
UPDATE jobs
SET status = 'failed', attempts = attempts + 1,
    error_code = $2, error_message = $3, finished_at = now()
WHERE id = $1 AND status = 'processing'
RETURNING id, user_id, status, idempotency_key, object_key, original_filename,
          size_bytes, duration_seconds, model, attempts, max_attempts,
          error_code, error_message, worker_id, lease_expires_at,
          created_at, started_at, finished_at, deleted_at
`

// FailExpiredJob terminates a job whose lease expired and that has already
// used up its retry budget.
func (q *Queries) FailExpiredJob(ctx context.Context, id uuid.UUID, errorCode, errorMessage string) (Job, error) {
	row := q.db.QueryRow(ctx, failExpiredJob, id, errorCode, errorMessage)
	return scanJob(row)
}

const softDeleteJob = `
UPDATE jobs
SET deleted_at = now()
WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
`

func (q *Queries) SoftDeleteJob(ctx context.Context, id, userID uuid.UUID) (int64, error) {
	tag, err := q.db.Exec(ctx, softDeleteJob, id, userID)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanJob(row rowScanner) (Job, error) {
	var j Job
	err := row.Scan(
		&j.ID, &j.UserID, &j.Status, &j.IdempotencyKey, &j.ObjectKey, &j.OriginalFilename,
		&j.SizeBytes, &j.DurationSeconds, &j.Model, &j.Attempts, &j.MaxAttempts,
		&j.ErrorCode, &j.ErrorMessage, &j.WorkerID, &j.LeaseExpiresAt,
		&j.CreatedAt, &j.StartedAt, &j.FinishedAt, &j.DeletedAt,
	)
	return j, err
}
