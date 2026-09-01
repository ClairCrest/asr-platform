import { Link } from 'react-router'
import type { Job } from '../api/types'
import { formatBytes, formatDuration, relativeTime } from '../lib/format'
import { StatusBadge } from './StatusBadge'

export function JobsTable({ jobs }: { jobs: Job[] }) {
  return (
    <div className="overflow-x-auto rounded-lg border border-gray-200 bg-white">
      <table className="min-w-full divide-y divide-gray-200 text-sm">
        <thead className="bg-gray-50">
          <tr>
            <th className="px-4 py-3 text-left font-medium text-gray-500">File</th>
            <th className="px-4 py-3 text-left font-medium text-gray-500">Status</th>
            <th className="px-4 py-3 text-left font-medium text-gray-500">Model</th>
            <th className="px-4 py-3 text-left font-medium text-gray-500">Duration</th>
            <th className="px-4 py-3 text-left font-medium text-gray-500">Size</th>
            <th className="px-4 py-3 text-left font-medium text-gray-500">Created</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-gray-100">
          {jobs.map((job) => (
            <tr key={job.id} className="hover:bg-gray-50">
              <td className="px-4 py-3">
                <Link to={`/jobs/${job.id}`} className="font-medium text-gray-900 hover:underline">
                  {job.original_filename}
                </Link>
              </td>
              <td className="px-4 py-3">
                <StatusBadge status={job.status} />
              </td>
              <td className="px-4 py-3 text-gray-500">{job.model}</td>
              <td className="px-4 py-3 text-gray-500">{formatDuration(job.duration_seconds)}</td>
              <td className="px-4 py-3 text-gray-500">{formatBytes(job.size_bytes)}</td>
              <td className="px-4 py-3 text-gray-500" title={job.created_at}>
                {relativeTime(job.created_at)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
