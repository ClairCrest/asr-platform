# 3. Redis Streams over NATS (or another dedicated broker)

## Status
Accepted

## Context
The control plane needs to hand a job off to a pool of workers with
at-least-once delivery, consumer-group load balancing (each job goes to
exactly one worker, not every worker), and a way to detect a worker that
died mid-job. Redis Streams and NATS JetStream both do this. So would
RabbitMQ, or a hosted queue like SQS.

## Decision
Redis Streams with a consumer group (`jobs:pending`, group `workers`).

The deciding factor was *what's already in the stack*, not a
feature-by-feature win over JetStream. Redis is already a hard
dependency for nothing else in this project — but MinIO and Postgres
are, and once Redis is added as the third piece of local infrastructure
(`docker-compose.yml`, `deploy/k8s/base/redis.yaml`), it's one more
moving part to run, monitor, and explain, not one-of-several. A
dedicated broker like NATS would be a fourth. For a portfolio-scoped
single-tenant service, "the queue is the same technology a cache would
be, and most engineers already know its operational shape" outweighs
JetStream's stronger built-in redelivery semantics.

That trade only holds because of a second decision this project made
deliberately: **the lease lives in Postgres, not in Redis's own
pending-entries list.** `internal/queue.Reaper` never calls `XCLAIM` or
inspects Redis's PEL at all — it scans `jobs WHERE status = 'processing'
AND lease_expires_at < now()` and, on finding one, publishes a **fresh**
`XADD` rather than reclaiming the original stream entry. NATS
JetStream's redelivery/ack-wait mechanism is more sophisticated than
Redis Streams' PEL, but this project doesn't lean on either broker's
native redelivery — Postgres is the single source of truth for "this
job needs to be retried," and the broker's only job is "get a fresh
message to some worker." That design choice is what makes Redis Streams
sufficient here regardless of which broker's redelivery story is
stronger on paper.

## Consequences
- One less piece of infrastructure than adding NATS would be, at the
  cost of hand-rolling the lease/reaper logic that JetStream would give
  closer to free. That trade was made once, in
  `internal/queue.Reaper`/`worker/store.py`, and applies to every job
  from then on — it's not a recurring cost.
- Redis Streams' consumer-group semantics (`XREADGROUP`,
  `entries-read`/`lag` via `XINFO GROUPS`) turned out to be exactly what
  KEDA's `redis-streams` scaler and this project's own
  `job_queue_depth` metric need — an accident of choosing Redis for
  reasons unrelated to autoscaling, not a design factor at the time.
- If a future requirement needed guaranteed message ordering across
  workers, exactly-once semantics, or multi-tenant topic isolation, that
  would be a real reason to revisit this — none of those are
  requirements today (see docs/architecture.md's "Queue guarantees").
