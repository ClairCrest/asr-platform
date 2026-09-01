"""Phase 2 acceptance check: submit a real audio file end to end (register,
presign, upload, create job) and poll until it reaches succeeded/failed,
printing the full job detail including the transcript job_events.

Usage: python scripts/e2e-check.py path/to/audio.wav
Requires the stack up (`make up && make migrate-up`), the API running
(`go run ./cmd/api`), and a worker running (`uv run python -m worker.main`).
"""

import json
import sys
import time
import urllib.request

API = "http://localhost:8080"


def post(path, body=None, token=None, method="POST"):
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(API + path, data=data, headers=headers, method=method)
    with urllib.request.urlopen(req) as resp:
        return json.loads(resp.read())


def put(url, data):
    req = urllib.request.Request(url, data=data, method="PUT")
    with urllib.request.urlopen(req) as resp:
        return resp.status


def main():
    email = f"e2e-{int(time.time())}@example.com"
    reg = post("/api/v1/auth/register", {"email": email, "password": "hunter2hunter2"})
    token = reg["access_token"]

    with open(sys.argv[1], "rb") as f:
        audio_bytes = f.read()

    upload = post(
        "/api/v1/uploads",
        {"filename": "speech.wav", "size_bytes": len(audio_bytes), "content_type": "audio/wav"},
        token=token,
    )
    print("object_key:", upload["object_key"])

    status = put(upload["upload_url"], audio_bytes)
    print("put status:", status)

    job = post(
        "/api/v1/jobs",
        {"object_key": upload["object_key"], "original_filename": "speech.wav"},
        token=token,
    )
    job_id = job["id"]
    print("job_id:", job_id, "initial status:", job["status"])

    for i in range(60):
        detail = post(f"/api/v1/jobs/{job_id}", token=token, method="GET")
        print(f"poll {i}: status={detail['status']}")
        if detail["status"] in ("succeeded", "failed"):
            print(json.dumps(detail, indent=2))
            break
        time.sleep(3)


if __name__ == "__main__":
    main()
