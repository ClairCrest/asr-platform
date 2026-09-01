from worker import store


class FakeCursor:
    def __init__(self, fetch_result=None):
        self.executed: list[tuple[str, dict]] = []
        self._fetch_result = fetch_result

    def __enter__(self) -> "FakeCursor":
        return self

    def __exit__(self, *exc_info: object) -> None:
        return None

    def execute(self, sql: str, params: dict | None = None) -> None:
        self.executed.append((sql, params or {}))

    def executemany(self, sql: str, seq_of_params: list[dict]) -> None:
        for params in seq_of_params:
            self.executed.append((sql, params))

    def fetchone(self):
        return self._fetch_result


class FakeConnection:
    def __init__(self, fetch_result=None):
        self.fetch_result = fetch_result
        self.cursors: list[FakeCursor] = []

    def cursor(self, row_factory=None) -> FakeCursor:
        cur = FakeCursor(self.fetch_result)
        self.cursors.append(cur)
        return cur

    def all_executed(self) -> list[tuple[str, dict]]:
        return [call for cur in self.cursors for call in cur.executed]


def test_claim_next_returns_none_when_no_row_matches() -> None:
    conn = FakeConnection(fetch_result=None)

    result = store.claim_next(conn, "job-1", "worker-1")

    assert result is None
    # No event should be written for a job that could not be claimed.
    assert not any("job_events" in sql for sql, _ in conn.all_executed())


def test_claim_next_returns_job_and_writes_leased_event() -> None:
    row = {
        "id": "job-1",
        "object_key": "audio/1.wav",
        "original_filename": "1.wav",
        "model": "small.en",
        "attempts": 0,
        "max_attempts": 3,
    }
    conn = FakeConnection(fetch_result=row)

    claimed = store.claim_next(conn, "job-1", "worker-1")

    assert claimed is not None
    assert claimed.id == "job-1"
    assert claimed.object_key == "audio/1.wav"
    events = [p for sql, p in conn.all_executed() if "job_events" in sql]
    assert len(events) == 1
    assert events[0]["type"] == "leased"


def test_renew_lease_only_touches_lease_expires_at() -> None:
    conn = FakeConnection()

    store.renew_lease(conn, "job-1")

    executed = conn.all_executed()
    assert len(executed) == 1
    sql, params = executed[0]
    assert "lease_expires_at" in sql
    assert "status = 'processing'" in sql
    assert params["id"] == "job-1"
    assert params["lease"] == store.LEASE_DURATION_SECONDS


def test_fail_or_retry_job_requeues_below_max_attempts() -> None:
    conn = FakeConnection()

    status = store.fail_or_retry_job(
        conn,
        "job-1",
        attempts=0,
        max_attempts=3,
        error_code="decode_error",
        error_message="bad file",
    )

    assert status == "queued"
    job_updates = [(sql, p) for sql, p in conn.all_executed() if "UPDATE jobs" in sql]
    assert len(job_updates) == 1
    assert "'queued'" in job_updates[0][0]
    assert job_updates[0][1]["attempts"] == 1

    event_types = [p["type"] for sql, p in conn.all_executed() if "job_events" in sql]
    assert event_types == ["retrying", "queued"]


def test_fail_or_retry_job_fails_at_max_attempts() -> None:
    conn = FakeConnection()

    status = store.fail_or_retry_job(
        conn, "job-1", attempts=2, max_attempts=3, error_code="model_crash", error_message="boom"
    )

    assert status == "failed"
    job_updates = [(sql, p) for sql, p in conn.all_executed() if "UPDATE jobs" in sql]
    assert len(job_updates) == 1
    assert "'failed'" in job_updates[0][0]
    assert job_updates[0][1]["attempts"] == 3

    event_types = [p["type"] for sql, p in conn.all_executed() if "job_events" in sql]
    assert event_types == ["failed"]


def test_complete_job_writes_succeeded_status_and_event() -> None:
    conn = FakeConnection()

    store.complete_job(conn, "job-1", duration_seconds=42.5)

    job_updates = [(sql, p) for sql, p in conn.all_executed() if "UPDATE jobs" in sql]
    assert job_updates[0][1]["duration"] == 42.5
    event_types = [p["type"] for sql, p in conn.all_executed() if "job_events" in sql]
    assert event_types == ["succeeded"]


def test_insert_segments_is_a_noop_for_empty_list() -> None:
    conn = FakeConnection()

    store.insert_segments(conn, "transcript-1", [])

    assert conn.all_executed() == []
