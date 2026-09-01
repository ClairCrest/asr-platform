# 1. Hand-write the sqlc output instead of running the code generator

## Status
Accepted

## Context
Section 1 of the build plan pins the database access layer to `pgx/v5` +
`sqlc`: hand-written, typed queries generated from `internal/store/queries/*.sql`
against the schema in `internal/store/migrations/`, with no ORM in between.

The `sqlc` CLI needs `cgo`, and `cgo` needs a working C toolchain that can
actually compile and link a binary. On the development machine used to build
phase 1, `gcc` is present and `gcc --version` runs, but invoking it to
produce an object file or executable (`gcc t.c -o t.exe`) fails silently
with a non-zero exit and no stderr output, for both cgo itself and a bare
"hello world" C program. Installing `sqlc` via `go install` therefore never
completes, and `go test -race` (which also needs cgo) fails the same way.

## Decision
Keep `internal/store/queries/*.sql` and `api/sqlc.yaml` as the source of
truth for the query set, exactly as if sqlc generation were working. Hand
write `internal/store/db/` to match the shape sqlc would produce: a
`Queries` struct wrapping a `pgx` connection or pool, one method per named
query, and `Models` structs mirroring the migrated schema with the same
field names and types sqlc's `overrides` in `sqlc.yaml` specify (`uuid.UUID`
for `uuid` columns, `time.Time` for `timestamptz`).

`make sqlc` (`sqlc generate`) stays in the Makefile unchanged. On any
machine with a working C toolchain — CI included — running it should
regenerate `internal/store/db/` to match, or close to match, what is
committed here. If a future query change is made by hand instead of via
`sqlc generate`, keep `queries.sql` and the hand-written Go in sync
manually until generation is available again.

## Consequences
- No dependency on a working local `cgo` toolchain to keep building the API
  in this environment.
- `internal/store/db/` is now a normal, reviewable Go package rather than a
  generated one — worth noting in review so nobody expects the usual
  "don't edit generated files" convention until sqlc is confirmed working.
- `go test -race` cannot be verified locally on this machine; it is expected
  to run in CI, where cgo is unconstrained.
