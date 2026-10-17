import { http, HttpResponse } from 'msw'
import { describe, expect, it, vi } from 'vitest'
import { z } from 'zod'
import { api, server } from '@/src/test/server'
import {
  ApiError,
  apiFetch,
  apiFetchBlob,
  refreshAccessToken,
  sessionExpiredEvent,
  setAccessToken,
  tokenRefreshedEvent
} from './client'

const okSchema = z.object({ ok: z.boolean() })

describe('apiFetch', () => {
  it('parses the body with the schema and sends credentials', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch')
    server.use(http.get(api('/api/thing'), () => HttpResponse.json({ ok: true, extra: 'dropped' })))

    await expect(apiFetch('/api/thing', okSchema)).resolves.toEqual({ ok: true })
    expect(fetchSpy.mock.calls[0][1]).toMatchObject({ credentials: 'include' })
  })

  it('rejects a body that fails the schema', async () => {
    server.use(http.get(api('/api/thing'), () => HttpResponse.json({ ok: 'yes' })))

    await expect(apiFetch('/api/thing', okSchema)).rejects.toThrow(z.ZodError)
  })

  it('throws ApiError with the error field or status text', async () => {
    server.use(
      http.get(api('/api/json-error'), () => HttpResponse.json({ error: 'job not found' }, { status: 404 })),
      http.get(
        api('/api/text-error'),
        () => new HttpResponse('oops', { status: 500, statusText: 'Internal Server Error' })
      )
    )

    const jsonErr = await apiFetch('/api/json-error', okSchema).catch((e: unknown) => e)
    expect(jsonErr).toBeInstanceOf(ApiError)
    expect(jsonErr).toMatchObject({ message: 'job not found', status: 404, name: 'ApiError' })

    await expect(apiFetch('/api/text-error', okSchema)).rejects.toMatchObject({
      message: 'Internal Server Error',
      status: 500
    })
  })

  it('attaches the access token, but keeps a caller Authorization header', async () => {
    const seen: (string | null)[] = []
    server.use(
      http.get(api('/api/thing'), ({ request }) => {
        seen.push(request.headers.get('Authorization'))
        return HttpResponse.json({ ok: true })
      })
    )

    await apiFetch('/api/thing', okSchema)
    setAccessToken('abc')
    await apiFetch('/api/thing', okSchema)
    await apiFetch('/api/thing', okSchema, { headers: { Authorization: 'Bearer custom' } })

    expect(seen).toEqual([null, 'Bearer abc', 'Bearer custom'])
  })

  it('refreshes once on a 401 and retries with the new token', async () => {
    setAccessToken('stale')
    const refreshed = vi.fn()
    window.addEventListener(tokenRefreshedEvent, refreshed)

    server.use(
      http.post(api('/api/profile/refresh'), () => HttpResponse.json({ token: 'fresh' })),
      http.get(api('/api/thing'), ({ request }) =>
        request.headers.get('Authorization') === 'Bearer fresh'
          ? HttpResponse.json({ ok: true })
          : HttpResponse.json({ error: 'expired' }, { status: 401 })
      )
    )

    await expect(apiFetch('/api/thing', okSchema)).resolves.toEqual({ ok: true })
    expect(refreshed).toHaveBeenCalledOnce()
    expect((refreshed.mock.calls[0][0] as CustomEvent<string>).detail).toBe('fresh')

    window.removeEventListener(tokenRefreshedEvent, refreshed)
  })

  it('dedupes concurrent refreshes', async () => {
    setAccessToken('stale')
    let refreshes = 0

    server.use(
      http.post(api('/api/profile/refresh'), async () => {
        refreshes++
        await new Promise((r) => setTimeout(r, 10))
        return HttpResponse.json({ token: 'fresh' })
      }),
      http.get(api('/api/thing'), ({ request }) =>
        request.headers.get('Authorization') === 'Bearer fresh'
          ? HttpResponse.json({ ok: true })
          : HttpResponse.json({}, { status: 401 })
      )
    )

    await Promise.all([
      apiFetch('/api/thing', okSchema),
      apiFetch('/api/thing', okSchema),
      apiFetch('/api/thing', okSchema)
    ])
    expect(refreshes).toBe(1)
  })

  it('dispatches session-expired when the refresh fails', async () => {
    setAccessToken('stale')
    const expired = vi.fn()
    window.addEventListener(sessionExpiredEvent, expired)

    server.use(http.get(api('/api/thing'), () => HttpResponse.json({ error: 'expired' }, { status: 401 })))

    await expect(apiFetch('/api/thing', okSchema)).rejects.toMatchObject({ status: 401 })
    expect(expired).toHaveBeenCalledOnce()

    window.removeEventListener(sessionExpiredEvent, expired)
  })

  it('does not refresh anonymous 401s', async () => {
    let refreshes = 0
    server.use(
      http.post(api('/api/profile/refresh'), () => {
        refreshes++
        return HttpResponse.json({ token: 'x' })
      }),
      http.post(api('/api/profile/login'), () =>
        HttpResponse.json({ error: 'invalid email or password' }, { status: 401 })
      )
    )

    await expect(apiFetch('/api/profile/login', okSchema, { method: 'POST' })).rejects.toMatchObject({
      message: 'invalid email or password'
    })
    expect(refreshes).toBe(0)
  })
})

describe('refreshAccessToken', () => {
  it.each([
    ['a non-2xx', () => HttpResponse.json({}, { status: 401 })],
    ['an empty token', () => HttpResponse.json({ token: '' })],
    ['a non-string token', () => HttpResponse.json({ token: 42 })],
    ['a network error', () => HttpResponse.error()]
  ])('returns null on %s', async (_name, resolver) => {
    server.use(http.post(api('/api/profile/refresh'), resolver))

    await expect(refreshAccessToken()).resolves.toBeNull()
  })
})

describe('apiFetchBlob', () => {
  it('returns the raw body', async () => {
    server.use(
      http.get(api('/api/file'), () => new HttpResponse('PDFDATA', { headers: { 'Content-Type': 'application/pdf' } }))
    )

    const blob = await apiFetchBlob('/api/file')
    expect(await blob.text()).toBe('PDFDATA')
  })

  it('throws ApiError on failure', async () => {
    server.use(http.get(api('/api/file'), () => HttpResponse.json({ error: 'resume not found' }, { status: 404 })))

    await expect(apiFetchBlob('/api/file')).rejects.toMatchObject({ message: 'resume not found', status: 404 })
  })
})
