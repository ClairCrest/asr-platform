"""Machine-readable failure taxonomy for the worker. Every failure path
maps to one of these error codes rather than surfacing a bare exception
string to the user, per the project's error handling convention.
"""

from __future__ import annotations


class TranscriptionError(Exception):
    """Base class for every worker failure that terminates a job.

    error_code is written verbatim to jobs.error_code; message is written
    to jobs.error_message.
    """

    error_code: str = "internal_error"

    def __init__(self, message: str) -> None:
        super().__init__(message)
        self.message = message


class UnsupportedMediaError(TranscriptionError):
    """ffmpeg/ffprobe could not identify the file as decodable audio."""

    error_code = "unsupported_media"


class DecodeError(TranscriptionError):
    """ffmpeg recognised the file but failed to decode/normalize it."""

    error_code = "decode_error"


class ModelCrashError(TranscriptionError):
    """faster-whisper raised while transcribing a file it should be able
    to handle — a bug or an unexpected input shape, not a resource limit.
    """

    error_code = "model_crash"


class OutOfMemoryError(TranscriptionError):
    """The process ran out of memory while transcribing."""

    error_code = "oom"


class DownloadError(TranscriptionError):
    """The source object could not be downloaded from object storage."""

    error_code = "download_error"
