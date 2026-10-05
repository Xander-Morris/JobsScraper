import { useUpdateProfileMutation } from '@/src/hooks/use-profile'
import { useAppForm } from '@/src/hooks/use-app-form'
import { useAutosave, type AutosaveStatus } from '@/src/hooks/use-autosave'
import type { UpdateProfileRequest } from '@/src/api/profile'
import type { Profile } from '@/src/api/schemas'
import { cn } from '@/src/lib/utils'
import { Card, CardContent, CardFooter, CardHeader } from '@/src/components/ui/card'
import { Input } from '@/src/components/ui/input'
import { Label } from '@/src/components/ui/label'
import { Switch } from '@/src/components/ui/switch'
import { AddressAutofill } from '@mapbox/search-js-react'
import { useId } from 'react'
import { z } from 'zod'

const optionalUrl = z.union([z.literal(''), z.url('Enter a valid URL')])

const basicInfoSchema = z.object({
  name: z.string(),
  address: z.string(),
  linked_in: optionalUrl,
  github: optionalUrl,
  portfolio: optionalUrl,
  email_notifications: z.boolean()
})

const statusText: Record<AutosaveStatus, string> = {
  idle: '',
  saving: 'Saving…',
  saved: 'All changes saved',
  retrying: "Couldn't save, retrying…",
  error: "Couldn't save changes"
}

export function BasicInfoSection({ profile }: { profile: Profile }) {
  const updateProfile = useUpdateProfileMutation()
  const id = useId()

  // Reset once saved so later profile refetches (e.g. applying a resume) show up in the form again.
  // Skipped if the form has moved on to unsaved invalid input, which reset would wipe.
  const autosave = useAutosave(updateProfile.mutateAsync, (value: UpdateProfileRequest) => {
    const current = form.state.values
    if ((Object.keys(value) as (keyof UpdateProfileRequest)[]).every((key) => current[key] === value[key])) {
      form.reset(value)
    }
  })

  const form = useAppForm({
    defaultValues: {
      name: profile.name,
      address: profile.address,
      linked_in: profile.linked_in,
      github: profile.github,
      portfolio: profile.portfolio,
      email_notifications: profile.email_notifications
    },
    validators: { onChange: basicInfoSchema },
    listeners: {
      onChange: ({ formApi }) => {
        const parsed = basicInfoSchema.safeParse(formApi.state.values)
        if (parsed.success) autosave.queue(parsed.data)
      }
    }
  })

  return (
    <Card>
      <CardHeader>
        <h3 className="text-sm font-semibold text-heading">Basic info</h3>
      </CardHeader>
      <form.AppForm>
        <form.Form>
          <CardContent className="grid gap-3 sm:grid-cols-2">
            <form.AppField name="name">{(f) => <f.TextField label="Name" />}</form.AppField>
            <form.Field name="address">
              {(f) => (
                <div className="space-y-1.5">
                  <Label htmlFor={`${id}-address`}>Address</Label>
                  <AddressAutofill accessToken={import.meta.env.VITE_MAPBOX_TOKEN ?? ''}>
                    <Input
                      id={`${id}-address`}
                      name={f.name}
                      autoComplete="street-address"
                      value={f.state.value}
                      onChange={(e) => f.handleChange(e.target.value)}
                      onBlur={f.handleBlur}
                    />
                  </AddressAutofill>
                </div>
              )}
            </form.Field>
            <form.AppField name="linked_in">{(f) => <f.TextField label="LinkedIn URL" type="url" />}</form.AppField>
            <form.AppField name="github">{(f) => <f.TextField label="GitHub URL" type="url" />}</form.AppField>
            <form.AppField name="portfolio">
              {(f) => <f.TextField label="Portfolio URL" type="url" className="sm:col-span-2" />}
            </form.AppField>
            <form.Field name="email_notifications">
              {(f) => (
                <div className="flex items-center sm:col-span-2 justify-between gap-3 rounded-lg border border-border p-3">
                  <div className="pointer-events-none space-y-0.5">
                    <Label htmlFor={`${id}-email-notifications`}>Email me matching jobs</Label>
                    <p className="text-xs text-muted-foreground">
                      A daily digest of new postings that fit your active resume.
                    </p>
                  </div>
                  <Switch
                    id={`${id}-email-notifications`}
                    checked={f.state.value}
                    onCheckedChange={(checked) => f.handleChange(checked)}
                  />
                </div>
              )}
            </form.Field>
          </CardContent>
          <CardFooter className="mt-(--card-spacing)">
            <form.Subscribe selector={(state) => state.isValid}>
              {(isValid) => (
                <p
                  aria-live="polite"
                  className={cn(
                    'text-xs text-muted-foreground',
                    (!isValid || autosave.status === 'error') && 'text-destructive'
                  )}
                >
                  {isValid ? statusText[autosave.status] : 'Fix the highlighted fields to save'}
                </p>
              )}
            </form.Subscribe>
          </CardFooter>
        </form.Form>
      </form.AppForm>
    </Card>
  )
}
