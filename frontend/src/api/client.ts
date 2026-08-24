import type { ZodType } from 'zod'

export const API_BASE_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8090'
const TOKEN_STORAGE_KEY = 'profile_token'
const tokenRefreshedEvent = 'profile-token-refreshed'
const sessionExpiredEvent = 'profile-session-expired'

let refreshInFlight: Promise<string | null> | null = null

export class ApiError extends Error {
  readonly status: number

  constructor(message: string, status: number) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

function hasBearerToken(init?: RequestInit): boolean {
  return new Headers(init?.headers).has('Authorization')
}

async function refreshAccessToken(): Promise<string | null> {
  if (refreshInFlight) return refreshInFlight

  refreshInFlight = (async () => {
    try {
      const res = await fetch(`${API_BASE_URL}/api/profile/refresh`, {
        method: 'POST',
        credentials: 'include',
      })
      if (!res.ok) return null

      const body = (await res.json()) as { token?: unknown }
      if (typeof body.token !== 'string' || body.token === '') return null

      localStorage.setItem(TOKEN_STORAGE_KEY, body.token)
      window.dispatchEvent(new CustomEvent<string>(tokenRefreshedEvent, { detail: body.token }))
      return body.token
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
  refreshed: boolean,
): Promise<T> {
  const res = await fetch(`${API_BASE_URL}${path}`, { ...init, credentials: init?.credentials ?? 'include' })

  if (res.status === 401 && !refreshed && path !== '/api/profile/refresh' && hasBearerToken(init)) {
    const accessToken = await refreshAccessToken()
    if (accessToken) {
      const headers = new Headers(init?.headers)
      headers.set('Authorization', `Bearer ${accessToken}`)
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

export { sessionExpiredEvent, tokenRefreshedEvent }
