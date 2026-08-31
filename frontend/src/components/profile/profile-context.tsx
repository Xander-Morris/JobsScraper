import { createContext, useContext, type ReactNode } from 'react'
import type { Profile } from '@/src/api/schemas'

interface ProfileOutletContextValue {
  token: string
  profile: Profile
}

const ProfileOutletContext = createContext<ProfileOutletContextValue | null>(null)

export function ProfileOutletProvider({ token, profile, children }: ProfileOutletContextValue & { children: ReactNode }) {
  return <ProfileOutletContext.Provider value={{ token, profile }}>{children}</ProfileOutletContext.Provider>
}

export function useProfileOutletContext(): ProfileOutletContextValue {
  const ctx = useContext(ProfileOutletContext)

  if (!ctx) {
    throw new Error('useProfileOutletContext must be used within a /profile route')
  }

  return ctx
}
