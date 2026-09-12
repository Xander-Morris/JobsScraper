import { createFileRoute } from '@tanstack/react-router'
import { SkillsSection } from '@/src/components/profile/skills-section'
import { useProfileOutletContext } from '@/src/components/profile/profile-context'

export const Route = createFileRoute('/profile/skills')({ component: RouteComponent })

function RouteComponent() {
  const { profile } = useProfileOutletContext()

  return <SkillsSection skills={profile.skills ?? []} />
}
