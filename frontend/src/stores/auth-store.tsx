import { useQueryClient } from '@tanstack/react-query'
import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import {
  API_BASE_URL,
  refreshAccessToken,
  sessionExpiredEvent,
  setAccessToken,
  tokenRefreshedEvent
} from '../api/client'
import { queryKeys } from '../api/query-keys'

const authChannelName = 'profile-auth'

// Started at import so the refresh overlaps app boot instead of waiting for first render.
const initialRefresh = refreshAccessToken()

type AuthMessage = { type: 'login'; token: string } | { type: 'logout' }

export interface ProfileAuthContextValue {
  token: string | null
  isAuthenticated: boolean
  isInitializing: boolean
  sessionMessage: string | null
  login: (token: string) => void
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

  const login = useCallback((next: string) => {
    setAccessToken(next)
    setToken(next)
    setSessionMessage(null)
    channelRef.current?.postMessage({ type: 'login', token: next } satisfies AuthMessage)
  }, [])

  // Jobs keys don't include the token, so drop per-user data (applied, match score) on logout.
  const clearUserJobs = useCallback(() => queryClient.removeQueries({ queryKey: queryKeys.jobs }), [queryClient])

  const logout = useCallback(
    (message?: string) => {
      void fetch(`${API_BASE_URL}/api/profile/logout`, { method: 'POST', credentials: 'include' })
      setAccessToken(null)
      setToken(null)
      clearUserJobs()
      setSessionMessage(message ?? null)
      channelRef.current?.postMessage({ type: 'logout' } satisfies AuthMessage)
    },
    [clearUserJobs]
  )

  useEffect(() => {
    if (typeof BroadcastChannel === 'undefined') return

    const channel = new BroadcastChannel(authChannelName)
    channelRef.current = channel

    channel.onmessage = (event: MessageEvent<AuthMessage>) => {
      if (event.data.type === 'login') {
        setAccessToken(event.data.token)
        setToken(event.data.token)
        setSessionMessage(null)
      } else {
        setAccessToken(null)
        setToken(null)
        clearUserJobs()
      }
    }

    return () => {
      channel.close()
      channelRef.current = null
    }
  }, [clearUserJobs])

  useEffect(() => {
    let cancelled = false

    initialRefresh.then((next) => {
      if (cancelled) return
      if (next) {
        setAccessToken(next)
        setToken(next)
      }
      setIsInitializing(false)
    })

    return () => {
      cancelled = true
    }
  }, [])

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
