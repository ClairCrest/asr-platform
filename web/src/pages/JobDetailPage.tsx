import { useState } from 'react'
import { useNavigate, useParams } from 'react-router'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { cancelJob, deleteJob, getJob, getTranscript, getTranscriptText, retryJob } from '../api/jobs'
import { StatusBadge } from '../components/StatusBadge'
import { TranscriptViewer } from '../components/TranscriptViewer'
import { ErrorState, LoadingSkeleton } from '../components/States'
import { downloadText } from '../lib/download'
import { formatBytes, formatDuration, relativeTime } from '../lib/format'

const TERMINAL_TRANSCRIPT_STATUSES = new Set(['succeeded'])

export function JobDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [actionError, setActionError] = useState<string | null>(null)

  const jobQuery = useQuery({
    queryKey: ['job', id],
    queryFn: () => getJob(id!),
    enabled: !!id,
  })

  const transcriptQuery = useQuery({
    queryKey: ['transcript', id],
    queryFn: () => getTranscript(id!),
    enabled: !!id && jobQuery.data?.status !== undefined && TERMINAL_TRANSCRIPT_STATUSES.has(jobQuery.data.status),
  })

  const invalidate = () => {
    void queryClient.invalidateQueries({ queryKey: ['job', id] })
    void queryClient.invalidateQueries({ queryKey: ['jobs'] })
  }

  const cancelMutation = useMutation({
    mutationFn: () => cancelJob(id!),
    onSuccess: invalidate,
    onError: (e) => setActionError(e instanceof Error ? e.message : 'Could not cancel job'),
  })
  const retryMutation = useMutation({
    mutationFn: () => retryJob(id!),
    onSuccess: invalidate,
    onError: (e) => setActionError(e instanceof Error ? e.message : 'Could not retry job'),
  })
  const deleteMutation = useMutation({
    mutationFn: () => deleteJob(id!),
    onSuccess: () => void navigate('/jobs'),
    onError: (e) => setActionError(e instanceof Error ? e.message : 'Could not delete job'),
  })

  const downloadAs = async (format: 'txt' | 'srt' | 'vtt') => {
    if (!id || !jobQuery.data) return
    const text = await getTranscriptText(id, format)
    const base = jobQuery.data.original_filename.replace(/\.[^.]+$/, '')
    downloadText(`${base}.${format}`, text, 'text/plain')
  }

  if (jobQuery.isLoading) return <LoadingSkeleton rows={3} />
  if (jobQuery.isError || !jobQuery.data) {
    return (
      <ErrorState
        message={jobQuery.error instanceof Error ? jobQuery.error.message : 'Could not load job'}
        onRetry={() => void jobQuery.refetch()}
      />
    )
  }

  const job = jobQuery.data
  const canCancel = job.status === 'queued' || job.status === 'processing'
  const canRetry = job.status === 'failed'

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between">
        <div>
          <h1 className="text-lg font-semibold text-gray-900">{job.original_filename}</h1>
          <p className="mt-1 text-sm text-gray-500">
            {job.model} · {formatBytes(job.size_bytes)} · created {relativeTime(job.created_at)}
          </p>
        </div>
        <StatusBadge status={job.status} />
      </div>

      {job.error_message && (
        <div className="rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
          <span className="font-medium">{job.error_code}:</span> {job.error_message}
        </div>
      )}

      <dl className="grid grid-cols-2 gap-4 rounded-lg border border-gray-200 bg-white p-4 text-sm sm:grid-cols-4">
        <div>
          <dt className="text-gray-400">Duration</dt>
          <dd className="mt-0.5 text-gray-900">{formatDuration(job.duration_seconds)}</dd>
        </div>
        <div>
          <dt className="text-gray-400">Attempts</dt>
          <dd className="mt-0.5 text-gray-900">
            {job.attempts} / {job.max_attempts}
          </dd>
        </div>
        <div>
          <dt className="text-gray-400">Started</dt>
          <dd className="mt-0.5 text-gray-900">{job.started_at ? relativeTime(job.started_at) : '—'}</dd>
        </div>
        <div>
          <dt className="text-gray-400">Finished</dt>
          <dd className="mt-0.5 text-gray-900">{job.finished_at ? relativeTime(job.finished_at) : '—'}</dd>
        </div>
      </dl>

      {actionError && <p className="text-sm text-red-600">{actionError}</p>}

      <div className="flex gap-2">
        {canCancel && (
          <button
            type="button"
            disabled={cancelMutation.isPending}
            onClick={() => cancelMutation.mutate()}
            className="rounded-md border border-gray-300 px-3 py-1.5 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50"
          >
            Cancel
          </button>
        )}
        {canRetry && (
          <button
            type="button"
            disabled={retryMutation.isPending}
            onClick={() => retryMutation.mutate()}
            className="rounded-md border border-gray-300 px-3 py-1.5 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50"
          >
            Retry
          </button>
        )}
        <button
          type="button"
          disabled={deleteMutation.isPending}
          onClick={() => {
            if (confirm('Delete this job and its audio file? This cannot be undone.')) {
              deleteMutation.mutate()
            }
          }}
          className="rounded-md border border-red-200 px-3 py-1.5 text-sm font-medium text-red-600 hover:bg-red-50 disabled:opacity-50"
        >
          Delete
        </button>
      </div>

      {job.status === 'succeeded' && (
        <section>
          {transcriptQuery.isLoading && <LoadingSkeleton rows={4} />}
          {transcriptQuery.isError && (
            <ErrorState message="Could not load transcript" onRetry={() => void transcriptQuery.refetch()} />
          )}
          {transcriptQuery.data && (
            <>
              <TranscriptViewer transcript={transcriptQuery.data} />
              <div className="mt-3 flex gap-2">
                {(['txt', 'srt', 'vtt'] as const).map((format) => (
                  <button
                    key={format}
                    type="button"
                    onClick={() => void downloadAs(format)}
                    className="rounded-md border border-gray-300 px-2.5 py-1 text-xs font-medium text-gray-600 hover:bg-gray-50"
                  >
                    Download .{format}
                  </button>
                ))}
              </div>
            </>
          )}
        </section>
      )}

      {job.status !== 'succeeded' && job.status !== 'failed' && job.status !== 'cancelled' && (
        <p className="text-sm text-gray-400">The transcript will appear here once the job finishes.</p>
      )}
    </div>
  )
}
