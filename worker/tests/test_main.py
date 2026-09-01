import time

from worker.main import LeaseRenewer


class FakeConn:
    pass


def test_lease_renewer_calls_renew_lease_while_active(monkeypatch) -> None:
    calls: list[str] = []
    monkeypatch.setattr("worker.main.renew_lease", lambda conn, job_id: calls.append(job_id))
    monkeypatch.setattr("worker.main.LEASE_RENEW_INTERVAL_SECONDS", 0.05)

    with LeaseRenewer(FakeConn(), "job-1"):
        time.sleep(0.2)

    assert len(calls) >= 2
    assert all(c == "job-1" for c in calls)


def test_lease_renewer_stops_after_context_exit(monkeypatch) -> None:
    calls: list[str] = []
    monkeypatch.setattr("worker.main.renew_lease", lambda conn, job_id: calls.append(job_id))
    monkeypatch.setattr("worker.main.LEASE_RENEW_INTERVAL_SECONDS", 0.05)

    with LeaseRenewer(FakeConn(), "job-1"):
        time.sleep(0.1)
    count_at_exit = len(calls)
    time.sleep(0.2)

    assert len(calls) == count_at_exit


def test_lease_renewer_swallows_renew_errors(monkeypatch) -> None:
    def boom(conn, job_id):
        raise RuntimeError("db down")

    monkeypatch.setattr("worker.main.renew_lease", boom)
    monkeypatch.setattr("worker.main.LEASE_RENEW_INTERVAL_SECONDS", 0.05)

    with LeaseRenewer(FakeConn(), "job-1"):
        time.sleep(0.15)
    # No exception should propagate from the background thread.
