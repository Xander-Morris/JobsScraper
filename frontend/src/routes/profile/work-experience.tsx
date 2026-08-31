import { createFileRoute } from '@tanstack/react-router'
import { WorkExperienceSection } from '@/src/components/profile/work-experience-section'
import { useProfileOutletContext } from '@/src/components/profile/profile-context'

export const Route = createFileRoute('/profile/work-experience')({ component: RouteComponent })

function RouteComponent() {
  const { token, profile } = useProfileOutletContext()

  return <WorkExperienceSection token={token} workExperience={profile.work_experience ?? []} />
}
