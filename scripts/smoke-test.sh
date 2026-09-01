#!/usr/bin/env bash
# Phase 1 acceptance check: register a user, presign an upload, create a
# job, and confirm it reads back as "queued". Requires the stack to be up
# (`make up && make migrate-up`) and the API running (`go run ./cmd/api`).
set -euo pipefail

API="${API_URL:-http://localhost:8080}"
EMAIL="smoke-$(date +%s)@example.com"
PASSWORD="hunter2hunter2"

echo "== register =="
REGISTER_RESP=$(curl -sf -X POST "$API/api/v1/auth/register" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}")
ACCESS_TOKEN=$(echo "$REGISTER_RESP" | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4)
if [ -z "$ACCESS_TOKEN" ]; then
  echo "register failed: $REGISTER_RESP" >&2
  exit 1
fi

echo "== create upload =="
UPLOAD_RESP=$(curl -sf -X POST "$API/api/v1/uploads" \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"filename":"sample.wav","size_bytes":1024,"content_type":"audio/wav"}')
UPLOAD_URL=$(echo "$UPLOAD_RESP" | grep -o '"upload_url":"[^"]*"' | cut -d'"' -f4 | sed 's/\\u0026/\&/g')
OBJECT_KEY=$(echo "$UPLOAD_RESP" | grep -o '"object_key":"[^"]*"' | cut -d'"' -f4)
if [ -z "$UPLOAD_URL" ] || [ -z "$OBJECT_KEY" ]; then
  echo "upload presign failed: $UPLOAD_RESP" >&2
  exit 1
fi

echo "== put audio bytes to presigned url =="
dd if=/dev/zero of=/tmp/sample.wav bs=1024 count=1 2>/dev/null
curl -sf -X PUT "$UPLOAD_URL" --data-binary @/tmp/sample.wav

echo "== create job =="
JOB_RESP=$(curl -sf -X POST "$API/api/v1/jobs" \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"object_key\":\"$OBJECT_KEY\",\"original_filename\":\"sample.wav\"}")
JOB_ID=$(echo "$JOB_RESP" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4)
JOB_STATUS=$(echo "$JOB_RESP" | grep -o '"status":"[^"]*"' | head -1 | cut -d'"' -f4)
echo "job $JOB_ID created with status $JOB_STATUS"

echo "== read job back =="
READBACK=$(curl -sf "$API/api/v1/jobs/$JOB_ID" -H "Authorization: Bearer $ACCESS_TOKEN")
READBACK_STATUS=$(echo "$READBACK" | grep -o '"status":"[^"]*"' | head -1 | cut -d'"' -f4)

if [ "$READBACK_STATUS" != "queued" ]; then
  echo "FAIL: expected status 'queued', got '$READBACK_STATUS'" >&2
  echo "$READBACK" >&2
  exit 1
fi

echo "PASS: job $JOB_ID is queued"
