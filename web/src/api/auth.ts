import { apiRequest } from './client'
import type { ApiKey, CreateApiKeyResult, TokenPair, User, WsTicket } from './types'

export function register(email: string, password: string): Promise<TokenPair> {
  return apiRequest<TokenPair>('/api/v1/auth/register', {
    method: 'POST',
    body: { email, password },
    auth: false,
  })
}

export function login(email: string, password: string): Promise<TokenPair> {
  return apiRequest<TokenPair>('/api/v1/auth/login', {
    method: 'POST',
    body: { email, password },
    auth: false,
  })
}

export function me(): Promise<User> {
  return apiRequest<User>('/api/v1/me')
}

export function createApiKey(name: string): Promise<CreateApiKeyResult> {
  return apiRequest<CreateApiKeyResult>('/api/v1/api-keys', { method: 'POST', body: { name } })
}

export function listApiKeys(): Promise<ApiKey[]> {
  return apiRequest<ApiKey[]>('/api/v1/api-keys')
}

export function revokeApiKey(id: string): Promise<void> {
  return apiRequest<void>(`/api/v1/api-keys/${id}`, { method: 'DELETE' })
}

export function createWsTicket(): Promise<WsTicket> {
  return apiRequest<WsTicket>('/api/v1/ws-ticket', { method: 'POST' })
}
