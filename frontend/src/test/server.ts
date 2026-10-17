import { http, HttpResponse } from 'msw'
import { setupServer } from 'msw/node'
import { API_BASE_URL } from '@/src/api/client'

export const api = (path: string) => `${API_BASE_URL}${path}`

// Defaults every test gets; tests override with server.use.
export const server = setupServer(
  http.post(api('/api/profile/refresh'), () => HttpResponse.json({ error: 'no session' }, { status: 401 })),
  http.post(api('/api/profile/logout'), () => new HttpResponse(null, { status: 204 })),
  http.get(api('/api/tags'), () => HttpResponse.json(['go', 'react', 'sql']))
)
