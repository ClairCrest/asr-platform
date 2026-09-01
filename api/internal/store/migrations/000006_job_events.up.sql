CREATE TABLE job_events (
    id         uuid PRIMARY KEY,
    job_id     uuid NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    event_type text NOT NULL CHECK (event_type IN (
                   'created', 'queued', 'leased', 'progress',
                   'succeeded', 'failed', 'retrying', 'cancelled'
               )),
    payload    jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX job_events_job_id_created_at_idx ON job_events(job_id, created_at);
