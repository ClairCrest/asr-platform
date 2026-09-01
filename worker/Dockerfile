# syntax=docker/dockerfile:1
FROM python:3.11-slim AS build
WORKDIR /src

COPY --from=ghcr.io/astral-sh/uv:0.12.5 /uv /usr/local/bin/uv

# --no-install-project: only third-party dependencies go into .venv. The
# project's own "worker" package is never installed as a distribution —
# it's run straight from source (see the final stage's explicit COPY),
# so this stage doesn't need pyproject.toml's build-backend to produce
# anything for it, and the final stage doesn't inherit an editable-install
# pointer back to a /src path that won't exist there.
COPY pyproject.toml uv.lock ./
RUN uv sync --frozen --no-dev --no-install-project

COPY worker/ ./worker/

# Bake the model weights into the image at build time — the whole point
# of doing this here is that the running container never touches the
# network for them. HF_HOME pins the cache to a path this stage owns so
# the next stage can copy it forward wholesale.
ENV HF_HOME=/model-cache
# Whatever model is baked in here is the only one this image can ever
# serve without a network call — the WORKER_MODEL env var the container
# reads at runtime (see deploy/k8s ConfigMap) must name this same model,
# or worker.config.load() will point faster-whisper at a model that was
# never cached and HF_HUB_OFFLINE will make it fail outright rather than
# silently downloading it.
ARG WORKER_MODEL=small.en
# Call .venv's python directly rather than `uv run`: uv run re-syncs the
# project by default, which would install the "worker" package we
# deliberately skipped above.
RUN .venv/bin/python -c "\
from faster_whisper import WhisperModel; \
WhisperModel('${WORKER_MODEL}', device='cpu', compute_type='int8')"

FROM python:3.11-slim AS final
RUN apt-get update \
    && apt-get install -y --no-install-recommends ffmpeg \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --create-home --uid 10001 worker

WORKDIR /app
COPY --from=build /src/.venv /app/.venv
COPY --from=build /model-cache /home/worker/.cache/huggingface
COPY --from=build /src/worker /app/worker
RUN chown -R worker:worker /home/worker/.cache

USER worker
ENV PATH="/app/.venv/bin:$PATH"
ENV HF_HUB_OFFLINE=1

ENTRYPOINT ["python", "-m", "worker.main"]
