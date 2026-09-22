import { useAddEducationMutation, useDeleteEducationMutation } from '@/src/hooks/use-profile'
import { useAppForm } from '@/src/hooks/use-app-form'
import type { Education } from '@/src/api/schemas'
import { Button } from '@/src/components/ui/button'
import { Card, CardContent, CardHeader } from '@/src/components/ui/card'
import { revalidateLogic } from '@tanstack/react-form'
import { z } from 'zod'

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

function formatEducation(entry: Education) {
  const parts = [`${entry.school_name} · ${entry.major}, ${entry.degree}`]
  if (entry.gpa != null) parts.push(`GPA ${entry.gpa}`)
  if (entry.start_date) parts.push(`${entry.start_date} – ${entry.end_date ?? 'present'}`)
  return parts.join(' · ')
}

export function EducationSection({ education }: { education: Education[] }) {
  const addEducation = useAddEducationMutation()
  const deleteEducation = useDeleteEducationMutation()

  const form = useAppForm({
    defaultValues: { school_name: '', major: '', degree: '', gpa: '', start_date: '', end_date: '' },
    validationLogic: revalidateLogic(),
    validators: { onDynamic: educationFormSchema },
    onSubmit: async ({ value, formApi }) => {
      await addEducation.mutateAsync({
        school_name: value.school_name.trim(),
        major: value.major.trim(),
        degree: value.degree.trim(),
        gpa: value.gpa ? Number(value.gpa) : null,
        start_date: value.start_date || undefined,
        end_date: value.end_date || undefined
      })
      formApi.reset()
    }
  })

  return (
    <Card>
      <CardHeader>
        <h3 className="text-sm font-semibold text-heading">Education</h3>
      </CardHeader>
      <CardContent>
        {education.length > 0 && (
          <ul className="mb-3 space-y-1">
            {education.map((entry) => (
              <li key={entry.id} className="flex items-center justify-between text-sm">
                <span>{formatEducation(entry)}</span>
                <Button type="button" variant="ghost" size="sm" onClick={() => deleteEducation.mutate(entry.id)}>
                  Remove
                </Button>
              </li>
            ))}
          </ul>
        )}
        <form.AppForm>
          <form.Form className="flex flex-wrap items-end gap-2">
            <form.AppField name="school_name">{(f) => <f.TextField label="School" srOnlyLabel />}</form.AppField>
            <form.AppField name="major">{(f) => <f.TextField label="Major" srOnlyLabel />}</form.AppField>
            <form.AppField name="degree">{(f) => <f.TextField label="Degree" srOnlyLabel />}</form.AppField>
            <form.AppField name="gpa">
              {(f) => (
                <f.TextField label="GPA" srOnlyLabel type="number" step="0.01" min="0" max="4" className="w-20" />
              )}
            </form.AppField>
            <form.AppField name="start_date">{(f) => <f.TextField label="Start date" type="date" />}</form.AppField>
            <form.AppField name="end_date">{(f) => <f.TextField label="End date" type="date" />}</form.AppField>
            <form.SubmitButton>Add</form.SubmitButton>
          </form.Form>
        </form.AppForm>
      </CardContent>
    </Card>
  )
}
