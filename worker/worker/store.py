"""Postgres persistence for job processing: claiming work, renewing the
lease, and writing the terminal result. This is the worker-side mirror of
the Go API's internal/job state machine — every transition here writes a
job_events row too, since the WebSocket feed and audit trail both read
from it regardless of which side made the change.
"""

from __future__ import annotations

import json
import logging
import uuid
from dataclasses import dataclass

import psycopg
from psycopg.rows import dict_row

logger = logging.getLogger("asr-worker.store")

LEASE_DURATION_SECONDS = 60


@dataclass(frozen=True)
class ClaimedJob:
    id: str
    object_key: str
    original_filename: str
    model: str
    attempts: int
    max_attempts: int


def claim_next(conn: psycopg.Connection, job_id: str, worker_id: str) -> ClaimedJob | None:
    """Move a queued job to processing and take the lease. Returns None if
    the job is no longer queued (already claimed, cancelled, or deleted by
    the time this worker got to it), which the caller should treat as a
    no-op rather than an error.
    """
    with conn.cursor(row_factory=dict_row) as cur:
        cur.execute(
            """
            UPDATE jobs
            SET status = 'processing',
                worker_id = %(worker_id)s,
                lease_expires_at = now() + make_interval(secs => %(lease)s),
                started_at = COALESCE(started_at, now())
            WHERE id = %(id)s AND status IN ('queued', 'retrying') AND deleted_at IS NULL
            RETURNING id, object_key, original_filename, model, attempts, max_attempts
            """,
            {"worker_id": worker_id, "lease": LEASE_DURATION_SECONDS, "id": job_id},
        )
        row = cur.fetchone()
        if row is None:
            return None

    insert_event(conn, job_id, "leased", {"worker_id": worker_id})
    return ClaimedJob(
        id=str(row["id"]),
        object_key=row["object_key"],
        original_filename=row["original_filename"],
        model=row["model"],
        attempts=row["attempts"],
        max_attempts=row["max_attempts"],
    )


def renew_lease(conn: psycopg.Connection, job_id: str) -> None:
    with conn.cursor() as cur:
        cur.execute(
            """
            UPDATE jobs
            SET lease_expires_at = now() + make_interval(secs => %(lease)s)
            WHERE id = %(id)s AND status = 'processing'
            """,
            {"lease": LEASE_DURATION_SECONDS, "id": job_id},
        )


def complete_job(conn: psycopg.Connection, job_id: str, duration_seconds: float) -> None:
    with conn.cursor() as cur:
        cur.execute(
            """
            UPDATE jobs
            SET status = 'succeeded', duration_seconds = %(duration)s, finished_at = now()
            WHERE id = %(id)s
            """,
            {"duration": duration_seconds, "id": job_id},
        )
    insert_event(conn, job_id, "succeeded", None)


def fail_or_retry_job(
    conn: psycopg.Connection,
    job_id: str,
    attempts: int,
    max_attempts: int,
    error_code: str,
    error_message: str,
) -> str:
    """Increment attempts and either requeue or terminate the job.
    Returns the resulting status ("queued" or "failed").
    """
    new_attempts = attempts + 1
    if new_attempts >= max_attempts:
        with conn.cursor() as cur:
            cur.execute(
                """
                UPDATE jobs
                SET status = 'failed', attempts = %(attempts)s,
                    error_code = %(code)s, error_message = %(message)s,
                    finished_at = now()
                WHERE id = %(id)s
                """,
                {
                    "attempts": new_attempts,
                    "code": error_code,
                    "message": error_message,
                    "id": job_id,
                },
            )
        insert_event(
            conn, job_id, "failed", {"error_code": error_code, "error_message": error_message}
        )
        return "failed"

    with conn.cursor() as cur:
        cur.execute(
            """
            UPDATE jobs
            SET status = 'queued', attempts = %(attempts)s,
                error_code = %(code)s, error_message = %(message)s,
                worker_id = NULL, lease_expires_at = NULL
            WHERE id = %(id)s
            """,
            {"attempts": new_attempts, "code": error_code, "message": error_message, "id": job_id},
        )
    insert_event(
        conn, job_id, "retrying", {"error_code": error_code, "error_message": error_message}
    )
    insert_event(conn, job_id, "queued", None)
    return "queued"


def insert_transcript(
    conn: psycopg.Connection,
    job_id: str,
    text: str,
    language_detected: str,
    language_probability: float,
    model: str,
    processing_seconds: float,
    real_time_factor: float,
) -> str:
    transcript_id = str(uuid.uuid4())
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO transcripts (
                id, job_id, text, language_detected, language_probability,
                model, processing_seconds, real_time_factor
            ) VALUES (
                %(id)s, %(job_id)s, %(text)s, %(lang)s, %(lang_prob)s,
                %(model)s, %(proc)s, %(rtf)s
            )
            """,
            {
                "id": transcript_id,
                "job_id": job_id,
                "text": text,
                "lang": language_detected,
                "lang_prob": language_probability,
                "model": model,
                "proc": processing_seconds,
                "rtf": real_time_factor,
            },
        )
    return transcript_id


def insert_segments(conn: psycopg.Connection, transcript_id: str, segments: list) -> None:
    if not segments:
        return
    with conn.cursor() as cur:
        cur.executemany(
            """
            INSERT INTO segments (id, transcript_id, idx, start_ms, end_ms, text, avg_logprob)
            VALUES (
                %(id)s, %(transcript_id)s, %(idx)s, %(start_ms)s, %(end_ms)s,
                %(text)s, %(avg_logprob)s
            )
            """,
            [
                {
                    "id": str(uuid.uuid4()),
                    "transcript_id": transcript_id,
                    "idx": s.idx,
                    "start_ms": s.start_ms,
                    "end_ms": s.end_ms,
                    "text": s.text,
                    "avg_logprob": s.avg_logprob,
                }
                for s in segments
            ],
        )


def insert_event(
    conn: psycopg.Connection, job_id: str, event_type: str, payload: dict | None
) -> None:
    with conn.cursor() as cur:
        cur.execute(
            """
            INSERT INTO job_events (id, job_id, event_type, payload)
            VALUES (%(id)s, %(job_id)s, %(type)s, %(payload)s)
            """,
            {
                "id": str(uuid.uuid4()),
                "job_id": job_id,
                "type": event_type,
                "payload": json.dumps(payload or {}),
            },
        )
