import { useQueryClient } from '@tanstack/react-query'
import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import {
  API_BASE_URL,
  refreshAccessToken,
  sessionExpiredEvent,
  setAccessToken,
  tokenRefreshedEvent
} from '../api/client'

const authChannelName = 'profile-auth'
const presenceChannelName = 'profile-presence'
const rememberKey = 'profile-remember'
const liveKey = 'profile-session-live'

const local = () => localStorage
const session = () => sessionStorage

function readStorage(storage: () => Storage, key: string): string | null {
  try {
    return storage().getItem(key)
  } catch {
    return null
  }
}

function writeStorage(storage: () => Storage, key: string, value: string | null) {
  try {
    if (value === null) storage().removeItem(key)
    else storage().setItem(key, value)
  } catch {
    // storage blocked
  }
}

// Open tabs holding a live session answer pings, so a new tab can tell whether the user left the site.
const presence = typeof BroadcastChannel === 'undefined' ? null : new BroadcastChannel(presenceChannelName)
presence?.addEventListener('message', (event: MessageEvent) => {
  if (event.data === 'ping' && readStorage(session, liveKey)) presence.postMessage('pong')
})

function otherTabLive(): Promise<boolean> {
  const channel = presence
  if (!channel) return Promise.resolve(false)

  return new Promise((resolve) => {
    const onMessage = (event: MessageEvent) => {
      if (event.data === 'pong') done(true)
    }
    const timer = setTimeout(() => done(false), 300)
    const done = (live: boolean) => {
      clearTimeout(timer)
      channel.removeEventListener('message', onMessage)
      resolve(live)
    }
    channel.addEventListener('message', onMessage)
    channel.postMessage('ping')
  })
}

// Without "remember me", a session only survives reloads and other open tabs, not leaving the site.
async function restoreSession(): Promise<string | null> {
  const mayForget = readStorage(local, rememberKey) === 'false' && !readStorage(session, liveKey)
  if (mayForget && !(await otherTabLive())) {
    writeStorage(local, rememberKey, null)
    await fetch(`${API_BASE_URL}/api/profile/logout`, { method: 'POST', credentials: 'include' }).catch(() => {})
    return null
  }
  return refreshAccessToken()
}

// Started at import so the refresh overlaps app boot instead of waiting for first render.
const initialRefresh = restoreSession()

type AuthMessage = { type: 'login'; token: string } | { type: 'logout' }

export interface ProfileAuthContextValue {
  token: string | null
  isAuthenticated: boolean
  isInitializing: boolean
  sessionMessage: string | null
  // Pass remember only on an explicit login; token refreshes leave it as is.
  login: (token: string, remember?: boolean) => void
  logout: (message?: string) => void
}

const ProfileAuthContext = createContext<ProfileAuthContextValue | undefined>(undefined)

export function ProfileAuthProvider({ children }: { children: ReactNode }) {
  // Access token lives in memory only; the httpOnly refresh cookie is what
  // actually persists the session, so a reload re-derives this instead of
  // reading a stale copy back out of localStorage.
  const [token, setToken] = useState<string | null>(null)
  const [isInitializing, setIsInitializing] = useState(true)
  const [sessionMessage, setSessionMessage] = useState<string | null>(null)
  // Keeps other tabs in sync, since each one holds its own in-memory token.
  const channelRef = useRef<BroadcastChannel | null>(null)
  const queryClient = useQueryClient()

  // Query keys don't include the user, so drop every cached response on logout.
  const applyToken = useCallback(
    (next: string | null) => {
      writeStorage(session, liveKey, next ? '1' : null)
      setAccessToken(next)
      setToken(next)
      if (!next) queryClient.clear()
    },
    [queryClient]
  )

  const login = useCallback(
    (next: string, remember?: boolean) => {
      if (remember !== undefined) writeStorage(local, rememberKey, String(remember))
      applyToken(next)
      setSessionMessage(null)
      channelRef.current?.postMessage({ type: 'login', token: next } satisfies AuthMessage)
    },
    [applyToken]
  )

  const logout = useCallback(
    (message?: string) => {
      void fetch(`${API_BASE_URL}/api/profile/logout`, { method: 'POST', credentials: 'include' })
      writeStorage(local, rememberKey, null)
      applyToken(null)
      setSessionMessage(message ?? null)
      channelRef.current?.postMessage({ type: 'logout' } satisfies AuthMessage)
    },
    [applyToken]
  )

  useEffect(() => {
    if (typeof BroadcastChannel === 'undefined') return

    const channel = new BroadcastChannel(authChannelName)
    channelRef.current = channel

    channel.onmessage = (event: MessageEvent<AuthMessage>) => {
      if (event.data.type === 'login') {
        applyToken(event.data.token)
        setSessionMessage(null)
      } else {
        applyToken(null)
      }
    }

    return () => {
      channel.close()
      channelRef.current = null
    }
  }, [applyToken])

  useEffect(() => {
    let cancelled = false

    initialRefresh.then((next) => {
      if (cancelled) return
      if (next) applyToken(next)
      setIsInitializing(false)
    })

    return () => {
      cancelled = true
    }
  }, [applyToken])

  useEffect(() => {
    const onTokenRefreshed = (event: Event) => {
      const token = (event as CustomEvent<string>).detail
      if (token) login(token)
    }
    const onSessionExpired = () => logout('Your session expired or is no longer valid. Please log in again.')

    window.addEventListener(tokenRefreshedEvent, onTokenRefreshed)
    window.addEventListener(sessionExpiredEvent, onSessionExpired)
    return () => {
      window.removeEventListener(tokenRefreshedEvent, onTokenRefreshed)
      window.removeEventListener(sessionExpiredEvent, onSessionExpired)
    }
  }, [login, logout])

  const value = useMemo(
    () => ({ token, isAuthenticated: token !== null, isInitializing, sessionMessage, login, logout }),
    [token, isInitializing, sessionMessage, login, logout]
  )

  return <ProfileAuthContext.Provider value={value}>{children}</ProfileAuthContext.Provider>
}

export function useAuth(): ProfileAuthContextValue {
  const ctx = useContext(ProfileAuthContext)

  if (!ctx) {
    throw new Error('useAuth must be used within a ProfileAuthProvider')
  }

  return ctx
}
