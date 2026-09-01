// Access and refresh tokens live in localStorage. This is a portfolio
// project trade-off: httpOnly cookies would be the safer default against
// XSS, but they need a same-site backend/frontend deployment and CSRF
// handling that isn't worth the complexity here.

const ACCESS_KEY = 'asr_access_token'
const REFRESH_KEY = 'asr_refresh_token'

export function getAccessToken(): string | null {
  return localStorage.getItem(ACCESS_KEY)
}

export function getRefreshToken(): string | null {
  return localStorage.getItem(REFRESH_KEY)
}

export function setTokens(accessToken: string, refreshToken: string): void {
  localStorage.setItem(ACCESS_KEY, accessToken)
  localStorage.setItem(REFRESH_KEY, refreshToken)
}

export function clearTokens(): void {
  localStorage.removeItem(ACCESS_KEY)
  localStorage.removeItem(REFRESH_KEY)
}

export function isAuthenticated(): boolean {
  return getAccessToken() !== null
}
