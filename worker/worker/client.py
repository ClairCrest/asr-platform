"""Connection factories and thin transport helpers for Redis, MinIO, and
Postgres. Query logic beyond a bare connection lives in worker.store.
"""

from __future__ import annotations

import logging

import psycopg
import redis
from minio import Minio

from worker.config import Config
from worker.errors import DownloadError

logger = logging.getLogger("asr-worker.client")

STREAM_NAME = "jobs:pending"
CONSUMER_GROUP = "workers"


def connect_db(cfg: Config) -> psycopg.Connection:
    conn = psycopg.connect(cfg.database_url, autocommit=True)
    return conn


def connect_redis(cfg: Config) -> redis.Redis:
    client = redis.Redis.from_url(f"redis://{cfg.redis_addr}", decode_responses=True)
    client.ping()
    try:
        client.xgroup_create(STREAM_NAME, CONSUMER_GROUP, id="0", mkstream=True)
    except redis.ResponseError as exc:
        if "BUSYGROUP" not in str(exc):
            raise
    return client


def connect_minio(cfg: Config) -> Minio:
    return Minio(
        cfg.s3_endpoint,
        access_key=cfg.s3_access_key,
        secret_key=cfg.s3_secret_key,
        secure=cfg.s3_use_ssl,
    )


def download_object(mc: Minio, bucket: str, object_key: str, dest_path: str) -> None:
    try:
        mc.fget_object(bucket, object_key, dest_path)
    except Exception as exc:
        raise DownloadError(f"could not download {object_key} from {bucket}: {exc}") from exc
