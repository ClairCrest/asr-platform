// Mirrors api/internal/http/dto — kept as plain types, not generated, since
// the DTOs are small and stable enough that hand-syncing beats a codegen
// step for a project this size.

export interface User {
  id: string
  email: string
  created_at: string
}

export interface TokenPair {
  access_token: string
  refresh_token: string
}

export interface ApiKey {
  id: string
  name: string
  last_used_at?: string
  created_at: string
}

export interface CreateApiKeyResult extends ApiKey {
  key: string
}

export interface WsTicket {
  ticket: string
}

export interface CreateUploadResult {
  upload_url: string
  object_key: string
}

export type JobStatus =
  | 'pending'
  | 'queued'
  | 'processing'
  | 'retrying'
  | 'succeeded'
  | 'failed'
  | 'cancelled'

export interface Job {
  id: string
  status: JobStatus
  object_key: string
  original_filename: string
  size_bytes: number
  duration_seconds?: number
  model: string
  attempts: number
  max_attempts: number
  error_code?: string
  error_message?: string
  created_at: string
  started_at?: string
  finished_at?: string
}

export interface JobEvent {
  event_type: string
  payload: Record<string, unknown>
  created_at: string
}

export interface JobDetail extends Job {
  events: JobEvent[]
}

export interface JobList {
  jobs: Job[]
  next_cursor?: string
}

export interface Segment {
  idx: number
  start_ms: number
  end_ms: number
  text: string
  avg_logprob?: number
}

export interface Transcript {
  audio_url: string
  text: string
  language_detected: string
  language_probability: number
  language_warning: boolean
  model: string
  processing_seconds: number
  real_time_factor: number
  created_at: string
  segments: Segment[]
}

export interface ApiErrorBody {
  error: {
    code: string
    message: string
    request_id?: string
  }
}
