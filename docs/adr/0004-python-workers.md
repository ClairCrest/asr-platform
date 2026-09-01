# 4. Python workers, not another Go process

## Status
Accepted

## Context
The API is Go. The worker could have been Go too — one language across
the whole backend, one build toolchain, one dependency ecosystem to
reason about. Instead the worker is Python.

## Decision
The worker is Python because the model is: `faster-whisper` (a
CTranslate2-based reimplementation of OpenAI's Whisper) is a Python
package with no maintained Go binding, and CTranslate2 itself is a C++
inference runtime that Python wraps directly. Writing the worker in Go
would mean either shelling out to a Python subprocess anyway (all the
complexity of a second language, none of the ergonomics) or binding to
CTranslate2's C++ API from Go via cgo — a substantially larger
undertaking than this project's scope justifies, for a runtime dependency
the Python ecosystem already solves well.

This is the flip side of the control-plane/worker split in
docs/architecture.md: because the worker never terminates an HTTP
request and holds no state the API depends on synchronously, the two
sides only ever talk through Postgres and Redis — never a function call,
never a shared in-process type. Nothing about the API's design assumes
its worker is written in any particular language. That's what makes
"the worker is Python for ML-ecosystem reasons, the API is Go for
everything else" a boundary the architecture actually supports, rather
than a language choice bolted onto a design that assumed one runtime
throughout.

## Consequences
- Two toolchains, two dependency managers (`go.mod`/`uv`), two sets of
  CI steps, two Dockerfiles with different base images and hardening
  approaches. Accepted deliberately — see the trade above.
- The worker inherits Python's ecosystem for exactly the problem it's
  solving (`faster-whisper`, `ffmpeg` via subprocess) and nothing more:
  `worker/pyproject.toml` has four runtime dependencies plus their
  transitive closures, not a general-purpose Python service's usual
  sprawl.
- If a future requirement needed the worker to expose its own HTTP
  surface beyond the Prometheus `/metrics` endpoint it already has, or
  to participate in request-scoped tracing with the API, the
  cross-language boundary would make that meaningfully harder than a
  shared-process Go implementation would. No such requirement exists
  today.
