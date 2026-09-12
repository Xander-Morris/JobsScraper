import { createFileRoute } from '@tanstack/react-router'
import { EducationSection } from '@/src/components/profile/education-section'
import { useProfileOutletContext } from '@/src/components/profile/profile-context'

export const Route = createFileRoute('/profile/education')({ component: RouteComponent })

function RouteComponent() {
  const { profile } = useProfileOutletContext()

  return <EducationSection education={profile.education ?? []} />
}
