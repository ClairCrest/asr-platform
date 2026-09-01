-- name: CreateJob :one
INSERT INTO jobs (
    id, user_id, status, idempotency_key, object_key,
    original_filename, size_bytes, model
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING *;

-- name: GetJobByID :one
SELECT * FROM jobs
WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL;

-- name: GetJobByIdempotencyKey :one
SELECT * FROM jobs
WHERE user_id = $1 AND idempotency_key = $2 AND deleted_at IS NULL;

-- name: ListJobsByUser :many
SELECT * FROM jobs
WHERE user_id = sqlc.arg(user_id)
  AND deleted_at IS NULL
  AND (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status))
  AND (sqlc.narg(cursor_created_at)::timestamptz IS NULL OR created_at < sqlc.narg(cursor_created_at))
ORDER BY created_at DESC
LIMIT sqlc.arg(row_limit);

-- name: UpdateJobStatus :one
UPDATE jobs
SET status = $2,
    started_at = CASE WHEN $2 = 'processing' AND started_at IS NULL THEN now() ELSE started_at END,
    finished_at = CASE WHEN $2 IN ('succeeded', 'failed', 'cancelled') THEN now() ELSE finished_at END
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: RetryJob :one
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
RETURNING *;

-- name: CancelJob :one
UPDATE jobs
SET status = 'cancelled', finished_at = now()
WHERE id = $1 AND user_id = $2 AND status IN ('queued', 'processing') AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteJob :execrows
UPDATE jobs
SET deleted_at = now()
WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL;
