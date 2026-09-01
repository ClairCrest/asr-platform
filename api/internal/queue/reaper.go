package queue

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// ExpiredJob is the minimal view of a processing job the reaper needs to
// decide whether to requeue or fail it.
type ExpiredJob struct {
	ID          uuid.UUID
	Attempts    int32
	MaxAttempts int32
}

// LeaseStore is the persistence boundary the reaper depends on. Every
// method here operates without a user_id, unlike internal/job.Store: the
// reaper acts on behalf of the system, not a user, since a worker dying
// mid-job is not something any user requested.
type LeaseStore interface {
	ListExpiredLeases(ctx context.Context) ([]ExpiredJob, error)
	RequeueExpiredJob(ctx context.Context, id uuid.UUID) error
	FailExpiredJob(ctx context.Context, id uuid.UUID, errorCode, errorMessage string) error
}

const (
	leaseExpiredErrorCode    = "lease_expired"
	leaseExpiredErrorMessage = "worker did not renew its lease before it expired"
)

// Enqueuer is satisfied by *Producer; declared as an interface so the
// reaper can be tested without a real Redis connection.
type Enqueuer interface {
	Enqueue(ctx context.Context, jobID uuid.UUID) error
}

// Reaper periodically reclaims jobs left processing past their lease —
// almost always because the worker holding them was killed. A job under
// its retry budget is requeued for redelivery; a job that has exhausted
// its attempts is terminated as failed.
type Reaper struct {
	store    LeaseStore
	producer Enqueuer
	logger   *slog.Logger
}

func NewReaper(store LeaseStore, producer Enqueuer, logger *slog.Logger) *Reaper {
	return &Reaper{store: store, producer: producer, logger: logger}
}

// Run scans for expired leases every interval until ctx is cancelled.
func (r *Reaper) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.ReapOnce(ctx)
		}
	}
}

// ReapOnce runs a single reap pass, logging (not returning) individual job
// failures so one bad row does not stop the rest of the batch from being
// reclaimed.
func (r *Reaper) ReapOnce(ctx context.Context) {
	expired, err := r.store.ListExpiredLeases(ctx)
	if err != nil {
		r.logger.Error("reaper: list expired leases", "error", err)
		return
	}

	for _, job := range expired {
		if job.Attempts+1 >= job.MaxAttempts {
			if err := r.store.FailExpiredJob(ctx, job.ID, leaseExpiredErrorCode, leaseExpiredErrorMessage); err != nil {
				r.logger.Error("reaper: fail expired job", "job_id", job.ID, "error", err)
				continue
			}
			r.logger.Warn("reaper: job failed after exhausting retries", "job_id", job.ID)
			continue
		}

		if err := r.store.RequeueExpiredJob(ctx, job.ID); err != nil {
			r.logger.Error("reaper: requeue expired job", "job_id", job.ID, "error", err)
			continue
		}
		if err := r.producer.Enqueue(ctx, job.ID); err != nil {
			r.logger.Error("reaper: re-enqueue expired job", "job_id", job.ID, "error", err)
			continue
		}
		r.logger.Info("reaper: job requeued after lease expiry", "job_id", job.ID)
	}
}
