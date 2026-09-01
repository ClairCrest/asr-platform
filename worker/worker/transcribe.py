"""faster-whisper wrapper. Model weights are baked into the worker image at
build time (see Dockerfile) — this module never downloads a model at
runtime, per the project's convention.
"""

from __future__ import annotations

import logging
import time
from dataclasses import dataclass

from faster_whisper import WhisperModel

from worker.errors import ModelCrashError, OutOfMemoryError

logger = logging.getLogger("asr-worker.transcribe")

# Below this, the audio very likely is not English and the transcript
# should carry a warning rather than being silently trusted.
ENGLISH_CONFIDENCE_THRESHOLD = 0.5


@dataclass(frozen=True)
class SegmentResult:
    idx: int
    start_ms: int
    end_ms: int
    text: str
    avg_logprob: float


@dataclass(frozen=True)
class TranscriptResult:
    text: str
    language_detected: str
    language_probability: float
    language_warning: bool
    segments: list[SegmentResult]
    processing_seconds: float
    real_time_factor: float


class Transcriber:
    def __init__(self, model_name: str) -> None:
        self._model_name = model_name
        self._model = WhisperModel(model_name, device="cpu", compute_type="int8")

    def transcribe(self, wav_path: str, audio_duration_seconds: float) -> TranscriptResult:
        started = time.monotonic()
        try:
            segments_iter, info = self._model.transcribe(wav_path, beam_size=5)
            segments = [
                SegmentResult(
                    idx=i,
                    start_ms=int(seg.start * 1000),
                    end_ms=int(seg.end * 1000),
                    text=seg.text.strip(),
                    avg_logprob=seg.avg_logprob,
                )
                for i, seg in enumerate(segments_iter)
            ]
        except MemoryError as exc:
            raise OutOfMemoryError(f"out of memory transcribing with {self._model_name}") from exc
        except Exception as exc:  # faster-whisper/ctranslate2 raise plain Exception/RuntimeError
            raise ModelCrashError(
                f"model crashed transcribing with {self._model_name}: {exc}"
            ) from exc

        processing_seconds = time.monotonic() - started
        real_time_factor = (
            processing_seconds / audio_duration_seconds if audio_duration_seconds > 0 else 0.0
        )
        text = " ".join(s.text for s in segments).strip()

        return TranscriptResult(
            text=text,
            language_detected=info.language,
            language_probability=info.language_probability,
            language_warning=info.language_probability < ENGLISH_CONFIDENCE_THRESHOLD,
            segments=segments,
            processing_seconds=processing_seconds,
            real_time_factor=real_time_factor,
        )
