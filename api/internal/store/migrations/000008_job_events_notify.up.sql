-- Fans job_events out to LISTEN/NOTIFY so the API's WebSocket hub learns
-- about status changes regardless of whether they were written by the API
-- (user-initiated cancel/retry) or by a worker (transcription progress).
-- A trigger is used rather than notifying from application code so no
-- write path can accidentally skip it.
CREATE FUNCTION notify_job_event() RETURNS trigger AS $$
BEGIN
    PERFORM pg_notify('job_events', json_build_object(
        'job_id', NEW.job_id,
        'user_id', (SELECT user_id FROM jobs WHERE id = NEW.job_id),
        'event_type', NEW.event_type,
        'payload', NEW.payload,
        'created_at', NEW.created_at
    )::text);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER job_events_notify
    AFTER INSERT ON job_events
    FOR EACH ROW EXECUTE FUNCTION notify_job_event();
