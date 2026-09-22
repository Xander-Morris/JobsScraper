import { useUpdateProfileMutation } from '@/src/hooks/use-profile'
import { useAppForm } from '@/src/hooks/use-app-form'
import type { Profile } from '@/src/api/schemas'
import { Card, CardContent, CardFooter, CardHeader } from '@/src/components/ui/card'
import { Input } from '@/src/components/ui/input'
import { Label } from '@/src/components/ui/label'
import { Switch } from '@/src/components/ui/switch'
import { AddressAutofill } from '@mapbox/search-js-react'
import { revalidateLogic } from '@tanstack/react-form'
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

export function BasicInfoSection({ profile }: { profile: Profile }) {
  const updateProfile = useUpdateProfileMutation()
  const id = useId()

  const form = useAppForm({
    defaultValues: {
      name: profile.name,
      address: profile.address,
      linked_in: profile.linked_in,
      github: profile.github,
      portfolio: profile.portfolio,
      email_notifications: profile.email_notifications
    },
    validationLogic: revalidateLogic(),
    validators: { onDynamic: basicInfoSchema },
    onSubmit: async ({ value }) => {
      await updateProfile.mutateAsync(value)
    }
  })

  return (
    <Card>
      <CardHeader>
        <h3 className="text-sm font-semibold text-heading">Basic info</h3>
      </CardHeader>
      <form.AppForm>
        <form.Form>
          <CardContent className="grid grid-cols-2 gap-3">
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
              {(f) => <f.TextField label="Portfolio URL" type="url" className="col-span-2" />}
            </form.AppField>
            <form.Field name="email_notifications">
              {(f) => (
                <div className="col-span-2 flex items-center justify-between gap-3 rounded-lg border border-border p-3">
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
          <CardFooter>
            <form.SubmitButton>Save</form.SubmitButton>
          </CardFooter>
        </form.Form>
      </form.AppForm>
    </Card>
  )
}
