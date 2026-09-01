# 5. Presigned uploads, not proxying the file through the API

## Status
Accepted

## Context
A user needs to get an audio file (up to 200MB, per the plan's upload
constraints) from their browser into MinIO. Two ways to do that: the
browser `PUT`s straight to MinIO with a time-limited signed URL the API
hands it (`POST /uploads` → `PresignPutURL`), or the browser uploads to
the API and the API relays the bytes into MinIO itself.

## Decision
Presigned uploads. `POST /uploads` returns a signed `PUT` URL and an
`object_key`; the browser uploads directly to MinIO; only then does
`POST /jobs` tell the API the upload happened.

Proxying would mean every upload's full body streams through the API
process — 200MB held in flight, twice (once received from the browser,
once forwarded to MinIO) — for a request the API does nothing with
except relay. That's memory and bandwidth spent on a control-plane
process for work that has nothing to do with the "state, delivery
guarantees, API contract" role docs/architecture.md describes for it.
Presigning keeps the API's job exactly what it already is: deciding
*whether* an upload should happen (auth, size/content-type validation in
`internal/http.UploadHandler`) and *where* — never carrying the bytes.

The real cost this decision took on: presigned URLs are only useful if
whoever holds one can actually reach the signing target, and the API's
own view of MinIO (`S3_ENDPOINT`, cluster-internal DNS in every
Kubernetes environment) is not the same address a browser can reach.
That gap surfaced directly in phase 4 — uploads failed with 500 until
`S3_PUBLIC_ENDPOINT` and a second presigning-only `minio.Client` were
added (`internal/objectstore.Client`, docs/architecture.md's job
walkthrough, step 1). Proxying would never have hit that problem, since
the API would be the only party ever addressing MinIO directly. That's
the trade this decision made: an extra configuration axis (internal vs.
public endpoint) in exchange for the API never carrying upload bodies.

## Consequences
- Upload bandwidth and memory pressure scale with MinIO, not with the
  API — the API stays cheap to run and easy to scale on request
  latency alone, unaffected by how large or how many files are in
  flight.
- Every environment needs a real answer to "what address can a browser
  reach MinIO at," not just "what address can the API reach it at."
  Local docker-compose gets this for free (both are `localhost:9000`);
  Kubernetes needed the Ingress path in `deploy/k8s/base/ingress.yaml`
  and `S3_PUBLIC_ENDPOINT` specifically to make it true there too.
- The upload constraints the plan specifies (200MB max, an accepted
  content-type allowlist) are enforced by the API before it presigns —
  `internal/http.UploadHandler` validates before MinIO ever sees a
  request — but MinIO itself has no enforcement of those limits on the
  presigned URL. A client that fabricates its own PUT to a leaked
  presigned URL, or a well-behaved client that lies about its own file
  size in the presign request, isn't stopped by anything downstream of
  that first check. Acceptable for a single-user-account portfolio
  scope; a real multi-tenant deployment would want MinIO-side policy
  enforcement too.
