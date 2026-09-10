import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { API_BASE_URL, refreshAccessToken, sessionExpiredEvent, tokenRefreshedEvent } from '../api/client'

interface ProfileAuthContextValue {
  token: string | null
  isAuthenticated: boolean
  isInitializing: boolean
  sessionMessage: string | null
  login: (token: string) => void
  logout: (message?: string) => void
}

const ProfileAuthContext = createContext<ProfileAuthContextValue | undefined>(undefined)

export function ProfileAuthProvider({ children }: { children: ReactNode }) {
  // Access token lives in memory only — the httpOnly refresh cookie is what
  // actually persists the session, so a reload re-derives this instead of
  // reading a stale copy back out of localStorage.
  const [token, setToken] = useState<string | null>(null)
  const [isInitializing, setIsInitializing] = useState(true)
  const [sessionMessage, setSessionMessage] = useState<string | null>(null)

  const login = useCallback((next: string) => {
    setToken(next)
    setSessionMessage(null)
  }, [])

  const logout = useCallback((message?: string) => {
    void fetch(`${API_BASE_URL}/api/profile/logout`, { method: 'POST', credentials: 'include' })
    setToken(null)
    setSessionMessage(message ?? null)
  }, [])

  useEffect(() => {
    let cancelled = false

    refreshAccessToken().then((next) => {
      if (cancelled) return
      if (next) setToken(next)
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
    [token, isInitializing, sessionMessage, login, logout],
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
