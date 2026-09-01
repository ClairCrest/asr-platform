# ASR Platform

Self-hosted, English-only speech-to-text transcription service. Upload
an audio file, watch it queue and transcribe, read the finished
transcript with clickable timestamps — no third-party API in the
critical path.

A Go control plane owns state, delivery guarantees, and the API
contract. Stateless Python workers run `faster-whisper` and can die at
any moment without losing a job. A React dashboard shows job status
updating live over WebSocket, with no page refresh. The whole thing
autoscales on queue depth and ships with real Prometheus metrics and a
Grafana dashboard, not just health checks.

![Upload an audio file, watch it queue and transcribe with the status updating live over WebSocket, then read the finished transcript with an audio player and clickable segment timestamps — no page refresh anywhere in this recording](docs/images/demo.gif)

## Architecture

![Architecture diagram — see docs/architecture.md for the full explanation](docs/images/architecture.svg)

The Go `api` never loads a transcription model; the Python `worker`
never terminates an HTTP request. Full reasoning, the queue's delivery
guarantees, and how the dashboard learns about worker-driven status
changes without the worker ever calling the API: **[docs/architecture.md](docs/architecture.md)**.
Individual design decisions are in **[docs/adr/](docs/adr/)**.

## Quickstart

```bash
git clone https://github.com/ClairCrest/asr-platform.git && cd asr-platform
cp .env.example .env
make kind-up
```

That's a full Kubernetes deployment — Postgres, Redis, MinIO, the API,
workers, the dashboard, KEDA autoscaling, Prometheus, and Grafana — cold
to running at **http://localhost:8080** on a local [kind](https://kind.sigs.k8s.io/)
cluster. `make kind-down` tears it down. `make grafana` / `make
prometheus` port-forward the observability stack.

For a lighter loop without Kubernetes:

```bash
cp .env.example .env
make up && make migrate-up      # Postgres, Redis, MinIO via Docker Compose
cd api && go run ./cmd/api &    # in one terminal
cd worker && uv run python -m worker.main &   # in another
cd web && npm install && npm run dev          # in a third
```

Requires Docker, and on the `make kind-up` path: [`kind`](https://kind.sigs.k8s.io/),
`kubectl`, and the [`golang-migrate`](https://github.com/golang-migrate/migrate)
CLI (only for the Docker Compose path's `make migrate-up`/`make
migrate-down` — `kind-up` runs migrations itself, as a Kubernetes init
container).

## Tech stack

| Layer | Technology |
|---|---|
| API | Go 1.25, chi/v5, pgx/v5, golang-migrate |
| Worker | Python 3.11+, faster-whisper (`small.en`, int8), ffmpeg, uv |
| Frontend | TypeScript, React 19, Vite, TanStack Query, react-router, Tailwind |
| Database | PostgreSQL 16 |
| Queue | Redis Streams with consumer groups |
| Object storage | MinIO (S3-compatible) |
| Realtime | WebSocket, fed by Postgres `LISTEN`/`NOTIFY` |
| Local orchestration | Docker Compose |
| Cluster | Kubernetes via kind, Kustomize (base + local/cloud overlays) |
| Autoscaling | KEDA on Redis Stream backlog, CPU-based HPA fallback |
| Observability | `log/slog` JSON, Prometheus, Grafana, k6 load testing |
| Quality | golangci-lint, ruff, oxlint, pytest, go test |

`internal/store/db` is hand-written to match what `sqlc` would generate
rather than actually generated — the dev sandbox this was built in has
no working `cgo` toolchain, which `sqlc` needs. `sqlc.yaml` and
`internal/store/queries/*.sql` are still the intended source of truth;
see [ADR 0001](docs/adr/0001-hand-written-store-layer.md).

## What this is not

- No model fine-tuning — inference only, on a pretrained English model.
- No Thai or multilingual support. English audio only, and the
  dashboard says so with a visible warning when a transcript's detected
  language confidence drops below 50%, rather than silently returning a
  transcript that might be wrong.
- No speaker diarization, streaming transcription, billing, or
  multi-tenant orgs in v1 — see [`ASR-PLATFORM-PLAN.md`](./ASR-PLATFORM-PLAN.md#9-future-work-to-list-in-the-readme-not-to-build)
  for the full list of what's deliberately out of scope.

## Verified, not just built

Every phase of this project was checked against a real running system,
not just "the code compiles" — and where that surfaced a real bug, the
bug and the fix are in the corresponding commit and, for the
architecture-level ones, in `docs/adr/`. A few examples:

- A 50-job burst against a real `kind` cluster scaled the worker
  Deployment 1→8→1 via KEDA, drained the queue in 76.3s, and held a
  submission p95 of 820ms — see [`docs/benchmarks.md`](docs/benchmarks.md)
  and the Grafana panel it links.
- Force-killing a worker pod mid-transcription, twice — once locally in
  phase 2, once against Kubernetes in phase 4 — and confirming the
  reaper requeued the job and a replacement worker finished it, with the
  correct transcript intact.
- A real headless-browser run of the full register → upload →
  transcribe → view-transcript flow, with zero console errors, is what
  the GIF above actually is — not a mockup.

## Repository layout

```
api/      Go control plane (auth, jobs, queue, WebSocket hub, metrics)
worker/   Python transcription worker (faster-whisper)
web/      Vite + React + TypeScript dashboard
deploy/   Kubernetes manifests (Kustomize) + k6 load test
docs/     architecture notes, ADRs, benchmarks
```

## License

MIT — see [`LICENSE`](./LICENSE).
