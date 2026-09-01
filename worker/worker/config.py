"""Environment-based configuration for the worker. Fails fast at import
time via load() rather than letting the process start half-configured.
"""

from __future__ import annotations

import os
from dataclasses import dataclass


class ConfigError(Exception):
    """Raised when a required environment variable is missing or invalid."""


@dataclass(frozen=True)
class Config:
    database_url: str

    redis_addr: str

    s3_endpoint: str
    s3_access_key: str
    s3_secret_key: str
    s3_bucket: str
    s3_use_ssl: bool

    model: str
    concurrency: int
    metrics_port: int


def load() -> Config:
    return Config(
        database_url=_require("DATABASE_URL"),
        redis_addr=_require("REDIS_ADDR"),
        s3_endpoint=_require("S3_ENDPOINT"),
        s3_access_key=_require("S3_ACCESS_KEY"),
        s3_secret_key=_require("S3_SECRET_KEY"),
        s3_bucket=_require("S3_BUCKET"),
        s3_use_ssl=_require_bool("S3_USE_SSL"),
        model=_require("WORKER_MODEL"),
        concurrency=_require_int("WORKER_CONCURRENCY"),
        metrics_port=_require_int("WORKER_METRICS_PORT"),
    )


def _require(key: str) -> str:
    value = os.environ.get(key)
    if not value:
        raise ConfigError(f"missing required env var {key}")
    return value


def _require_bool(key: str) -> bool:
    value = _require(key)
    if value.lower() not in ("true", "false"):
        raise ConfigError(f"env var {key} must be 'true' or 'false', got {value!r}")
    return value.lower() == "true"


def _require_int(key: str) -> int:
    value = _require(key)
    try:
        return int(value)
    except ValueError as exc:
        raise ConfigError(f"env var {key} must be an integer, got {value!r}") from exc
