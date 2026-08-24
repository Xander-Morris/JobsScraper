import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { API_BASE_URL, sessionExpiredEvent, tokenRefreshedEvent } from '../api/client'

const TOKEN_STORAGE_KEY = 'profile_token'

interface ProfileAuthContextValue {
  token: string | null
  isAuthenticated: boolean
  sessionMessage: string | null
  login: (token: string) => void
  logout: (message?: string) => void
}

const ProfileAuthContext = createContext<ProfileAuthContextValue | undefined>(undefined)

export function ProfileAuthProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null>(() => localStorage.getItem(TOKEN_STORAGE_KEY))
  const [sessionMessage, setSessionMessage] = useState<string | null>(null)

  const login = useCallback((next: string) => {
    localStorage.setItem(TOKEN_STORAGE_KEY, next)
    setToken(next)
    setSessionMessage(null)
  }, [])

  const logout = useCallback((message?: string) => {
    void fetch(`${API_BASE_URL}/api/profile/logout`, { method: 'POST', credentials: 'include' })
    localStorage.removeItem(TOKEN_STORAGE_KEY)
    setToken(null)
    setSessionMessage(message ?? null)
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
    () => ({ token, isAuthenticated: token !== null, sessionMessage, login, logout }),
    [token, sessionMessage, login, logout],
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
