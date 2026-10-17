import { QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook, waitFor } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import type { ReactNode } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createTestQueryClient } from '@/src/test/render'
import { api, server } from '@/src/test/server'

// The store starts restoring the session at import, so each test loads a fresh copy.
async function loadStore() {
  vi.resetModules()
  const store = await import('./auth-store')
  const client = await import('../api/client')
  return { ...store, client }
}

async function renderAuth() {
  const mod = await loadStore()
  const queryClient = createTestQueryClient()
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>
      <mod.ProfileAuthProvider>{children}</mod.ProfileAuthProvider>
    </QueryClientProvider>
  )

  const hook = renderHook(() => mod.useAuth(), { wrapper })
  await waitFor(() => expect(hook.result.current.isInitializing).toBe(false))

  return { ...hook, queryClient, mod }
}

let logouts: number
let refreshes: number

beforeEach(() => {
  logouts = 0
  refreshes = 0
  server.use(
    http.post(api('/api/profile/logout'), () => {
      logouts++
      return new HttpResponse(null, { status: 204 })
    })
  )
})

function refreshWith(token: string | null) {
  server.use(
    http.post(api('/api/profile/refresh'), () => {
      refreshes++
      return token ? HttpResponse.json({ token }) : HttpResponse.json({ error: 'no session' }, { status: 401 })
    })
  )
}

describe('ProfileAuthProvider', () => {
  it('throws when useAuth is used outside the provider', async () => {
    const { useAuth } = await loadStore()
    vi.spyOn(console, 'error').mockImplementation(() => {})

    expect(() => renderHook(() => useAuth())).toThrow('useAuth must be used within a ProfileAuthProvider')
  })

  it('starts signed out when there is no session', async () => {
    refreshWith(null)
    const { result } = await renderAuth()

    expect(result.current).toMatchObject({ token: null, isAuthenticated: false, sessionMessage: null })
  })

  it('restores a session from the refresh cookie', async () => {
    refreshWith('restored')
    const { result } = await renderAuth()

    expect(result.current).toMatchObject({ token: 'restored', isAuthenticated: true })
    expect(sessionStorage.getItem('profile-session-live')).toBe('1')
  })

  it('logs in and records remember me', async () => {
    refreshWith(null)
    const { result } = await renderAuth()

    act(() => result.current.login('new-token', false))

    expect(result.current.token).toBe('new-token')
    expect(localStorage.getItem('profile-remember')).toBe('false')
    expect(sessionStorage.getItem('profile-session-live')).toBe('1')

    // A refresh login leaves remember alone.
    act(() => result.current.login('rotated'))
    expect(localStorage.getItem('profile-remember')).toBe('false')
  })

  it('logs out, clears the cache, and shows the message', async () => {
    refreshWith('restored')
    const { result, queryClient } = await renderAuth()
    queryClient.setQueryData(['profile'], { secret: true })
    localStorage.setItem('profile-remember', 'true')

    act(() => result.current.logout('Bye'))

    expect(result.current).toMatchObject({ token: null, isAuthenticated: false, sessionMessage: 'Bye' })
    expect(queryClient.getQueryData(['profile'])).toBeUndefined()
    expect(localStorage.getItem('profile-remember')).toBeNull()
    expect(sessionStorage.getItem('profile-session-live')).toBeNull()
    await waitFor(() => expect(logouts).toBe(1))
  })

  it('logs out with a message when the session expires', async () => {
    refreshWith('restored')
    const { result, mod } = await renderAuth()

    act(() => {
      window.dispatchEvent(new Event(mod.client.sessionExpiredEvent))
    })

    expect(result.current.isAuthenticated).toBe(false)
    expect(result.current.sessionMessage).toMatch(/session expired/i)
  })

  it('picks up tokens refreshed by the api client', async () => {
    refreshWith(null)
    const { result, mod } = await renderAuth()

    act(() => {
      window.dispatchEvent(new CustomEvent(mod.client.tokenRefreshedEvent, { detail: 'from-client' }))
    })

    expect(result.current.token).toBe('from-client')
  })

  it('forgets a non-remembered session once every tab has closed', async () => {
    refreshWith('should-not-be-used')
    localStorage.setItem('profile-remember', 'false')

    const { result } = await renderAuth()

    expect(result.current.isAuthenticated).toBe(false)
    expect(refreshes).toBe(0)
    expect(logouts).toBe(1)
    expect(localStorage.getItem('profile-remember')).toBeNull()
  })

  it('keeps a non-remembered session across a reload', async () => {
    refreshWith('kept')
    localStorage.setItem('profile-remember', 'false')
    sessionStorage.setItem('profile-session-live', '1')

    const { result } = await renderAuth()

    expect(result.current.token).toBe('kept')
    expect(logouts).toBe(0)
  })
})
