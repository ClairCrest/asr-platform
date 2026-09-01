// Package queue produces job messages onto the Redis Stream that workers
// consume from via a consumer group. This package only ever adds messages;
// the reaper and consumer-group bookkeeping belong to the worker (phase 2).
package queue

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// StreamName is the Redis Stream jobs are published to. Workers consume it
// via the "workers" consumer group.
const StreamName = "jobs:pending"

type Producer struct {
	rdb *redis.Client
}

func NewProducer(rdb *redis.Client) *Producer {
	return &Producer{rdb: rdb}
}

// Enqueue publishes jobID onto the pending stream. Delivery is
// at-least-once: the worker acks after a successful write, and the reaper
// requeues anything left claimed past its lease.
func (p *Producer) Enqueue(ctx context.Context, jobID uuid.UUID) error {
	err := p.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: StreamName,
		Values: map[string]any{"job_id": jobID.String()},
	}).Err()
	if err != nil {
		return fmt.Errorf("queue: enqueue job %s: %w", jobID, err)
	}
	return nil
}
