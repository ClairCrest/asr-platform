package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/ClairCrest/asr-platform/api/internal/queue"
	"github.com/ClairCrest/asr-platform/api/internal/store/db"
)

// LeaseStore implements queue.LeaseStore against Postgres, reusing the
// same *db.Queries as JobStore since both operate on the jobs table.
type LeaseStore struct {
	q *db.Queries
}

func NewLeaseStore(pool db.DBTX) *LeaseStore {
	return &LeaseStore{q: db.New(pool)}
}

func (s *LeaseStore) ListExpiredLeases(ctx context.Context) ([]queue.ExpiredJob, error) {
	rows, err := s.q.ListExpiredLeases(ctx)
	if err != nil {
		return nil, fmt.Errorf("leasestore: list expired leases: %w", err)
	}
	out := make([]queue.ExpiredJob, 0, len(rows))
	for _, r := range rows {
		out = append(out, queue.ExpiredJob{ID: r.ID, Attempts: r.Attempts, MaxAttempts: r.MaxAttempts})
	}
	return out, nil
}

func (s *LeaseStore) RequeueExpiredJob(ctx context.Context, id uuid.UUID) error {
	if _, err := s.q.RequeueExpiredJob(ctx, id); err != nil {
		return fmt.Errorf("leasestore: requeue: %w", err)
	}
	if _, err := s.q.CreateJobEvent(ctx, uuid.New(), id, "retrying", []byte(`{"reason":"lease_expired"}`)); err != nil {
		return fmt.Errorf("leasestore: record retrying event: %w", err)
	}
	if _, err := s.q.CreateJobEvent(ctx, uuid.New(), id, "queued", []byte("{}")); err != nil {
		return fmt.Errorf("leasestore: record queued event: %w", err)
	}
	return nil
}

func (s *LeaseStore) FailExpiredJob(ctx context.Context, id uuid.UUID, errorCode, errorMessage string) error {
	if _, err := s.q.FailExpiredJob(ctx, id, errorCode, errorMessage); err != nil {
		return fmt.Errorf("leasestore: fail: %w", err)
	}
	payload := []byte(fmt.Sprintf(`{"error_code":%q,"error_message":%q}`, errorCode, errorMessage))
	if _, err := s.q.CreateJobEvent(ctx, uuid.New(), id, "failed", payload); err != nil {
		return fmt.Errorf("leasestore: record failed event: %w", err)
	}
	return nil
}
