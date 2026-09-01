package db

import (
	"context"

	"github.com/google/uuid"
)

const createJobEvent = `
INSERT INTO job_events (id, job_id, event_type, payload)
VALUES ($1, $2, $3, $4)
RETURNING id, job_id, event_type, payload, created_at
`

func (q *Queries) CreateJobEvent(ctx context.Context, id, jobID uuid.UUID, eventType string, payload []byte) (JobEvent, error) {
	row := q.db.QueryRow(ctx, createJobEvent, id, jobID, eventType, payload)
	var e JobEvent
	err := row.Scan(&e.ID, &e.JobID, &e.EventType, &e.Payload, &e.CreatedAt)
	return e, err
}

const listJobEventsByJob = `
SELECT id, job_id, event_type, payload, created_at
FROM job_events
WHERE job_id = $1
ORDER BY created_at ASC
`

func (q *Queries) ListJobEventsByJob(ctx context.Context, jobID uuid.UUID) ([]JobEvent, error) {
	rows, err := q.db.Query(ctx, listJobEventsByJob, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []JobEvent{}
	for rows.Next() {
		var e JobEvent
		if err := rows.Scan(&e.ID, &e.JobID, &e.EventType, &e.Payload, &e.CreatedAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}
