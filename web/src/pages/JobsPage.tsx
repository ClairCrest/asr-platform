import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { listJobs } from '../api/jobs'
import type { JobStatus } from '../api/types'
import { UploadDropzone } from '../components/UploadDropzone'
import { JobsTable } from '../components/JobsTable'
import { EmptyState, ErrorState, LoadingSkeleton } from '../components/States'

const STATUS_FILTERS: { label: string; value: JobStatus | undefined }[] = [
  { label: 'All', value: undefined },
  { label: 'Queued', value: 'queued' },
  { label: 'Processing', value: 'processing' },
  { label: 'Succeeded', value: 'succeeded' },
  { label: 'Failed', value: 'failed' },
  { label: 'Cancelled', value: 'cancelled' },
]

export function JobsPage() {
  const [status, setStatus] = useState<JobStatus | undefined>(undefined)

  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['jobs', status],
    queryFn: () => listJobs({ status }),
  })

  return (
    <div className="space-y-8">
      <section>
        <h2 className="text-sm font-medium text-gray-700">Upload audio</h2>
        <div className="mt-2">
          <UploadDropzone />
        </div>
      </section>

      <section>
        <div className="mb-3 flex items-center justify-between">
          <h2 className="text-sm font-medium text-gray-700">Jobs</h2>
          <div className="flex gap-1">
            {STATUS_FILTERS.map((f) => (
              <button
                key={f.label}
                type="button"
                onClick={() => setStatus(f.value)}
                className={`rounded-md px-2.5 py-1 text-xs font-medium ${
                  status === f.value ? 'bg-gray-900 text-white' : 'text-gray-500 hover:bg-gray-100'
                }`}
              >
                {f.label}
              </button>
            ))}
          </div>
        </div>

        {isLoading && <LoadingSkeleton />}
        {isError && (
          <ErrorState
            message={error instanceof Error ? error.message : 'Could not load jobs'}
            onRetry={() => void refetch()}
          />
        )}
        {data && data.jobs.length === 0 && (
          <EmptyState title="No jobs yet" description="Upload an audio file to get started." />
        )}
        {data && data.jobs.length > 0 && <JobsTable jobs={data.jobs} />}
      </section>
    </div>
  )
}
