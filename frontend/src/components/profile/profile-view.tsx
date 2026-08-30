import { ApiError } from '@/src/api/client'
import { useProfileQuery } from '@/src/api/profile'
import { useAuth } from '@/src/stores/profile-store'
import { useEffect } from 'react'
import { Button } from '../ui/button'
import { BasicInfoSection } from './basic-info-section'
import { EducationSection } from './education-section'
import { ResumesSection } from './resumes-section'
import { SkillsSection } from './skills-section'
import { WorkExperienceSection } from './work-experience-section'

interface ProfileViewProps {
  token: string
}

export default function ProfileView({ token }: ProfileViewProps) {
  const { logout } = useAuth()
  const { data: profile, error, isLoading, isError } = useProfileQuery(token)
  const isInvalidSession = error instanceof ApiError && (error.status === 401 || error.status === 404)

  useEffect(() => {
    if (isInvalidSession) {
      logout('Your session expired or is no longer valid. Please log in again.')
    }
  }, [isInvalidSession, logout])

  if (isInvalidSession) return <p className="mt-10 text-muted-foreground">Signing you out...</p>

  if (isError) {
    return <p role="alert" className="mt-10 text-muted-foreground">Unable to load your profile. Please try again.</p>
  }

  if (isLoading || !profile) return <p className="mt-10 text-muted-foreground">Loading profile…</p>

  return (
    <div className="mx-auto mt-8 max-w-2xl space-y-6 text-left mb-4">
      <div className="flex items-center justify-between">
        <h2>{profile.email}</h2>
        <Button type="button" variant="ghost" size="sm" onClick={() => logout()}>
          Log out
        </Button>
      </div>

      <BasicInfoSection token={token} profile={profile} />
      <EducationSection token={token} education={profile.education ?? []} />
      <SkillsSection token={token} skills={profile.skills ?? []} />
      <WorkExperienceSection token={token} workExperience={profile.work_experience ?? []} />
      <ResumesSection token={token} resumes={profile.resumes ?? []} profile={profile} />
    </div>
  )
}
