import { createFileRoute } from '@tanstack/react-router'
import { SkillsSection } from '@/src/components/profile/skills-section'
import { useProfileOutletContext } from '@/src/components/profile/profile-context'

export const Route = createFileRoute('/profile/skills')({ component: RouteComponent })

function RouteComponent() {
  const { token, profile } = useProfileOutletContext()

  return <SkillsSection token={token} skills={profile.skills ?? []} />
}
