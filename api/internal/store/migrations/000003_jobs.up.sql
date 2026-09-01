CREATE TABLE jobs (
    id                uuid PRIMARY KEY,
    user_id           uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status            text NOT NULL CHECK (status IN (
                          'pending', 'queued', 'processing', 'retrying',
                          'succeeded', 'failed', 'cancelled'
                      )),
    idempotency_key   text,
    object_key        text NOT NULL,
    original_filename text NOT NULL,
    size_bytes        bigint NOT NULL,
    duration_seconds  double precision,
    model             text NOT NULL DEFAULT 'small.en',
    attempts          int NOT NULL DEFAULT 0,
    max_attempts      int NOT NULL DEFAULT 3,
    error_code        text,
    error_message     text,
    worker_id         text,
    lease_expires_at  timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now(),
    started_at        timestamptz,
    finished_at       timestamptz
);

CREATE UNIQUE INDEX jobs_user_idempotency_key_idx
    ON jobs(user_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE INDEX jobs_user_id_created_at_idx ON jobs(user_id, created_at DESC);
CREATE INDEX jobs_status_idx ON jobs(status);
CREATE INDEX jobs_status_lease_expires_at_idx ON jobs(status, lease_expires_at);
