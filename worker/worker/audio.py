"""ffmpeg-backed audio probing and normalization. The worker never trusts a
client-declared content type: it asks ffprobe what the file actually is.
"""

from __future__ import annotations

import json
import logging
import subprocess
from dataclasses import dataclass

from worker.errors import DecodeError, UnsupportedMediaError

logger = logging.getLogger("asr-worker.audio")

TARGET_SAMPLE_RATE = 16000
MAX_DURATION_SECONDS = 2 * 60 * 60  # 2 hours, per the plan's upload constraints


@dataclass(frozen=True)
class AudioInfo:
    duration_seconds: float
    codec: str


def probe(path: str) -> AudioInfo:
    """Identify path as decodable audio via ffprobe.

    Raises UnsupportedMediaError if ffprobe cannot identify an audio
    stream at all (corrupt file, non-audio file, unsupported codec).
    """
    try:
        result = subprocess.run(
            [
                "ffprobe",
                "-v",
                "error",
                "-print_format",
                "json",
                "-show_format",
                "-show_streams",
                "-select_streams",
                "a:0",
                path,
            ],
            capture_output=True,
            text=True,
            timeout=30,
        )
    except (OSError, subprocess.TimeoutExpired) as exc:
        raise UnsupportedMediaError(f"ffprobe could not run: {exc}") from exc

    if result.returncode != 0:
        raise UnsupportedMediaError(f"ffprobe rejected the file: {result.stderr.strip()}")

    try:
        info = json.loads(result.stdout)
        streams = info.get("streams", [])
        if not streams:
            raise UnsupportedMediaError("no audio stream found")
        stream = streams[0]
        duration = float(info.get("format", {}).get("duration") or stream.get("duration") or 0.0)
        codec = str(stream.get("codec_name", "unknown"))
    except (ValueError, KeyError) as exc:
        raise UnsupportedMediaError(f"could not parse ffprobe output: {exc}") from exc

    if duration <= 0:
        raise UnsupportedMediaError("audio stream has zero or unknown duration")
    if duration > MAX_DURATION_SECONDS:
        raise UnsupportedMediaError(
            f"audio duration {duration:.0f}s exceeds the {MAX_DURATION_SECONDS}s limit"
        )

    return AudioInfo(duration_seconds=duration, codec=codec)


def normalize(src_path: str, dst_path: str) -> None:
    """Convert src_path to 16 kHz mono PCM WAV at dst_path.

    Raises DecodeError if ffmpeg fails partway through, which is distinct
    from ffprobe rejecting the file outright: the file looked decodable
    but the actual conversion failed.
    """
    try:
        result = subprocess.run(
            [
                "ffmpeg",
                "-y",
                "-i",
                src_path,
                "-ar",
                str(TARGET_SAMPLE_RATE),
                "-ac",
                "1",
                "-c:a",
                "pcm_s16le",
                dst_path,
            ],
            capture_output=True,
            text=True,
            timeout=600,
        )
    except (OSError, subprocess.TimeoutExpired) as exc:
        raise DecodeError(f"ffmpeg could not run: {exc}") from exc

    if result.returncode != 0:
        raise DecodeError(f"ffmpeg failed to normalize audio: {result.stderr.strip()[-500:]}")
