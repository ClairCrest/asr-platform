-- Extends the job_events NOTIFY payload (000008) with fields the metrics
-- listener needs but that job_events itself doesn't carry: the job's
-- audio duration (for audio_seconds_processed_total, only meaningful on
-- succeeded) and wall-clock seconds since the job was created (for the
-- job_duration_seconds histogram). Same trigger, no new one.
CREATE OR REPLACE FUNCTION notify_job_event() RETURNS trigger AS $$
BEGIN
    PERFORM pg_notify('job_events', (
        SELECT json_build_object(
            'job_id', NEW.job_id,
            'user_id', jobs.user_id,
            'event_type', NEW.event_type,
            'payload', NEW.payload,
            'created_at', NEW.created_at,
            'duration_seconds', jobs.duration_seconds,
            'elapsed_seconds', EXTRACT(EPOCH FROM (NEW.created_at - jobs.created_at))
        )::text
        FROM jobs WHERE jobs.id = NEW.job_id
    ));
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
