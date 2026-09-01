ALTER TABLE jobs ADD COLUMN deleted_at timestamptz;

CREATE INDEX jobs_user_id_deleted_at_idx ON jobs(user_id, deleted_at);
