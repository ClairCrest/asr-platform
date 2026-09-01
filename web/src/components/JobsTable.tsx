import { Link } from 'react-router'
import type { Job } from '../api/types'
import { formatBytes, formatDuration, relativeTime } from '../lib/format'
import { StatusBadge } from './StatusBadge'

export function JobsTable({ jobs }: { jobs: Job[] }) {
  return (
    <div className="overflow-x-auto rounded-lg border border-gray-200 bg-white dark:border-gray-800 dark:bg-gray-900">
      <table className="min-w-full divide-y divide-gray-200 text-sm dark:divide-gray-800">
        <thead className="bg-gray-50 dark:bg-gray-900">
          <tr>
            <th className="px-4 py-3 text-left font-medium text-gray-500 dark:text-gray-400">File</th>
            <th className="px-4 py-3 text-left font-medium text-gray-500 dark:text-gray-400">Status</th>
            <th className="px-4 py-3 text-left font-medium text-gray-500 dark:text-gray-400">Model</th>
            <th className="px-4 py-3 text-left font-medium text-gray-500 dark:text-gray-400">Duration</th>
            <th className="px-4 py-3 text-left font-medium text-gray-500 dark:text-gray-400">Size</th>
            <th className="px-4 py-3 text-left font-medium text-gray-500 dark:text-gray-400">Created</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-gray-100 dark:divide-gray-800">
          {jobs.map((job) => (
            <tr key={job.id} className="hover:bg-gray-50 dark:hover:bg-gray-800/50">
              <td className="px-4 py-3">
                <Link
                  to={`/jobs/${job.id}`}
                  className="font-medium text-gray-900 hover:underline dark:text-gray-100"
                >
                  {job.original_filename}
                </Link>
              </td>
              <td className="px-4 py-3">
                <StatusBadge status={job.status} />
              </td>
              <td className="px-4 py-3 text-gray-500 dark:text-gray-400">{job.model}</td>
              <td className="px-4 py-3 text-gray-500 dark:text-gray-400">{formatDuration(job.duration_seconds)}</td>
              <td className="px-4 py-3 text-gray-500 dark:text-gray-400">{formatBytes(job.size_bytes)}</td>
              <td className="px-4 py-3 text-gray-500 dark:text-gray-400" title={job.created_at}>
                {relativeTime(job.created_at)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
