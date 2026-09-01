import { useState, type FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { createApiKey, listApiKeys, revokeApiKey } from '../api/auth'
import { EmptyState, ErrorState, LoadingSkeleton } from '../components/States'
import { relativeTime } from '../lib/format'

export function ApiKeysPage() {
  const queryClient = useQueryClient()
  const [name, setName] = useState('')
  const [revealedKey, setRevealedKey] = useState<string | null>(null)

  const keysQuery = useQuery({ queryKey: ['api-keys'], queryFn: listApiKeys })

  const createMutation = useMutation({
    mutationFn: () => createApiKey(name),
    onSuccess: (result) => {
      setRevealedKey(result.key)
      setName('')
      void queryClient.invalidateQueries({ queryKey: ['api-keys'] })
    },
  })

  const revokeMutation = useMutation({
    mutationFn: (id: string) => revokeApiKey(id),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['api-keys'] }),
  })

  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    if (name.trim()) createMutation.mutate()
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-lg font-semibold text-gray-900">API Keys</h1>
        <p className="mt-1 text-sm text-gray-500">
          Use an API key with the <code>X-API-Key</code> header for programmatic access.
        </p>
      </div>

      {revealedKey && (
        <div className="rounded-md border border-amber-300 bg-amber-50 p-4 text-sm">
          <p className="font-medium text-amber-800">
            Copy this key now — it won't be shown again.
          </p>
          <code className="mt-2 block break-all rounded bg-white px-3 py-2 text-xs text-gray-800">
            {revealedKey}
          </code>
          <button
            type="button"
            onClick={() => setRevealedKey(null)}
            className="mt-2 text-xs font-medium text-amber-700 hover:underline"
          >
            Dismiss
          </button>
        </div>
      )}

      <form onSubmit={onSubmit} className="flex gap-2">
        <input
          type="text"
          placeholder="Key name (e.g. ci)"
          value={name}
          onChange={(e) => setName(e.target.value)}
          className="flex-1 rounded-md border border-gray-300 px-3 py-2 text-sm focus:border-gray-900 focus:outline-none"
        />
        <button
          type="submit"
          disabled={createMutation.isPending || !name.trim()}
          className="rounded-md bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-700 disabled:opacity-50"
        >
          Create key
        </button>
      </form>

      {keysQuery.isLoading && <LoadingSkeleton rows={2} />}
      {keysQuery.isError && (
        <ErrorState message="Could not load API keys" onRetry={() => void keysQuery.refetch()} />
      )}
      {keysQuery.data && keysQuery.data.length === 0 && (
        <EmptyState title="No API keys yet" description="Create one to access the API programmatically." />
      )}
      {keysQuery.data && keysQuery.data.length > 0 && (
        <ul className="divide-y divide-gray-100 rounded-lg border border-gray-200 bg-white">
          {keysQuery.data.map((key) => (
            <li key={key.id} className="flex items-center justify-between px-4 py-3 text-sm">
              <div>
                <p className="font-medium text-gray-900">{key.name}</p>
                <p className="text-xs text-gray-400">
                  Created {relativeTime(key.created_at)}
                  {key.last_used_at && ` · last used ${relativeTime(key.last_used_at)}`}
                </p>
              </div>
              <button
                type="button"
                disabled={revokeMutation.isPending}
                onClick={() => revokeMutation.mutate(key.id)}
                className="rounded-md border border-red-200 px-2.5 py-1 text-xs font-medium text-red-600 hover:bg-red-50 disabled:opacity-50"
              >
                Revoke
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
