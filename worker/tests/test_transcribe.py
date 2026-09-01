from dataclasses import dataclass

import pytest

from worker import transcribe
from worker.errors import ModelCrashError, OutOfMemoryError


@dataclass
class FakeSegment:
    start: float
    end: float
    text: str
    avg_logprob: float


@dataclass
class FakeInfo:
    language: str
    language_probability: float


class FakeWhisperModel:
    """Stands in for faster_whisper.WhisperModel so tests never load real
    weights or touch the network.
    """

    def __init__(
        self, segments: list[FakeSegment], info: FakeInfo, raises: Exception | None = None
    ):
        self._segments = segments
        self._info = info
        self._raises = raises

    def transcribe(self, wav_path: str, beam_size: int = 5):
        if self._raises is not None:
            raise self._raises
        return iter(self._segments), self._info


def make_transcriber(
    monkeypatch: pytest.MonkeyPatch, fake_model: FakeWhisperModel
) -> transcribe.Transcriber:
    monkeypatch.setattr(transcribe, "WhisperModel", lambda *a, **k: fake_model)
    return transcribe.Transcriber("small.en")


def test_transcribe_builds_segments_and_text(monkeypatch: pytest.MonkeyPatch) -> None:
    fake_model = FakeWhisperModel(
        segments=[
            FakeSegment(start=0.0, end=1.5, text=" hello ", avg_logprob=-0.1),
            FakeSegment(start=1.5, end=3.0, text=" world ", avg_logprob=-0.2),
        ],
        info=FakeInfo(language="en", language_probability=0.98),
    )
    transcriber = make_transcriber(monkeypatch, fake_model)

    result = transcriber.transcribe("/tmp/audio.wav", audio_duration_seconds=3.0)

    assert result.text == "hello world"
    assert len(result.segments) == 2
    assert result.segments[0].start_ms == 0
    assert result.segments[0].end_ms == 1500
    assert result.segments[1].idx == 1
    assert result.language_detected == "en"
    assert result.language_warning is False


def test_transcribe_flags_low_confidence_language(monkeypatch: pytest.MonkeyPatch) -> None:
    fake_model = FakeWhisperModel(
        segments=[FakeSegment(start=0.0, end=1.0, text="bonjour", avg_logprob=-0.3)],
        info=FakeInfo(language="fr", language_probability=0.3),
    )
    transcriber = make_transcriber(monkeypatch, fake_model)

    result = transcriber.transcribe("/tmp/audio.wav", audio_duration_seconds=1.0)

    assert result.language_warning is True


def test_transcribe_computes_real_time_factor(monkeypatch: pytest.MonkeyPatch) -> None:
    fake_model = FakeWhisperModel(
        segments=[FakeSegment(start=0.0, end=1.0, text="hi", avg_logprob=-0.1)],
        info=FakeInfo(language="en", language_probability=0.9),
    )
    transcriber = make_transcriber(monkeypatch, fake_model)

    result = transcriber.transcribe("/tmp/audio.wav", audio_duration_seconds=10.0)

    assert result.real_time_factor == pytest.approx(result.processing_seconds / 10.0)


def test_transcribe_maps_memory_error(monkeypatch: pytest.MonkeyPatch) -> None:
    fake_model = FakeWhisperModel(segments=[], info=FakeInfo("en", 0.9), raises=MemoryError("oom"))
    transcriber = make_transcriber(monkeypatch, fake_model)

    with pytest.raises(OutOfMemoryError):
        transcriber.transcribe("/tmp/audio.wav", audio_duration_seconds=1.0)


def test_transcribe_maps_unexpected_error_to_model_crash(monkeypatch: pytest.MonkeyPatch) -> None:
    fake_model = FakeWhisperModel(
        segments=[], info=FakeInfo("en", 0.9), raises=RuntimeError("boom")
    )
    transcriber = make_transcriber(monkeypatch, fake_model)

    with pytest.raises(ModelCrashError):
        transcriber.transcribe("/tmp/audio.wav", audio_duration_seconds=1.0)
