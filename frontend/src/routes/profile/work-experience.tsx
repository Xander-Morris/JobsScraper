import { createFileRoute } from '@tanstack/react-router'
import { WorkExperienceSection } from '@/src/components/profile/work-experience-section'
import { useProfileOutletContext } from '@/src/components/profile/profile-context'

export const Route = createFileRoute('/profile/work-experience')({ component: RouteComponent })

function RouteComponent() {
  const { profile } = useProfileOutletContext()

  return <WorkExperienceSection workExperience={profile.work_experience ?? []} />
}
