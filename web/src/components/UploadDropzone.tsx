import { useRef, useState, type DragEvent } from 'react'
import { useUpload } from '../hooks/useUpload'

export function UploadDropzone() {
  const { state, upload, reset } = useUpload()
  const [dragActive, setDragActive] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)

  const handleFiles = (files: FileList | null) => {
    const file = files?.[0]
    if (file) void upload(file)
  }

  const handleDrop = (e: DragEvent<HTMLDivElement>) => {
    e.preventDefault()
    setDragActive(false)
    handleFiles(e.dataTransfer.files)
  }

  const busy = state.phase === 'uploading' || state.phase === 'creating'

  return (
    <div
      onDragOver={(e) => {
        e.preventDefault()
        setDragActive(true)
      }}
      onDragLeave={() => setDragActive(false)}
      onDrop={handleDrop}
      className={`rounded-lg border-2 border-dashed p-8 text-center transition-colors ${
        dragActive ? 'border-gray-900 bg-gray-50' : 'border-gray-300'
      }`}
    >
      <input
        ref={inputRef}
        type="file"
        accept="audio/wav,audio/mpeg,audio/mp4,audio/x-m4a,audio/flac,audio/ogg"
        className="hidden"
        onChange={(e) => handleFiles(e.target.files)}
      />

      {state.phase === 'idle' && (
        <>
          <p className="text-sm text-gray-600">Drag and drop an audio file here, or</p>
          <button
            type="button"
            onClick={() => inputRef.current?.click()}
            className="mt-3 rounded-md bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-700"
          >
            Choose file
          </button>
          <p className="mt-2 text-xs text-gray-400">WAV, MP3, MP4, M4A, FLAC, OGG — up to 200 MB</p>
        </>
      )}

      {state.phase === 'uploading' && (
        <div className="space-y-2">
          <p className="text-sm text-gray-600">Uploading… {state.progress}%</p>
          <div className="h-2 w-full overflow-hidden rounded-full bg-gray-200">
            <div
              className="h-full rounded-full bg-gray-900 transition-all"
              style={{ width: `${state.progress}%` }}
            />
          </div>
        </div>
      )}

      {state.phase === 'creating' && <p className="text-sm text-gray-600">Creating job…</p>}

      {state.phase === 'error' && (
        <div className="space-y-2">
          <p className="text-sm font-medium text-red-600">{state.message}</p>
          <button
            type="button"
            onClick={reset}
            className="rounded-md bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-700"
          >
            Try again
          </button>
        </div>
      )}

      {busy && <p className="sr-only" role="status">Upload in progress</p>}
    </div>
  )
}
