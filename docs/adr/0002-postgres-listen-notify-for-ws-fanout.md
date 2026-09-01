# 2. Postgres LISTEN/NOTIFY to fan job_events out to the WebSocket hub

## Status
Accepted

## Context
The dashboard's WebSocket feed (section 4, `GET /ws`) needs to learn about
every job status change and push it to the owning user's connection. The
build plan does not say how the API should learn about a change, and that
matters here specifically because not every change is made by the API:
user-initiated actions (cancel, retry, delete) go through
`internal/job.Service`, but transcription progress (leased, succeeded,
failed, retrying) is written directly to Postgres by the Python worker,
which never talks to the Go API at all. A fanout mechanism driven by
`job.Service` alone would miss every worker-written event.

## Decision
Add a trigger on `job_events` (migration `000008`) that calls
`pg_notify('job_events', ...)` with the job id, owning user id, event
type, payload, and timestamp on every insert, regardless of which process
performed it. The API holds one dedicated `pgxpool` connection running
`LISTEN job_events` and forwards each notification to `internal/ws.Hub`,
which fans it out to that user's open WebSocket connections.

A trigger was chosen over notifying from application code (in both the Go
API and the Python worker) because it can't be skipped by a future write
path that forgets to call it — the guarantee lives in the schema, not in
every caller remembering to uphold it. The dashboard hook treats a
notification as a signal to invalidate/refetch the affected job's React
Query cache entry rather than trying to reconstruct the full row from the
NOTIFY payload, since Postgres caps a NOTIFY payload at 8000 bytes and the
row is already one cheap `GET /jobs/{id}` away.

## Consequences
- Real-time updates work identically whether the change came from the API
  or the worker, with no coordination required between the two codebases.
- The API needs one long-lived Postgres connection outside its normal
  pool for `LISTEN`; if that connection drops it must reconnect and
  re-`LISTEN`, or the WebSocket feed silently stops updating until it
  does. `internal/ws` retries with backoff for this reason.
- `NOTIFY` is fire-and-forget: a notification sent while the API is down
  is lost. This is acceptable because the dashboard's `GET /jobs/{id}`
  and the 5s polling fallback in section 4 both provide eventual
  consistency independent of the WebSocket feed.
