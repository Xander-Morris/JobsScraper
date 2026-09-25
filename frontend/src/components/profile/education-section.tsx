import type { AddEducationRequest } from '@/src/api/profile'
import type { Education } from '@/src/api/schemas'
import { Card, CardContent, CardHeader } from '@/src/components/ui/card'
import { useAppForm } from '@/src/hooks/use-app-form'
import {
  useAddEducationMutation,
  useDeleteEducationMutation,
  useUpdateEducationMutation
} from '@/src/hooks/use-profile'
import { revalidateLogic } from '@tanstack/react-form'
import { useState } from 'react'
import { z } from 'zod'
import { Button } from '../ui/button'

const required = z.string().trim().min(1, 'Required')

const educationFormSchema = z
  .object({
    school_name: required,
    major: required,
    degree: required,
    gpa: z.string().refine((v) => v === '' || (Number(v) >= 0 && Number(v) <= 4), 'GPA must be 0–4'),
    start_date: z.string(),
    end_date: z.string()
  })
  .refine((v) => !v.start_date || !v.end_date || v.end_date >= v.start_date, {
    message: 'End is before start',
    path: ['end_date']
  })

type EducationFormValues = z.input<typeof educationFormSchema>

const emptyEducation: EducationFormValues = {
  school_name: '',
  major: '',
  degree: '',
  gpa: '',
  start_date: '',
  end_date: ''
}

function toFormValues(entry: Education): EducationFormValues {
  return {
    school_name: entry.school_name,
    major: entry.major,
    degree: entry.degree,
    gpa: entry.gpa?.toString() ?? '',
    start_date: entry.start_date ?? '',
    end_date: entry.end_date ?? ''
  }
}

function toRequest(value: EducationFormValues): AddEducationRequest {
  return {
    school_name: value.school_name.trim(),
    major: value.major.trim(),
    degree: value.degree.trim(),
    gpa: value.gpa ? Number(value.gpa) : null,
    start_date: value.start_date || undefined,
    end_date: value.end_date || undefined
  }
}

export function EducationSection({ education }: { education: Education[] }) {
  const addEducation = useAddEducationMutation()

  return (
    <Card>
      <CardHeader>
        <h3 className="text-sm font-semibold text-heading">Education</h3>
      </CardHeader>
      <CardContent>
        {education.length > 0 && (
          <ul className="mb-3 space-y-3">
            {education.map((entry) => (
              <EducationEntry key={entry.id} entry={entry} />
            ))}
          </ul>
        )}
        <EducationForm
          defaultValues={emptyEducation}
          submitLabel="Add"
          onSubmit={(req) => addEducation.mutateAsync(req)}
        />
      </CardContent>
    </Card>
  )
}

function EducationEntry({ entry }: { entry: Education }) {
  const updateEducation = useUpdateEducationMutation()
  const deleteEducation = useDeleteEducationMutation()
  const [isEditing, setIsEditing] = useState(false)

  if (isEditing) {
    const original = toRequest(toFormValues(entry))

    return (
      <li>
        <EducationForm
          defaultValues={toFormValues(entry)}
          submitLabel="Save"
          onSubmit={async (req) => {
            // skip request when nothing changed
            if (JSON.stringify(req) !== JSON.stringify(original)) {
              await updateEducation.mutateAsync({ id: entry.id, req })
            }
            setIsEditing(false)
          }}
          onCancel={() => setIsEditing(false)}
        />
        {updateEducation.error && (
          <p role="alert" className="mt-1 text-xs text-destructive">
            {updateEducation.error.message}
          </p>
        )}
      </li>
    )
  }

  return (
    <li className="flex w-full items-center justify-between gap-4 text-sm">
      <div className="flex flex-col">
        <span>{entry.end_date}</span>
        <span className="font-bold">{entry.school_name}</span>
        <span>
          {entry.major} - {entry.degree}
        </span>
        <span>{entry.gpa?.toFixed(2)}</span>
      </div>
      <div className="flex gap-2">
        <Button type="button" variant="outline" size="sm" onClick={() => setIsEditing(true)}>
          Edit
        </Button>
        <Button
          type="button"
          size="sm"
          onClick={() => deleteEducation.mutate(entry.id)}
          disabled={deleteEducation.isPending}
        >
          Remove
        </Button>
      </div>
    </li>
  )
}

function EducationForm({
  defaultValues,
  submitLabel,
  onSubmit,
  onCancel
}: {
  defaultValues: EducationFormValues
  submitLabel: string
  onSubmit: (req: AddEducationRequest) => Promise<unknown>
  onCancel?: () => void
}) {
  const form = useAppForm({
    defaultValues,
    validationLogic: revalidateLogic(),
    validators: { onDynamic: educationFormSchema },
    onSubmit: async ({ value, formApi }) => {
      await onSubmit(toRequest(value))
      formApi.reset()
    }
  })

  return (
    <form.AppForm>
      <form.Form className="flex flex-wrap items-end gap-2">
        <form.AppField name="school_name">{(f) => <f.TextField label="School" srOnlyLabel />}</form.AppField>
        <form.AppField name="major">{(f) => <f.TextField label="Major" srOnlyLabel />}</form.AppField>
        <form.AppField name="degree">{(f) => <f.TextField label="Degree" srOnlyLabel />}</form.AppField>
        <form.AppField name="gpa">
          {(f) => <f.TextField label="GPA" srOnlyLabel type="number" step="0.01" min="0" max="4" className="w-20" />}
        </form.AppField>
        <form.AppField name="start_date">{(f) => <f.TextField label="Start date" type="date" />}</form.AppField>
        <form.AppField name="end_date">{(f) => <f.TextField label="End date" type="date" />}</form.AppField>
        <form.SubmitButton>{submitLabel}</form.SubmitButton>
        {onCancel && (
          <Button type="button" variant="ghost" onClick={onCancel}>
            Cancel
          </Button>
        )}
      </form.Form>
    </form.AppForm>
  )
}
