import { useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { createJob, createUpload, putUpload } from '../api/jobs'

export const ACCEPTED_CONTENT_TYPES = [
  'audio/wav',
  'audio/mpeg',
  'audio/mp4',
  'audio/x-m4a',
  'audio/flac',
  'audio/ogg',
]
export const MAX_UPLOAD_SIZE_BYTES = 200 * 1024 * 1024

export function validateAudioFile(file: File): string | null {
  if (file.size > MAX_UPLOAD_SIZE_BYTES) return 'File must be 200 MB or smaller.'
  if (!ACCEPTED_CONTENT_TYPES.includes(file.type)) {
    return 'Unsupported file type. Use WAV, MP3, MP4, M4A, FLAC, or OGG.'
  }
  return null
}

type UploadState =
  | { phase: 'idle' }
  | { phase: 'uploading'; progress: number }
  | { phase: 'creating' }
  | { phase: 'error'; message: string }

export function useUpload() {
  const [state, setState] = useState<UploadState>({ phase: 'idle' })
  const queryClient = useQueryClient()

  const upload = async (file: File) => {
    const validationError = validateAudioFile(file)
    if (validationError) {
      setState({ phase: 'error', message: validationError })
      return
    }

    try {
      setState({ phase: 'uploading', progress: 0 })
      const { upload_url, object_key } = await createUpload({
        filename: file.name,
        size_bytes: file.size,
        content_type: file.type,
      })

      await putUpload(upload_url, file, (pct) => setState({ phase: 'uploading', progress: pct }))

      setState({ phase: 'creating' })
      await createJob({ object_key, original_filename: file.name, size_bytes: file.size })

      await queryClient.invalidateQueries({ queryKey: ['jobs'] })
      setState({ phase: 'idle' })
    } catch (err) {
      setState({ phase: 'error', message: err instanceof Error ? err.message : 'Upload failed' })
    }
  }

  const reset = () => setState({ phase: 'idle' })

  return { state, upload, reset }
}
