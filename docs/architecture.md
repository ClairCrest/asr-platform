# Architecture

![Architecture diagram: browser talks to web and api; api owns Postgres, MinIO, and the Redis Streams producer; N worker replicas consume from Redis, write results directly to Postgres and MinIO, and Postgres LISTEN/NOTIFY pushes those changes back through the api to the browser over WebSocket](images/architecture.svg)

## Why the control plane is separate from the worker

The Go `api` never loads a transcription model and the Python `worker`
never terminates an HTTP request. That split is deliberate, not
incidental, and it's the single decision most of the rest of this
document explains the consequences of. (Why the worker is Python
specifically, rather than another Go process, is
[ADR 0004](adr/0004-python-workers.md).)

- **The API owns state and delivery guarantees; the worker owns
  nothing.** Every fact about a job — its status, its attempts, its
  transcript — lives in Postgres, written through `internal/job`'s state
  machine (API-initiated changes) or directly by the worker
  (transcription-driven changes). A worker holds no cross-job state and
  is safe to kill at any point in its lifecycle: mid-download,
  mid-transcription, mid-write. Nothing about the system's correctness
  depends on a worker process surviving.
- **A worker that dies doesn't lose work.** Every job gets a lease
  (`jobs.lease_expires_at`) when a worker claims it, renewed every 20s
  during long files (`worker/main.py`'s `LeaseRenewer`). The API's
  reaper (`internal/queue.Reaper`) scans for jobs whose lease has
  expired without the job reaching a terminal state, and requeues them
  onto the Redis Stream for the next available worker — verified in
  phase 2 and again against a live Kubernetes cluster in phase 4 by
  force-killing a worker mid-job and watching the job complete anyway.
- **Scaling the transcription workload never touches the request path.**
  KEDA scales worker replicas 0→8 (locally 1→8, see
  `deploy/k8s/components/keda-scaling`) on Redis Stream backlog, with no
  coordination required from the API beyond publishing to the stream.
  The API itself scales independently on its own request load, if it
  ever needs to.

## Queue guarantees

Redis Streams with a consumer group (`jobs:pending`, group `workers`) is
the durable handoff between "the API accepted an upload" and "some
worker is responsible for transcribing it." What that buys, precisely:

- **At-least-once delivery.** `XADD` on job creation, `XREADGROUP` to
  claim, `XACK` only after the worker's own terminal write (success or a
  permanent failure) succeeds. A worker that dies between claiming a
  message and acking it leaves that message in the consumer group's
  pending entries list — but redelivery in this system happens through
  the **lease in Postgres**, not through re-claiming the original Redis
  message (see below), so the practical guarantee is "the job gets
  retried," not "the exact same stream entry gets redelivered."
- **The lease, not Redis's PEL, is the source of truth for
  "abandoned."** The reaper doesn't inspect Redis's pending-entries list
  at all — it queries `jobs WHERE status = 'processing' AND
  lease_expires_at < now()`. When it finds one, it increments
  `attempts`, and either moves the job back to `queued` and publishes a
  **new** `XADD` (if attempts remain) or marks it `failed` (if not). The
  original stream entry from the dead worker is simply abandoned; a
  fresh message carries the retry. This was a deliberate simplification
  over `XCLAIM`-based reclaim — see
  [ADR 0003](adr/0003-redis-streams-over-nats.md) for the fuller
  reasoning — and it means Redis's own consumer-group bookkeeping never
  needs to be reconciled with job state; Postgres is the only place that
  has to agree with itself.
- **Ordering is not guaranteed, and nothing in this system needs it to
  be.** Two jobs from the same user can complete out of order depending
  on which worker picks them up and how long each takes. The dashboard
  reflects per-job status independently, so this was never a
  requirement worth designing around.
- **Idempotent job creation.** `POST /jobs` with an `Idempotency-Key`
  header returns the existing job on a retry rather than creating a
  duplicate (`internal/job.Service.Create` checks
  `GetJobByIdempotencyKey` before inserting) — the same guarantee a
  payment API gives you, applied here because upload+create is a
  two-step client operation that can legitimately be retried after a
  network blip between the two steps.

## How the dashboard learns about worker-driven changes

The Python worker writes directly to Postgres — it never calls the Go
API. That's true for `succeeded`, `failed`, `retrying`, and `leased`
events, all written from `worker/store.py`. So when the dashboard needs
to show a job flipping from `processing` to `succeeded` without a page
refresh, the API has no HTTP request to observe; it has to find out some
other way.

The answer is a Postgres trigger (migration `000008`, extended by
`000009`) that calls `pg_notify('job_events', ...)` on every insert into
`job_events`, regardless of which process performed the insert. The API
holds one dedicated `pgxpool` connection running `LISTEN job_events`
(`internal/ws.Listener`) and fans each notification out over WebSocket
to that job's owner (`internal/ws.Hub`), and separately feeds the same
notification stream into the Prometheus counters in `internal/metrics`
that track worker-driven completions. One trigger, two independent
consumers of the same signal, and no possibility of a future write path
forgetting to emit it — the guarantee lives in the schema, not in
application code remembering to uphold it. The full reasoning, including
why a trigger over notifying from application code, is in
[ADR 0002](adr/0002-postgres-listen-notify-for-ws-fanout.md).

## What happens to a job, end to end

1. **`POST /uploads`** — the API asks MinIO to presign a PUT URL. The
   presigning client is deliberately a *different* MinIO client than the
   one the API uses for its own operations (bucket checks, deletes):
   presigned URLs are handed to a browser, which can only reach MinIO
   through whatever's actually exposed to it (`S3_PUBLIC_ENDPOINT`),
   while the API's own MinIO calls go over the cluster-internal address
   (`S3_ENDPOINT`) — the same object store, two different addresses
   depending on who's asking. See
   [ADR 0005](adr/0005-presigned-uploads.md) for why presigned uploads
   at all, rather than proxying the body through the API.
2. **Browser `PUT`s the file directly to MinIO.** The upload body never
   touches the API process.
3. **`POST /jobs`** — a row lands in Postgres as `pending`, a
   `job_events` row records `created`, then (after the DB transition
   commits — enqueueing before that would let a fast worker see a job
   that isn't queued yet, a real race this project hit and fixed) the
   job moves to `queued` and an `XADD` publishes it to the stream.
4. **A worker's `XREADGROUP` claims the message**, atomically flips the
   job to `processing` with a lease, downloads the object from MinIO,
   probes it with `ffprobe`, normalizes to 16kHz mono WAV with `ffmpeg`,
   and transcribes with `faster-whisper`.
5. **The worker writes the transcript, segments, and job status directly
   to Postgres**, acks the Redis message, and the trigger above takes it
   from there.
6. **The dashboard's already-open WebSocket connection gets pushed the
   event** and invalidates the relevant React Query cache entries — the
   status badge and, once ready, the transcript itself update with no
   reload.

## Keyset over offset pagination

`GET /jobs` paginates on a `created_at` cursor, not `?page=N` /
`?offset=N`. See [ADR 0006](adr/0006-keyset-pagination.md) for the
reasoning — the short version is that offset pagination gets slower as
the offset grows and produces skipped-or-duplicated rows when jobs are
created between page requests, and a keyset cursor has neither problem
at the cost of not being able to jump to an arbitrary page number, which
this API's clients never need to do.
