import { vi } from 'vitest'
import type { ReactNode } from 'react'
import type { ProfileAuthContextValue } from '@/src/stores/auth-store'

// Stand-in for the auth store; use with vi.mock('@/src/stores/auth-store', () => import('@/src/test/auth-mock')).
export const auth: ProfileAuthContextValue = {
  token: null,
  isAuthenticated: false,
  isInitializing: false,
  sessionMessage: null,
  login: vi.fn(),
  logout: vi.fn()
}

export function signIn(token = 'test-token') {
  Object.assign(auth, { token, isAuthenticated: true, isInitializing: false })
}

export function resetAuth() {
  Object.assign(auth, {
    token: null,
    isAuthenticated: false,
    isInitializing: false,
    sessionMessage: null,
    login: vi.fn(),
    logout: vi.fn()
  })
}

export function useAuth(): ProfileAuthContextValue {
  return auth
}

export function ProfileAuthProvider({ children }: { children: ReactNode }) {
  return children
}
