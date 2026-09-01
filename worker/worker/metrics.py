"""Prometheus metrics for the worker. Started once at process startup
(worker.main.run) on its own HTTP server, per the plan's phase 5:
worker_rtf and worker_busy.
"""

from __future__ import annotations

from prometheus_client import Gauge, start_http_server

# The real-time factor of the most recently finished transcription
# (processing_seconds / audio_duration_seconds) — a gauge rather than a
# histogram since a single worker only ever has one "most recent" value,
# and Grafana can still histogram/aggregate across workers at query time.
worker_rtf = Gauge("worker_rtf", "Real-time factor of the most recently completed transcription.")

# 1 while a job is being downloaded/normalized/transcribed, 0 otherwise —
# summed across workers, this is how KEDA's CPU-based HPA fallback (see
# deploy/k8s/overlays/cloud) and any capacity dashboard read utilization.
worker_busy = Gauge("worker_busy", "1 while this worker is processing a job, 0 when idle.")


def start(port: int) -> None:
    start_http_server(port)
