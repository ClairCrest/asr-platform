-- name: CreateJobEvent :one
INSERT INTO job_events (id, job_id, event_type, payload)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListJobEventsByJob :many
SELECT * FROM job_events
WHERE job_id = $1
ORDER BY created_at ASC;

-- name: ListLatestJobEventsByJob :many
SELECT * FROM job_events
WHERE job_id = $1
ORDER BY created_at DESC
LIMIT $2;
