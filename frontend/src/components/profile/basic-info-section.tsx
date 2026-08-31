import { useId, useState, type FormEvent } from 'react'
import { useUpdateProfileMutation } from '@/src/api/profile'
import type { Profile } from '@/src/api/schemas'
import { Button } from '@/src/components/ui/button'
import { Card, CardContent, CardFooter, CardHeader } from '@/src/components/ui/card'
import { Input } from '@/src/components/ui/input'
import { Label } from '@/src/components/ui/label'
import { Switch } from '@/src/components/ui/switch'

export function BasicInfoSection({ token, profile }: { token: string; profile: Profile }) {
  const updateProfile = useUpdateProfileMutation(token)
  const [name, setName] = useState(profile.name)
  const [address, setAddress] = useState(profile.address)
  const [linkedIn, setLinkedIn] = useState(profile.linked_in)
  const [github, setGithub] = useState(profile.github)
  const [portfolio, setPortfolio] = useState(profile.portfolio)
  const [emailNotifications, setEmailNotifications] = useState(profile.email_notifications)
  const id = useId()

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    updateProfile.mutate({ name, address, linked_in: linkedIn, github, portfolio, email_notifications: emailNotifications })
  }

  return (
    <Card>
      <CardHeader><h3 className="text-sm font-semibold text-heading">Basic info</h3></CardHeader>
      <form onSubmit={handleSubmit}>
        <CardContent className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5"><Label htmlFor={`${id}-name`}>Name</Label><Input id={`${id}-name`} value={name} onChange={(e) => setName(e.target.value)} /></div>
          <div className="space-y-1.5"><Label htmlFor={`${id}-address`}>Address</Label><Input id={`${id}-address`} value={address} onChange={(e) => setAddress(e.target.value)} /></div>
          <div className="space-y-1.5"><Label htmlFor={`${id}-linkedin`}>LinkedIn URL</Label><Input id={`${id}-linkedin`} type="url" value={linkedIn} onChange={(e) => setLinkedIn(e.target.value)} /></div>
          <div className="space-y-1.5"><Label htmlFor={`${id}-github`}>GitHub URL</Label><Input id={`${id}-github`} type="url" value={github} onChange={(e) => setGithub(e.target.value)} /></div>
          <div className="col-span-2 space-y-1.5"><Label htmlFor={`${id}-portfolio`}>Portfolio URL</Label><Input id={`${id}-portfolio`} type="url" value={portfolio} onChange={(e) => setPortfolio(e.target.value)} /></div>
          <div className="col-span-2 flex items-center justify-between gap-3 rounded-lg border border-border p-3">
            <div className="pointer-events-none space-y-0.5">
              <Label htmlFor={`${id}-email-notifications`}>Email me matching jobs</Label>
              <p className="text-xs text-muted-foreground">A daily digest of new postings that fit your active resume.</p>
            </div>
            <Switch id={`${id}-email-notifications`} checked={emailNotifications} onCheckedChange={setEmailNotifications} />
          </div>
        </CardContent>
        <CardFooter><Button type="submit" disabled={updateProfile.isPending}>Save</Button></CardFooter>
      </form>
    </Card>
  )
}
