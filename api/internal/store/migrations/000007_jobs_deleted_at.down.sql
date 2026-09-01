DROP INDEX jobs_user_id_deleted_at_idx;
ALTER TABLE jobs DROP COLUMN deleted_at;
