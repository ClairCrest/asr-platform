import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import * as authApi from '../api/auth'
import { clearTokens, isAuthenticated, setTokens } from '../api/tokens'
import type { User } from '../api/types'

interface AuthContextValue {
  user: User | null
  loading: boolean
  login: (email: string, password: string) => Promise<void>
  register: (email: string, password: string) => Promise<void>
  logout: () => void
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)
  const queryClient = useQueryClient()

  const loadUser = useCallback(async () => {
    if (!isAuthenticated()) {
      setUser(null)
      setLoading(false)
      return
    }
    try {
      const u = await authApi.me()
      setUser(u)
    } catch {
      clearTokens()
      setUser(null)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void loadUser()
  }, [loadUser])

  const login = useCallback(
    async (email: string, password: string) => {
      const pair = await authApi.login(email, password)
      setTokens(pair.access_token, pair.refresh_token)
      await loadUser()
    },
    [loadUser],
  )

  const register = useCallback(
    async (email: string, password: string) => {
      const pair = await authApi.register(email, password)
      setTokens(pair.access_token, pair.refresh_token)
      await loadUser()
    },
    [loadUser],
  )

  const logout = useCallback(() => {
    clearTokens()
    setUser(null)
    queryClient.clear()
  }, [queryClient])

  return (
    <AuthContext.Provider value={{ user, loading, login, register, logout }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
