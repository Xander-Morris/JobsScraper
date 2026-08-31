import { createFileRoute } from '@tanstack/react-router'
import { BasicInfoSection } from '@/src/components/profile/basic-info-section'
import { useProfileOutletContext } from '@/src/components/profile/profile-context'

export const Route = createFileRoute('/profile/basic')({ component: RouteComponent })

function RouteComponent() {
  const { token, profile } = useProfileOutletContext()

  return <BasicInfoSection token={token} profile={profile} />
}
