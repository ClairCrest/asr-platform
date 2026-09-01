# ASR Platform

Self-hosted, English-only speech-to-text transcription service. Upload an audio file, watch it
queue and transcribe, read the finished transcript with timestamps — no third-party API in the
critical path.

Go control plane (state, delivery guarantees, API contract) + Python inference workers
(stateless, disposable, run `faster-whisper`) + a TypeScript/React dashboard with live job
updates over WebSocket. See [`ASR-PLATFORM-PLAN.md`](./ASR-PLATFORM-PLAN.md) for the full design
and phase-by-phase build plan.

> Status: Phase 0 (skeleton) in progress. The sections below describe the target shape of the
> project; not everything here is built yet.

## Tech stack

| Layer | Technology |
|---|---|
| API | Go 1.23+, chi/v5, pgx/v5 + sqlc, golang-migrate |
| Worker | Python 3.11+, faster-whisper (`small.en`, int8), ffmpeg |
| Frontend | TypeScript, React 18, Vite, TanStack Query, Tailwind |
| Database | PostgreSQL 16 |
| Queue | Redis Streams with consumer groups |
| Object storage | MinIO (S3-compatible) |
| Realtime | WebSocket |
| Orchestration | Docker Compose locally, Kubernetes (kind + Kustomize) for the real deployment |

## What this is not

- No model fine-tuning — inference only, on a pretrained English model.
- No Thai or multilingual support. English audio only, and the system says so when it isn't
  confident the input is English.
- No speaker diarization, billing, or multi-tenant orgs in v1.

## Quickstart (local dev)

```bash
cp .env.example .env
make up            # Postgres, Redis, MinIO via Docker Compose
make migrate-up     # apply SQL migrations
```

Requires Docker, and the [`golang-migrate`](https://github.com/golang-migrate/migrate) CLI on
your `PATH` for `make migrate-up`/`make migrate-down`.

## Repository layout

```
api/      Go control plane (auth, jobs, queue, WebSocket hub)
worker/   Python transcription worker (faster-whisper)
web/      Vite + React + TypeScript dashboard
deploy/   Kubernetes manifests (Kustomize)
docs/     architecture notes, ADRs, benchmarks
```

## License

MIT — see [`LICENSE`](./LICENSE).
