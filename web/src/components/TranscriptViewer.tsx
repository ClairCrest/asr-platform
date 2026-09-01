import { useRef, useState } from 'react'
import type { Segment, Transcript } from '../api/types'
import { formatMs } from '../lib/format'

export function TranscriptViewer({ transcript }: { transcript: Transcript }) {
  const audioRef = useRef<HTMLAudioElement>(null)
  const [activeIdx, setActiveIdx] = useState<number | null>(null)
  const [copied, setCopied] = useState(false)

  const seekTo = (segment: Segment) => {
    if (audioRef.current) {
      audioRef.current.currentTime = segment.start_ms / 1000
      void audioRef.current.play()
    }
    setActiveIdx(segment.idx)
  }

  const copyText = async () => {
    await navigator.clipboard.writeText(transcript.text)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <div className="space-y-4">
      {transcript.language_warning && (
        <div className="rounded-md border border-amber-300 bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:border-amber-900 dark:bg-amber-950 dark:text-amber-300">
          This audio does not appear to be English (detected{' '}
          <span className="font-medium">{transcript.language_detected}</span> at{' '}
          {Math.round(transcript.language_probability * 100)}% confidence). The transcript below
          may be unreliable.
        </div>
      )}

      <audio ref={audioRef} controls src={transcript.audio_url} className="w-full">
        <track kind="captions" />
      </audio>

      <div className="flex items-center justify-between">
        <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300">Transcript</h3>
        <div className="flex gap-2">
          <button
            type="button"
            onClick={() => void copyText()}
            className="rounded-md border border-gray-300 px-2.5 py-1 text-xs font-medium text-gray-600 hover:bg-gray-50 dark:border-gray-700 dark:text-gray-400 dark:hover:bg-gray-800"
          >
            {copied ? 'Copied!' : 'Copy text'}
          </button>
        </div>
      </div>

      <div className="max-h-96 space-y-1 overflow-y-auto rounded-lg border border-gray-200 bg-white p-4 dark:border-gray-800 dark:bg-gray-900">
        {transcript.segments.length === 0 && (
          <p className="text-sm text-gray-400 dark:text-gray-500">{transcript.text}</p>
        )}
        {transcript.segments.map((segment) => (
          <button
            key={segment.idx}
            type="button"
            onClick={() => seekTo(segment)}
            className={`block w-full rounded-md px-2 py-1.5 text-left text-sm ${
              activeIdx === segment.idx
                ? 'bg-gray-100 dark:bg-gray-800'
                : 'hover:bg-gray-50 dark:hover:bg-gray-800/50'
            }`}
          >
            <span className="mr-2 font-mono text-xs text-gray-400 dark:text-gray-500">
              {formatMs(segment.start_ms)}
            </span>
            <span className="text-gray-800 dark:text-gray-200">{segment.text}</span>
          </button>
        ))}
      </div>

      <p className="text-xs text-gray-400 dark:text-gray-500">
        {transcript.model} · real-time factor {transcript.real_time_factor.toFixed(2)}x ·{' '}
        {transcript.processing_seconds.toFixed(1)}s processing
      </p>
    </div>
  )
}
