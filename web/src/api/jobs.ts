import { apiRequest, apiRequestText } from './client'
import type { CreateUploadResult, JobDetail, JobList, JobStatus, Transcript } from './types'

export interface CreateUploadParams {
  filename: string
  size_bytes: number
  content_type: string
}

export function createUpload(params: CreateUploadParams): Promise<CreateUploadResult> {
  return apiRequest<CreateUploadResult>('/api/v1/uploads', { method: 'POST', body: params })
}

export async function putUpload(uploadUrl: string, file: File, onProgress?: (pct: number) => void): Promise<void> {
  await new Promise<void>((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open('PUT', uploadUrl)
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable && onProgress) onProgress(Math.round((e.loaded / e.total) * 100))
    }
    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) resolve()
      else reject(new Error(`upload failed with status ${xhr.status}`))
    }
    xhr.onerror = () => reject(new Error('upload failed'))
    xhr.send(file)
  })
}

export interface CreateJobParams {
  object_key: string
  original_filename: string
  size_bytes: number
  model?: string
  idempotencyKey?: string
}

export function createJob(params: CreateJobParams): Promise<JobDetail> {
  const { idempotencyKey, ...body } = params
  return apiRequest<JobDetail>('/api/v1/jobs', {
    method: 'POST',
    body,
    headers: idempotencyKey ? { 'Idempotency-Key': idempotencyKey } : {},
  })
}

export interface ListJobsParams {
  status?: JobStatus
  cursor?: string
  limit?: number
}

export function listJobs(params: ListJobsParams = {}): Promise<JobList> {
  const query = new URLSearchParams()
  if (params.status) query.set('status', params.status)
  if (params.cursor) query.set('cursor', params.cursor)
  if (params.limit) query.set('limit', String(params.limit))
  const qs = query.toString()
  return apiRequest<JobList>(`/api/v1/jobs${qs ? `?${qs}` : ''}`)
}

export function getJob(id: string): Promise<JobDetail> {
  return apiRequest<JobDetail>(`/api/v1/jobs/${id}`)
}

export function cancelJob(id: string): Promise<JobDetail> {
  return apiRequest<JobDetail>(`/api/v1/jobs/${id}/cancel`, { method: 'POST' })
}

export function retryJob(id: string): Promise<JobDetail> {
  return apiRequest<JobDetail>(`/api/v1/jobs/${id}/retry`, { method: 'POST' })
}

export function deleteJob(id: string): Promise<void> {
  return apiRequest<void>(`/api/v1/jobs/${id}`, { method: 'DELETE' })
}

export function getTranscript(id: string): Promise<Transcript> {
  return apiRequest<Transcript>(`/api/v1/jobs/${id}/transcript?format=json`)
}

export function getTranscriptText(id: string, format: 'txt' | 'srt' | 'vtt'): Promise<string> {
  return apiRequestText(`/api/v1/jobs/${id}/transcript?format=${format}`)
}
