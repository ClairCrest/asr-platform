CREATE OR REPLACE FUNCTION notify_job_event() RETURNS trigger AS $$
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
