import { useAddSkillMutation, useDeleteSkillMutation } from '@/src/hooks/use-profile'
import { useAppForm } from '@/src/hooks/use-app-form'
import type { Skill } from '@/src/api/schemas'
import { badgeVariants } from '@/src/components/ui/badge'
import { Card, CardContent, CardHeader } from '@/src/components/ui/card'
import { cn } from '@/src/lib/utils'
import { XIcon } from 'lucide-react'

export function SkillsSection({ skills }: { skills: Skill[] }) {
  const addSkill = useAddSkillMutation()
  const deleteSkill = useDeleteSkillMutation()

  const form = useAppForm({
    defaultValues: { skill: '' },
    onSubmit: async ({ value, formApi }) => {
      const skill = value.skill.trim()
      if (!skill) return
      await addSkill.mutateAsync({ skill })
      formApi.reset()
    }
  })

  return (
    <Card>
      <CardHeader>
        <h3 className="text-sm font-semibold text-heading">Skills</h3>
      </CardHeader>
      <CardContent>
        {skills.length > 0 && (
          <ul className="mb-3 flex flex-wrap gap-1.5">
            {skills.map((entry) => (
              <li key={entry.id}>
                <button
                  type="button"
                  onClick={() => deleteSkill.mutate(entry.id)}
                  aria-label={`Remove ${entry.skill}`}
                  className={cn(
                    badgeVariants({ variant: 'secondary' }),
                    'gap-1 hover:bg-destructive/10 hover:text-destructive'
                  )}
                >
                  {entry.skill}
                  <XIcon aria-hidden="true" />
                </button>
              </li>
            ))}
          </ul>
        )}
        <form.AppForm>
          <form.Form className="flex gap-2">
            <form.AppField name="skill">
              {(f) => <f.TextField label="Add a skill" srOnlyLabel className="flex-1" />}
            </form.AppField>
            <form.SubmitButton>Add</form.SubmitButton>
          </form.Form>
        </form.AppForm>
      </CardContent>
    </Card>
  )
}
