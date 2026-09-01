package queue

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
)

type fakeLeaseStore struct {
	expired  []ExpiredJob
	requeued []uuid.UUID
	failed   []uuid.UUID
	listErr  error
}

func (f *fakeLeaseStore) ListExpiredLeases(_ context.Context) ([]ExpiredJob, error) {
	return f.expired, f.listErr
}

func (f *fakeLeaseStore) RequeueExpiredJob(_ context.Context, id uuid.UUID) error {
	f.requeued = append(f.requeued, id)
	return nil
}

func (f *fakeLeaseStore) FailExpiredJob(_ context.Context, id uuid.UUID, _, _ string) error {
	f.failed = append(f.failed, id)
	return nil
}

type fakeEnqueuer struct {
	enqueued []uuid.UUID
}

func (f *fakeEnqueuer) Enqueue(_ context.Context, id uuid.UUID) error {
	f.enqueued = append(f.enqueued, id)
	return nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestReaperRequeuesJobUnderRetryBudget(t *testing.T) {
	id := uuid.New()
	store := &fakeLeaseStore{expired: []ExpiredJob{{ID: id, Attempts: 0, MaxAttempts: 3}}}
	enq := &fakeEnqueuer{}
	r := NewReaper(store, enq, testLogger())

	r.ReapOnce(context.Background())

	if len(store.requeued) != 1 || store.requeued[0] != id {
		t.Errorf("expected job %s to be requeued, got %v", id, store.requeued)
	}
	if len(store.failed) != 0 {
		t.Errorf("expected no failures, got %v", store.failed)
	}
	if len(enq.enqueued) != 1 || enq.enqueued[0] != id {
		t.Errorf("expected job %s to be re-enqueued, got %v", id, enq.enqueued)
	}
}

func TestReaperFailsJobAtRetryBudget(t *testing.T) {
	id := uuid.New()
	store := &fakeLeaseStore{expired: []ExpiredJob{{ID: id, Attempts: 2, MaxAttempts: 3}}}
	enq := &fakeEnqueuer{}
	r := NewReaper(store, enq, testLogger())

	r.ReapOnce(context.Background())

	if len(store.failed) != 1 || store.failed[0] != id {
		t.Errorf("expected job %s to be failed, got %v", id, store.failed)
	}
	if len(store.requeued) != 0 {
		t.Errorf("expected no requeues, got %v", store.requeued)
	}
	if len(enq.enqueued) != 0 {
		t.Errorf("expected no re-enqueue for a failed job, got %v", enq.enqueued)
	}
}

func TestReaperContinuesAfterListError(t *testing.T) {
	store := &fakeLeaseStore{listErr: errors.New("db down")}
	r := NewReaper(store, &fakeEnqueuer{}, testLogger())

	r.ReapOnce(context.Background()) // must not panic
}

func TestReaperHandlesMultipleJobs(t *testing.T) {
	requeueID := uuid.New()
	failID := uuid.New()
	store := &fakeLeaseStore{expired: []ExpiredJob{
		{ID: requeueID, Attempts: 0, MaxAttempts: 3},
		{ID: failID, Attempts: 3, MaxAttempts: 3},
	}}
	r := NewReaper(store, &fakeEnqueuer{}, testLogger())

	r.ReapOnce(context.Background())

	if len(store.requeued) != 1 || store.requeued[0] != requeueID {
		t.Errorf("requeued = %v, want [%s]", store.requeued, requeueID)
	}
	if len(store.failed) != 1 || store.failed[0] != failID {
		t.Errorf("failed = %v, want [%s]", store.failed, failID)
	}
}
