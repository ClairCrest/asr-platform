-- name: GetTranscriptByJobID :one
SELECT * FROM transcripts WHERE job_id = $1;

-- name: ListSegmentsByTranscript :many
SELECT * FROM segments WHERE transcript_id = $1 ORDER BY idx ASC;
