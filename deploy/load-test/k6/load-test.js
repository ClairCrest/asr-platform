// Phase 5 load test: 50 concurrent job submissions, recording submission
// latency (p95) and total queue drain time — how long from "the first
// job was submitted" to "the last job reached a terminal state".
//
// Run: k6 run deploy/load-test/k6/load-test.js
// Against a different target: k6 run -e API_URL=http://localhost:8080 ...
//
// One user, registered once in setup(): the realistic burst this test
// models is one account submitting a batch of jobs at once (the actual
// shape of "a burst of 50 jobs" in the phase 5 acceptance criteria), not
// 50 accounts registering simultaneously. That distinction matters
// operationally, not just semantically — argon2id's password hashing
// (internal/auth's HashPassword, memory=64MiB per call) is deliberately
// memory-hard, so 50 *concurrent registrations* would demand tens of
// MiB each all at once. The first version of this script did register
// per VU and OOM-killed the api pod (exit 137) well before a single job
// reached the queue, which was never what this test was meant to probe.
//
// Drain time is measured by having every VU poll its own job through to
// a terminal state and report a completion timestamp, rather than
// scraping :9090/metrics directly — the metrics port isn't exposed
// through the Ingress (see deploy/k8s/base/ingress.yaml), only
// GET /api/v1/jobs/{id} is, and reachable the same way a real client
// would reach it.
import http from 'k6/http'
import { check, sleep } from 'k6'
import { Trend } from 'k6/metrics'

const API_URL = __ENV.API_URL || 'http://localhost:8080'
const VUS = 50
const sampleAudio = open('./sample.wav', 'b')

export const options = {
  scenarios: {
    burst: {
      executor: 'per-vu-iterations',
      vus: VUS,
      iterations: 1,
      maxDuration: '3m',
    },
  },
  thresholds: {
    // The numbers that actually matter for this load test — see
    // handleSummary. No hard pass/fail gate: a single local kind
    // cluster's baseline varies too much by host machine for a fixed
    // threshold to mean anything here.
    submission_duration: ['p(95)<30000'],
  },
}

const submissionDuration = new Trend('submission_duration', true)
const submissionStartMs = new Trend('submission_start_epoch_ms', false)
const jobCompletedMs = new Trend('job_completed_epoch_ms', false)

export function setup() {
  const email = `loadtest-${Date.now()}@example.com`
  const res = http.post(
    `${API_URL}/api/v1/auth/register`,
    JSON.stringify({ email, password: 'hunter2hunter2' }),
    { headers: { 'Content-Type': 'application/json' } },
  )
  check(res, { 'register succeeded': (r) => r.status === 201 })
  return { token: res.json('access_token') }
}

function waitForTerminal(jobId, authHeaders) {
  for (let i = 0; i < 240; i++) {
    const res = http.get(`${API_URL}/api/v1/jobs/${jobId}`, authHeaders)
    const status = res.json('status')
    if (status === 'succeeded' || status === 'failed' || status === 'cancelled') {
      jobCompletedMs.add(Date.now())
      return
    }
    sleep(1)
  }
  console.warn(`job ${jobId} did not reach a terminal state within the poll window`)
}

export default function (data) {
  const start = Date.now()
  submissionStartMs.add(start)

  const authHeaders = { headers: { Authorization: `Bearer ${data.token}`, 'Content-Type': 'application/json' } }

  const uploadRes = http.post(
    `${API_URL}/api/v1/uploads`,
    JSON.stringify({ filename: 'sample.wav', size_bytes: sampleAudio.byteLength, content_type: 'audio/wav' }),
    authHeaders,
  )
  check(uploadRes, { 'presign succeeded': (r) => r.status === 200 })
  const { upload_url: uploadUrl, object_key: objectKey } = uploadRes.json()

  const putRes = http.put(uploadUrl, sampleAudio, { headers: { 'Content-Type': 'audio/wav' } })
  check(putRes, { 'upload succeeded': (r) => r.status === 200 })

  const jobRes = http.post(
    `${API_URL}/api/v1/jobs`,
    JSON.stringify({
      object_key: objectKey,
      original_filename: 'sample.wav',
      size_bytes: sampleAudio.byteLength,
    }),
    authHeaders,
  )
  check(jobRes, { 'job created': (r) => r.status === 201 })
  submissionDuration.add(Date.now() - start)

  waitForTerminal(jobRes.json('id'), authHeaders)
}

export function handleSummary(data) {
  const p95 = data.metrics.submission_duration?.values['p(95)']
  const earliestStart = data.metrics.submission_start_epoch_ms?.values.min
  const latestCompletion = data.metrics.job_completed_epoch_ms?.values.max
  const drainSeconds =
    earliestStart && latestCompletion ? (latestCompletion - earliestStart) / 1000 : null

  console.log(
    `\nsubmission p95: ${p95 ? p95.toFixed(0) : 'n/a'}ms across ${VUS} concurrent submissions` +
      `\nqueue drain time: ${drainSeconds ? drainSeconds.toFixed(1) : 'n/a'}s ` +
      `(first submission to last job reaching a terminal state)\n`,
  )
  return { stdout: JSON.stringify(data, null, 2) }
}
