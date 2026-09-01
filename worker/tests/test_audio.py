import shutil
import subprocess
from pathlib import Path
from unittest.mock import MagicMock

import pytest

from worker import audio
from worker.errors import DecodeError, UnsupportedMediaError

FFMPEG_AVAILABLE = shutil.which("ffmpeg") is not None and shutil.which("ffprobe") is not None


@pytest.fixture
def silent_wav(tmp_path: Path) -> Path:
    """A genuine 1-second silent WAV, generated with the real ffmpeg CLI so
    probe()/normalize() are exercised against real audio rather than a mock.
    """
    path = tmp_path / "silence.wav"
    subprocess.run(
        [
            "ffmpeg",
            "-y",
            "-f",
            "lavfi",
            "-i",
            "anullsrc=r=44100:cl=stereo",
            "-t",
            "1",
            str(path),
        ],
        capture_output=True,
        check=True,
    )
    return path


@pytest.mark.skipif(not FFMPEG_AVAILABLE, reason="ffmpeg/ffprobe not on PATH")
def test_probe_reads_real_audio(silent_wav: Path) -> None:
    info = audio.probe(str(silent_wav))

    assert info.duration_seconds == pytest.approx(1.0, abs=0.1)
    assert info.codec


@pytest.mark.skipif(not FFMPEG_AVAILABLE, reason="ffmpeg/ffprobe not on PATH")
def test_normalize_produces_16khz_mono(silent_wav: Path, tmp_path: Path) -> None:
    dst = tmp_path / "normalized.wav"

    audio.normalize(str(silent_wav), str(dst))

    assert dst.exists()
    info = audio.probe(str(dst))
    assert info.duration_seconds == pytest.approx(1.0, abs=0.1)


@pytest.mark.skipif(not FFMPEG_AVAILABLE, reason="ffmpeg/ffprobe not on PATH")
def test_probe_rejects_non_audio_file(tmp_path: Path) -> None:
    path = tmp_path / "not-audio.txt"
    path.write_text("this is not audio data")

    with pytest.raises(UnsupportedMediaError):
        audio.probe(str(path))


def test_probe_rejects_when_ffprobe_missing(monkeypatch: pytest.MonkeyPatch) -> None:
    def fake_run(*args: object, **kwargs: object) -> None:
        raise OSError("ffprobe not found")

    monkeypatch.setattr(subprocess, "run", fake_run)

    with pytest.raises(UnsupportedMediaError):
        audio.probe("/tmp/whatever.wav")


def test_normalize_raises_decode_error_on_ffmpeg_failure(monkeypatch: pytest.MonkeyPatch) -> None:
    fake_result = MagicMock(returncode=1, stderr="ffmpeg: invalid data")
    monkeypatch.setattr(subprocess, "run", lambda *a, **k: fake_result)

    with pytest.raises(DecodeError):
        audio.normalize("/tmp/in.wav", "/tmp/out.wav")


def test_probe_rejects_zero_duration(monkeypatch: pytest.MonkeyPatch) -> None:
    fake_result = MagicMock(
        returncode=0,
        stdout='{"format": {"duration": "0"}, "streams": [{"codec_name": "pcm_s16le"}]}',
    )
    monkeypatch.setattr(subprocess, "run", lambda *a, **k: fake_result)

    with pytest.raises(UnsupportedMediaError, match="zero or unknown duration"):
        audio.probe("/tmp/in.wav")
