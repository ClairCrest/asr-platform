import type { JobStatus } from '../api/types'

const STYLES: Record<JobStatus, string> = {
  pending: 'bg-gray-100 text-gray-700 dark:bg-gray-800 dark:text-gray-300',
  queued: 'bg-blue-100 text-blue-700 dark:bg-blue-950 dark:text-blue-300',
  processing: 'bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-300',
  retrying: 'bg-amber-100 text-amber-700 dark:bg-amber-950 dark:text-amber-300',
  succeeded: 'bg-green-100 text-green-700 dark:bg-green-950 dark:text-green-300',
  failed: 'bg-red-100 text-red-700 dark:bg-red-950 dark:text-red-300',
  cancelled: 'bg-gray-100 text-gray-500 dark:bg-gray-800 dark:text-gray-500',
}

export function StatusBadge({ status }: { status: JobStatus }) {
  return (
    <span
      className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium capitalize ${STYLES[status]}`}
    >
      {status}
    </span>
  )
}
