import {
  useAddWorkExperienceBulletMutation,
  useAddWorkExperienceMutation,
  useDeleteWorkExperienceBulletMutation,
  useDeleteWorkExperienceMutation,
  useUpdateWorkExperienceBulletMutation
} from '@/src/hooks/use-profile'
import { useAppForm } from '@/src/hooks/use-app-form'
import { jobTypeSchema, type JobType, type WorkExperience, type WorkExperienceBullet } from '@/src/api/schemas'
import { Button, buttonVariants } from '@/src/components/ui/button'
import { Card, CardContent, CardHeader } from '@/src/components/ui/card'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger
} from '@/src/components/ui/dropdown-menu'
import { cn } from '@/src/lib/utils'
import { revalidateLogic } from '@tanstack/react-form'
import { ChevronDownIcon } from 'lucide-react'
import { useState } from 'react'
import { z } from 'zod'

const jobTypeOptions: { value: JobType; label: string }[] = [
  { value: 'internship', label: 'Internship' },
  { value: 'full_time', label: 'Full-time' },
  { value: 'part_time', label: 'Part-time' },
  { value: 'contract', label: 'Contract' }
]

const required = z.string().trim().min(1, 'Required')

const workExperienceFormSchema = z
  .object({
    company: required,
    job_title: required,
    job_type: jobTypeSchema,
    location: z.string(),
    start_date: z.string(),
    end_date: z.string()
  })
  .refine((v) => !v.start_date || !v.end_date || v.end_date >= v.start_date, {
    message: 'End is before start',
    path: ['end_date']
  })

function jobTypeLabel(jobType: string) {
  return jobTypeOptions.find((option) => option.value === jobType)?.label ?? jobType
}

export function WorkExperienceSection({ workExperience }: { workExperience: WorkExperience[] }) {
  const addWorkExperience = useAddWorkExperienceMutation()
  const deleteWorkExperience = useDeleteWorkExperienceMutation()

  const form = useAppForm({
    defaultValues: {
      company: '',
      job_title: '',
      job_type: 'internship' as JobType,
      location: '',
      start_date: '',
      end_date: ''
    },
    validationLogic: revalidateLogic(),
    validators: { onDynamic: workExperienceFormSchema },
    onSubmit: async ({ value, formApi }) => {
      await addWorkExperience.mutateAsync({
        ...value,
        company: value.company.trim(),
        job_title: value.job_title.trim(),
        location: value.location.trim()
      })
      formApi.reset()
    }
  })

  return (
    <Card>
      <CardHeader>
        <h3 className="text-sm font-semibold text-heading">Work experience</h3>
      </CardHeader>
      <CardContent>
        {workExperience.length > 0 && (
          <div className="mb-4 space-y-4">
            {workExperience.map((entry) => (
              <WorkExperienceEntry
                key={entry.id}
                entry={entry}
                onDelete={() => deleteWorkExperience.mutate(entry.id)}
              />
            ))}
          </div>
        )}
        <form.AppForm>
          <form.Form className="flex flex-wrap items-end gap-2">
            <form.AppField name="company">{(f) => <f.TextField label="Company" srOnlyLabel />}</form.AppField>
            <form.AppField name="job_title">{(f) => <f.TextField label="Job title" srOnlyLabel />}</form.AppField>
            <form.Field name="job_type">
              {(f) => (
                <DropdownMenu>
                  <DropdownMenuTrigger
                    aria-label="Job type"
                    className={cn(buttonVariants({ variant: 'outline' }), 'w-32 justify-between font-normal')}
                  >
                    {jobTypeLabel(f.state.value)}
                    <ChevronDownIcon className="opacity-50" />
                  </DropdownMenuTrigger>
                  <DropdownMenuContent>
                    {jobTypeOptions.map((option) => (
                      <DropdownMenuItem key={option.value} onClick={() => f.handleChange(option.value)}>
                        {option.label}
                      </DropdownMenuItem>
                    ))}
                  </DropdownMenuContent>
                </DropdownMenu>
              )}
            </form.Field>
            <form.AppField name="location">{(f) => <f.TextField label="Location" srOnlyLabel />}</form.AppField>
            <form.AppField name="start_date">{(f) => <f.TextField label="Start date" type="date" />}</form.AppField>
            <form.AppField name="end_date">{(f) => <f.TextField label="End date" type="date" />}</form.AppField>
            <form.SubmitButton>Add</form.SubmitButton>
          </form.Form>
        </form.AppForm>
      </CardContent>
    </Card>
  )
}

function WorkExperienceEntry({ entry, onDelete }: { entry: WorkExperience; onDelete: () => void }) {
  const addBullet = useAddWorkExperienceBulletMutation()

  const form = useAppForm({
    defaultValues: { bullet: '' },
    onSubmit: async ({ value, formApi }) => {
      const bullet = value.bullet.trim()
      if (!bullet) return
      await addBullet.mutateAsync({ workExperienceId: entry.id, req: { bullet } })
      formApi.reset()
    }
  })

  return (
    <div className="rounded-lg border border-border p-3">
      <div className="flex items-start justify-between">
        <div>
          <p className="text-sm font-semibold text-heading">
            {entry.job_title} · {entry.company}
          </p>
          <p className="text-xs text-muted-foreground">
            {jobTypeLabel(entry.job_type)}
            {entry.location ? ` · ${entry.location}` : ''}
            {entry.start_date ? ` · ${entry.start_date} – ${entry.end_date ?? 'present'}` : ''}
          </p>
        </div>
        <Button type="button" size="sm" onClick={onDelete}>
          Remove
        </Button>
      </div>
      {(entry.bullets ?? []).length > 0 && (
        <ul className="mt-2 space-y-1 text-sm">
          {(entry.bullets ?? []).map((item) => (
            <BulletItem key={item.id} workExperienceId={entry.id} item={item} />
          ))}
        </ul>
      )}
      <form.AppForm>
        <form.Form className="mt-2 flex gap-2">
          <form.AppField name="bullet">
            {(f) => <f.TextField label="Add a bullet point" srOnlyLabel className="flex-1" />}
          </form.AppField>
          <form.SubmitButton>Add</form.SubmitButton>
        </form.Form>
      </form.AppForm>
    </div>
  )
}

function BulletItem({ workExperienceId, item }: { workExperienceId: number; item: WorkExperienceBullet }) {
  const updateBullet = useUpdateWorkExperienceBulletMutation()
  const deleteBullet = useDeleteWorkExperienceBulletMutation()
  const [isEditing, setIsEditing] = useState(false)

  const form = useAppForm({
    defaultValues: { bullet: item.bullet },
    validationLogic: revalidateLogic(),
    validators: { onDynamic: z.object({ bullet: required }) },
    onSubmit: async ({ value }) => {
      const bullet = value.bullet.trim()
      // skip request when nothing changed
      if (bullet !== item.bullet) {
        await updateBullet.mutateAsync({ workExperienceId, id: item.id, bullet })
      }
      setIsEditing(false)
    }
  })

  function handleCancel() {
    form.reset({ bullet: item.bullet })
    setIsEditing(false)
  }

  if (isEditing) {
    return (
      <li>
        <form.AppForm>
          <form.Form className="flex items-start gap-2">
            <form.AppField name="bullet">
              {(f) => <f.TextField label="Bullet point" srOnlyLabel className="flex-1" autoFocus />}
            </form.AppField>
            <form.SubmitButton size="sm">Save</form.SubmitButton>
            <Button type="button" size="sm" onClick={handleCancel}>
              Cancel
            </Button>
          </form.Form>
        </form.AppForm>
        {updateBullet.error && (
          <p role="alert" className="mt-1 text-xs text-destructive">
            {updateBullet.error.message}
          </p>
        )}
      </li>
    )
  }

  return (
    <li className="flex items-center justify-between gap-2">
      <span>• {item.bullet}</span>
      <div className="flex gap-1">
        <Button
          type="button"
          size="sm"
          onClick={() => {
            form.reset({ bullet: item.bullet })
            setIsEditing(true)
          }}
        >
          Edit
        </Button>
        <Button
          type="button"
          size="sm"
          onClick={() => deleteBullet.mutate({ workExperienceId, id: item.id })}
          disabled={deleteBullet.isPending}
        >
          Remove
        </Button>
      </div>
    </li>
  )
}
