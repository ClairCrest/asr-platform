"""Consumer loop: pull one job at a time from the Redis Stream, transcribe
it, write the result, and ack. The worker is stateless and disposable — it
holds no cross-job state and is expected to survive being killed at any
moment; the Go API's reaper reclaims anything left processing past its
lease.
"""

from __future__ import annotations

import logging
import os
import socket
import tempfile
import threading
import time

import redis

from worker import audio, client, config, metrics
from worker.errors import TranscriptionError
from worker.store import (
    claim_next,
    complete_job,
    fail_or_retry_job,
    insert_segments,
    insert_transcript,
    renew_lease,
)
from worker.transcribe import Transcriber

logger = logging.getLogger("asr-worker.main")

BLOCK_MS = 5000
LEASE_RENEW_INTERVAL_SECONDS = 20


def worker_id() -> str:
    return f"{socket.gethostname()}-{os.getpid()}"


class LeaseRenewer:
    """Renews a job's lease on a background thread while the main thread
    is busy transcribing, so a long file does not get reclaimed by the
    reaper mid-processing.
    """

    def __init__(self, conn, job_id: str) -> None:
        self._conn = conn
        self._job_id = job_id
        self._stop = threading.Event()
        self._thread = threading.Thread(target=self._run, daemon=True)

    def _run(self) -> None:
        while not self._stop.wait(LEASE_RENEW_INTERVAL_SECONDS):
            try:
                renew_lease(self._conn, self._job_id)
            except Exception:
                logger.exception("failed to renew lease for job %s", self._job_id)

    def __enter__(self) -> LeaseRenewer:
        self._thread.start()
        return self

    def __exit__(self, *exc_info: object) -> None:
        self._stop.set()
        self._thread.join(timeout=5)


def process_job(
    conn,
    mc,
    bucket: str,
    rdb: redis.Redis,
    transcriber: Transcriber,
    job_id: str,
    this_worker_id: str,
) -> None:
    claimed = claim_next(conn, job_id, this_worker_id)
    if claimed is None:
        logger.info("job %s no longer claimable, skipping", job_id)
        return

    logger.info("job %s claimed, downloading %s", claimed.id, claimed.object_key)
    metrics.worker_busy.set(1)
    try:
        _transcribe_and_store(conn, mc, bucket, rdb, transcriber, claimed)
    finally:
        metrics.worker_busy.set(0)


def _transcribe_and_store(
    conn, mc, bucket: str, rdb: redis.Redis, transcriber: Transcriber, claimed
) -> None:
    with LeaseRenewer(conn, claimed.id), tempfile.TemporaryDirectory() as tmp_dir:
        src_path = os.path.join(tmp_dir, "source")
        wav_path = os.path.join(tmp_dir, "normalized.wav")
        try:
            client.download_object(mc, bucket, claimed.object_key, src_path)
            info = audio.probe(src_path)
            audio.normalize(src_path, wav_path)
            result = transcriber.transcribe(wav_path, info.duration_seconds)
        except TranscriptionError as exc:
            logger.warning("job %s failed: %s: %s", claimed.id, exc.error_code, exc.message)
            new_status = fail_or_retry_job(
                conn,
                claimed.id,
                claimed.attempts,
                claimed.max_attempts,
                exc.error_code,
                exc.message,
            )
            if new_status == "queued":
                rdb.xadd(client.STREAM_NAME, {"job_id": claimed.id})
            return

    transcript_id = insert_transcript(
        conn,
        claimed.id,
        result.text,
        result.language_detected,
        result.language_probability,
        claimed.model,
        result.processing_seconds,
        result.real_time_factor,
    )
    insert_segments(conn, transcript_id, result.segments)
    complete_job(conn, claimed.id, info.duration_seconds)
    metrics.worker_rtf.set(result.real_time_factor)
    logger.info(
        "job %s succeeded: rtf=%.2f language=%s (%.2f)",
        claimed.id,
        result.real_time_factor,
        result.language_detected,
        result.language_probability,
    )


def run() -> None:
    logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(name)s %(message)s")

    cfg = config.load()
    this_worker_id = worker_id()
    consumer_name = this_worker_id

    logger.info("asr-worker starting, model=%s, worker_id=%s", cfg.model, this_worker_id)
    metrics.start(cfg.metrics_port)
    metrics.worker_busy.set(0)

    conn = client.connect_db(cfg)
    rdb = client.connect_redis(cfg)
    mc = client.connect_minio(cfg)
    transcriber = Transcriber(cfg.model)

    logger.info("asr-worker ready, consuming %s", client.STREAM_NAME)

    while True:
        try:
            messages = rdb.xreadgroup(
                client.CONSUMER_GROUP,
                consumer_name,
                {client.STREAM_NAME: ">"},
                count=1,
                block=BLOCK_MS,
            )
        except redis.RedisError:
            logger.exception("redis read failed, retrying in 5s")
            time.sleep(5)
            continue

        if not messages:
            continue

        for _stream_name, entries in messages:
            for message_id, fields in entries:
                job_id = fields.get("job_id")
                try:
                    if job_id:
                        process_job(
                            conn, mc, cfg.s3_bucket, rdb, transcriber, job_id, this_worker_id
                        )
                except Exception:
                    logger.exception("unhandled error processing job %s", job_id)
                finally:
                    rdb.xack(client.STREAM_NAME, client.CONSUMER_GROUP, message_id)


def main() -> None:
    run()


if __name__ == "__main__":
    main()
