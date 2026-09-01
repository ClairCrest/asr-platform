import pytest

from worker import config

REQUIRED_ENV = {
    "DATABASE_URL": "postgres://asr:asr@localhost:5432/asr?sslmode=disable",
    "REDIS_ADDR": "localhost:6379",
    "S3_ENDPOINT": "localhost:9000",
    "S3_ACCESS_KEY": "minioadmin",
    "S3_SECRET_KEY": "minioadmin",
    "S3_BUCKET": "asr-audio",
    "S3_USE_SSL": "false",
    "WORKER_MODEL": "small.en",
    "WORKER_CONCURRENCY": "1",
}


def test_load_succeeds_with_all_vars(monkeypatch: pytest.MonkeyPatch) -> None:
    for key, value in REQUIRED_ENV.items():
        monkeypatch.setenv(key, value)

    cfg = config.load()

    assert cfg.database_url == REQUIRED_ENV["DATABASE_URL"]
    assert cfg.s3_use_ssl is False
    assert cfg.concurrency == 1


@pytest.mark.parametrize("missing_key", list(REQUIRED_ENV))
def test_load_fails_fast_on_missing_var(monkeypatch: pytest.MonkeyPatch, missing_key: str) -> None:
    for key, value in REQUIRED_ENV.items():
        if key != missing_key:
            monkeypatch.setenv(key, value)
    monkeypatch.delenv(missing_key, raising=False)

    with pytest.raises(config.ConfigError, match=missing_key):
        config.load()


def test_load_rejects_invalid_bool(monkeypatch: pytest.MonkeyPatch) -> None:
    for key, value in REQUIRED_ENV.items():
        monkeypatch.setenv(key, value)
    monkeypatch.setenv("S3_USE_SSL", "not-a-bool")

    with pytest.raises(config.ConfigError, match="S3_USE_SSL"):
        config.load()


def test_load_rejects_invalid_int(monkeypatch: pytest.MonkeyPatch) -> None:
    for key, value in REQUIRED_ENV.items():
        monkeypatch.setenv(key, value)
    monkeypatch.setenv("WORKER_CONCURRENCY", "not-a-number")

    with pytest.raises(config.ConfigError, match="WORKER_CONCURRENCY"):
        config.load()
