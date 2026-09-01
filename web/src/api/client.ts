import { clearTokens, getAccessToken, getRefreshToken, setTokens } from './tokens'
import type { ApiErrorBody, TokenPair } from './types'

const BASE_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8080'

export class ApiError extends Error {
  code: string
  status: number
  requestId?: string

  constructor(status: number, code: string, message: string, requestId?: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.requestId = requestId
  }
}

// Concurrent 401s share a single in-flight refresh instead of each firing
// their own POST /auth/refresh (which would race to rotate the token).
let refreshPromise: Promise<string> | null = null

async function refreshAccessToken(): Promise<string> {
  if (refreshPromise) return refreshPromise

  refreshPromise = (async () => {
    const refreshToken = getRefreshToken()
    if (!refreshToken) throw new ApiError(401, 'unauthorized', 'not logged in')

    const res = await fetch(`${BASE_URL}/api/v1/auth/refresh`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: refreshToken }),
    })
    if (!res.ok) {
      clearTokens()
      throw new ApiError(401, 'unauthorized', 'session expired')
    }
    const pair = (await res.json()) as TokenPair
    setTokens(pair.access_token, pair.refresh_token)
    return pair.access_token
  })()

  try {
    return await refreshPromise
  } finally {
    refreshPromise = null
  }
}

interface RequestOptions {
  method?: string
  body?: unknown
  auth?: boolean
  headers?: Record<string, string>
}

export async function apiRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { method = 'GET', body, auth = true, headers = {} } = options

  const doFetch = async (): Promise<Response> => {
    const finalHeaders: Record<string, string> = { ...headers }
    if (body !== undefined) finalHeaders['Content-Type'] = 'application/json'
    if (auth) {
      const token = getAccessToken()
      if (token) finalHeaders.Authorization = `Bearer ${token}`
    }
    return fetch(`${BASE_URL}${path}`, {
      method,
      headers: finalHeaders,
      body: body !== undefined ? JSON.stringify(body) : undefined,
    })
  }

  let res = await doFetch()

  if (res.status === 401 && auth && getRefreshToken()) {
    try {
      await refreshAccessToken()
      res = await doFetch()
    } catch {
      // fall through to the normal error handling below with the
      // original 401 response
    }
  }

  if (!res.ok) {
    let errorBody: ApiErrorBody | null = null
    try {
      errorBody = (await res.json()) as ApiErrorBody
    } catch {
      // response body was not JSON; fall back to a generic error below
    }
    throw new ApiError(
      res.status,
      errorBody?.error.code ?? 'unknown_error',
      errorBody?.error.message ?? res.statusText,
      errorBody?.error.request_id,
    )
  }

  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

export async function apiRequestText(path: string, options: RequestOptions = {}): Promise<string> {
  const { method = 'GET', auth = true } = options
  const headers: Record<string, string> = {}
  if (auth) {
    const token = getAccessToken()
    if (token) headers.Authorization = `Bearer ${token}`
  }
  const res = await fetch(`${BASE_URL}${path}`, { method, headers })
  if (!res.ok) throw new ApiError(res.status, 'unknown_error', res.statusText)
  return res.text()
}

export function apiBaseUrl(): string {
  return BASE_URL
}
