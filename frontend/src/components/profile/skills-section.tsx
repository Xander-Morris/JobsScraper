import { useId, useState, type FormEvent } from 'react'
import { useAddSkillMutation, useDeleteSkillMutation } from '@/src/api/profile'
import type { Skill } from '@/src/api/schemas'
import { badgeVariants } from '@/src/components/ui/badge'
import { Button } from '@/src/components/ui/button'
import { Card, CardContent, CardHeader } from '@/src/components/ui/card'
import { Input } from '@/src/components/ui/input'
import { Label } from '@/src/components/ui/label'
import { cn } from '@/src/lib/utils'
import { useAuth } from '@/src/stores/profile-store'
import { XIcon } from 'lucide-react'

export function SkillsSection({ skills }: { skills: Skill[] }) {
  const { token } = useAuth()
  const addSkill = useAddSkillMutation(token)
  const deleteSkill = useDeleteSkillMutation(token)
  const [skill, setSkill] = useState('')
  const id = useId()

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (!skill.trim()) return
    addSkill.mutate({ skill: skill.trim() }, { onSuccess: () => setSkill('') })
  }

  return <Card>
    <CardHeader><h3 className="text-sm font-semibold text-heading">Skills</h3></CardHeader>
    <CardContent>
      {skills.length > 0 && <ul className="mb-3 flex flex-wrap gap-1.5">
        {skills.map((entry) => <li key={entry.id}>
          <button type="button" onClick={() => deleteSkill.mutate(entry.id)} aria-label={`Remove ${entry.skill}`} className={cn(badgeVariants({ variant: 'secondary' }), 'gap-1 hover:bg-destructive/10 hover:text-destructive')}>
            {entry.skill}<XIcon aria-hidden="true" />
          </button>
        </li>)}
      </ul>}
      <form onSubmit={handleSubmit} className="flex gap-2">
        <Label htmlFor={id} className="sr-only">Add a skill</Label>
        <Input id={id} placeholder="Add a skill" value={skill} onChange={(e) => setSkill(e.target.value)} />
        <Button type="submit" disabled={addSkill.isPending}>Add</Button>
      </form>
    </CardContent>
  </Card>
}
