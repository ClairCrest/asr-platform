CREATE TABLE transcripts (
    id                    uuid PRIMARY KEY,
    job_id                uuid NOT NULL UNIQUE REFERENCES jobs(id) ON DELETE CASCADE,
    text                  text NOT NULL,
    language_detected     text NOT NULL,
    language_probability  double precision NOT NULL,
    model                 text NOT NULL,
    processing_seconds    double precision NOT NULL,
    real_time_factor      double precision NOT NULL,
    created_at            timestamptz NOT NULL DEFAULT now()
);
