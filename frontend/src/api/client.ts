import type { ZodType } from 'zod'

export const API_BASE_URL = (import.meta.env.VITE_API_URL ?? 'http://localhost:8090').replace(/\/+$/, '')
const tokenRefreshedEvent = 'profile-token-refreshed'
const sessionExpiredEvent = 'profile-session-expired'

let refreshInFlight: Promise<string | null> | null = null

// The access token lives here rather than being threaded through every call.
// ProfileAuthProvider keeps it in sync with the React state it renders from.
let accessToken: string | null = null

export function setAccessToken(token: string | null): void {
  accessToken = token
}

// For the few requests that bypass apiFetch because the response isn't JSON.
export function authHeaders(): HeadersInit {
  return accessToken ? { Authorization: `Bearer ${accessToken}` } : {}
}

export class ApiError extends Error {
  readonly status: number

  constructor(message: string, status: number) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

// withAuthHeader attaches the current access token, leaving an Authorization
// header the caller set alone. Returns null when there's no token to send, so
// anonymous requests stay anonymous and a 401 on one isn't worth a refresh.
function withAuthHeader(init?: RequestInit): RequestInit | null {
  const headers = new Headers(init?.headers)
  if (headers.has('Authorization')) return { ...init, headers }
  if (!accessToken) return null

  headers.set('Authorization', `Bearer ${accessToken}`)
  return { ...init, headers }
}

async function requestAccessToken(): Promise<string | null> {
  const res = await fetch(`${API_BASE_URL}/api/profile/refresh`, {
    method: 'POST',
    credentials: 'include'
  })
  if (!res.ok) return null

  const body = (await res.json()) as { token?: unknown }
  if (typeof body.token !== 'string' || body.token === '') return null

  return body.token
}

async function refreshAccessToken(): Promise<string | null> {
  if (refreshInFlight) return refreshInFlight

  refreshInFlight = (async () => {
    try {
      // Refresh tokens are single-use, so tabs refreshing at once would invalidate each other.
      const token =
        'locks' in navigator
          ? await navigator.locks.request('profile-refresh', requestAccessToken)
          : await requestAccessToken()
      if (!token) return null

      // Set before the event so a request firing now uses the new token
      // instead of waiting on the provider's next render.
      accessToken = token
      window.dispatchEvent(new CustomEvent<string>(tokenRefreshedEvent, { detail: token }))
      return token
    } catch {
      return null
    } finally {
      refreshInFlight = null
    }
  })()

  return refreshInFlight
}

async function apiFetchInternal<T>(
  path: string,
  schema: ZodType<T>,
  init: RequestInit | undefined,
  refreshed: boolean
): Promise<T> {
  const authed = withAuthHeader(init)
  const request = authed ?? init
  const res = await fetch(`${API_BASE_URL}${path}`, { ...request, credentials: request?.credentials ?? 'include' })

  if (res.status === 401 && !refreshed && path !== '/api/profile/refresh' && authed) {
    const token = await refreshAccessToken()
    if (token) {
      const headers = new Headers(init?.headers)
      headers.set('Authorization', `Bearer ${token}`)
      return apiFetchInternal(path, schema, { ...init, headers }, true)
    }

    window.dispatchEvent(new Event(sessionExpiredEvent))
  }

  if (!res.ok) {
    let message = res.statusText

    try {
      const body = (await res.json()) as { error?: string }
      if (body.error) message = body.error
    } catch {
      // response had no JSON body
    }

    throw new ApiError(message, res.status)
  }

  return schema.parse(await res.json())
}

export function apiFetch<T>(path: string, schema: ZodType<T>, init?: RequestInit): Promise<T> {
  return apiFetchInternal(path, schema, init, false)
}

export { refreshAccessToken, sessionExpiredEvent, tokenRefreshedEvent }
