import { createContext, useContext, type ReactNode } from 'react'
import type { Profile } from '@/src/api/schemas'

interface ProfileOutletContextValue {
  profile: Profile
}

const ProfileOutletContext = createContext<ProfileOutletContextValue | null>(null)

export function ProfileOutletProvider({ profile, children }: ProfileOutletContextValue & { children: ReactNode }) {
  return <ProfileOutletContext.Provider value={{ profile }}>{children}</ProfileOutletContext.Provider>
}

export function useProfileOutletContext(): ProfileOutletContextValue {
  const ctx = useContext(ProfileOutletContext)

  if (!ctx) {
    throw new Error('useProfileOutletContext must be used within a /profile route')
  }

  return ctx
}
