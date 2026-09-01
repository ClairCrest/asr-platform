import { Navigate } from 'react-router'
import { useAuth } from '../auth/AuthContext'
import { Layout } from './Layout'

export function ProtectedRoute() {
  const { user, loading } = useAuth()

  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p className="text-sm text-gray-400">Loading…</p>
      </div>
    )
  }

  if (!user) return <Navigate to="/login" replace />

  return <Layout />
}
