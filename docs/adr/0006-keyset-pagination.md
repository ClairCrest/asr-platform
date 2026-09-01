# 6. Keyset pagination for GET /jobs, not offset/page numbers

## Status
Accepted

## Context
`GET /jobs` needs to page through a user's job history, newest first.
The two standard approaches: offset-based (`?page=3` or
`?offset=40&limit=20`, backed by SQL `OFFSET`/`LIMIT`) or keyset-based
(`?cursor=<opaque token>`, backed by `WHERE created_at < :cursor ORDER
BY created_at DESC LIMIT :n`).

## Decision
Keyset pagination. `internal/job.Service.List` takes a `cursor
*time.Time` and queries `WHERE created_at < cursor`;
`internal/http.JobHandler.List` encodes/decodes that cursor as a
base64'd RFC3339 timestamp rather than exposing a raw offset or the
timestamp itself as a plain query parameter.

Two concrete problems offset pagination has, both real for this
specific table: `jobs(user_id, created_at DESC)` has an index, but
`OFFSET 10000` still means Postgres walks and discards 10,000 rows
before returning the 20 the client asked for — the query gets slower
the deeper a user pages, with no way to fix that other than changing the
pagination strategy. And because jobs keep getting created while a user
is paging (a dashboard polling or refreshing while new uploads land),
offset pagination silently skips or duplicates rows across pages: page 2
computed as `OFFSET 20` after 5 new jobs landed is not "the next 20
after what the user already saw," it's "the next 20 after a position
that's now 5 rows off from where the user actually was." A keyset
cursor doesn't have either failure mode — `WHERE created_at < cursor`
means "everything strictly older than the last row I showed you," which
stays correct regardless of what's been inserted since, and costs the
same index seek at row 10 or row 10,000.

## Consequences
- No "jump to page 7" — a keyset cursor only supports "give me the next
  page after this one." Nothing in this API's actual usage (a dashboard
  showing recent jobs, scrolling forward in time) needs arbitrary page
  jumps; if that ever became a requirement, it would need a different
  (or additional) pagination strategy, not a small change to this one.
- The cursor is an opaque token the client must pass back verbatim, not
  a page number it can compute itself — slightly more ceremony for a
  hand-written API client, offset by TanStack Query on the dashboard
  side treating `next_cursor` as exactly the kind of opaque pagination
  token it already knows how to manage.
- Sorting is fixed to `created_at DESC` for cursor stability; a future
  "sort by duration" or "sort by status" would need its own keyset
  column (or a compound cursor), not a drop-in `ORDER BY` change the way
  offset pagination would allow.
