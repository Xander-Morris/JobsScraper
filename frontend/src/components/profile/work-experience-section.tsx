import { useId, useState, type FormEvent } from 'react'
import { ChevronDownIcon } from 'lucide-react'
import {
  useAddWorkExperienceBulletMutation,
  useAddWorkExperienceMutation,
  useDeleteWorkExperienceBulletMutation,
  useDeleteWorkExperienceMutation
} from '@/src/api/profile'
import type { JobType, WorkExperience } from '@/src/api/schemas'
import { Button, buttonVariants } from '@/src/components/ui/button'
import { Card, CardContent, CardHeader } from '@/src/components/ui/card'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger
} from '@/src/components/ui/dropdown-menu'
import { Input } from '@/src/components/ui/input'
import { Label } from '@/src/components/ui/label'
import { cn } from '@/src/lib/utils'
import { useAuth } from '@/src/stores/profile-store'

const jobTypeOptions: { value: JobType; label: string }[] = [
  { value: 'internship', label: 'Internship' },
  { value: 'full_time', label: 'Full-time' },
  { value: 'part_time', label: 'Part-time' },
  { value: 'contract', label: 'Contract' }
]

export function WorkExperienceSection({ workExperience }: { workExperience: WorkExperience[] }) {
  const { token } = useAuth()
  const addWorkExperience = useAddWorkExperienceMutation(token)
  const deleteWorkExperience = useDeleteWorkExperienceMutation(token)
  const [company, setCompany] = useState('')
  const [jobTitle, setJobTitle] = useState('')
  const [jobType, setJobType] = useState<JobType>('internship')
  const [location, setLocation] = useState('')
  const [startDate, setStartDate] = useState('')
  const [endDate, setEndDate] = useState('')
  const id = useId()

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    addWorkExperience.mutate(
      { company, job_title: jobTitle, job_type: jobType, location, start_date: startDate, end_date: endDate },
      {
        onSuccess: () => {
          setCompany('')
          setJobTitle('')
          setLocation('')
          setStartDate('')
          setEndDate('')
        }
      }
    )
  }

  return (
    <Card>
      <CardHeader>
        <h3 className="text-sm font-semibold text-heading">Work experience</h3>
      </CardHeader>
      <CardContent>
        {workExperience.length > 0 && (
          <div className="mb-4 space-y-4">
            {workExperience.map((entry) => (
              <WorkExperienceEntry key={entry.id} entry={entry} onDelete={() => deleteWorkExperience.mutate(entry.id)} />
            ))}
          </div>
        )}
        <form onSubmit={handleSubmit} className="flex flex-wrap items-end gap-2">
          <div className="space-y-1.5">
            <Label htmlFor={`${id}-company`} className="sr-only">
              Company
            </Label>
            <Input
              id={`${id}-company`}
              required
              placeholder="Company"
              value={company}
              onChange={(e) => setCompany(e.target.value)}
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor={`${id}-title`} className="sr-only">
              Job title
            </Label>
            <Input
              id={`${id}-title`}
              required
              placeholder="Job title"
              value={jobTitle}
              onChange={(e) => setJobTitle(e.target.value)}
            />
          </div>
          <DropdownMenu>
            <DropdownMenuTrigger
              className={cn(buttonVariants({ variant: 'outline' }), 'w-32 justify-between font-normal')}
            >
              {jobTypeOptions.find((option) => option.value === jobType)?.label}
              <ChevronDownIcon className="opacity-50" />
            </DropdownMenuTrigger>
            <DropdownMenuContent>
              {jobTypeOptions.map((option) => (
                <DropdownMenuItem key={option.value} onClick={() => setJobType(option.value)}>
                  {option.label}
                </DropdownMenuItem>
              ))}
            </DropdownMenuContent>
          </DropdownMenu>
          <div className="space-y-1.5">
            <Label htmlFor={`${id}-location`} className="sr-only">
              Location
            </Label>
            <Input
              id={`${id}-location`}
              placeholder="Location"
              value={location}
              onChange={(e) => setLocation(e.target.value)}
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor={`${id}-start`}>Start date</Label>
            <Input id={`${id}-start`} type="date" value={startDate} onChange={(e) => setStartDate(e.target.value)} />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor={`${id}-end`}>End date</Label>
            <Input id={`${id}-end`} type="date" value={endDate} onChange={(e) => setEndDate(e.target.value)} />
          </div>
          <Button type="submit" disabled={addWorkExperience.isPending}>
            Add
          </Button>
        </form>
      </CardContent>
    </Card>
  )
}

function WorkExperienceEntry({ entry, onDelete }: { entry: WorkExperience; onDelete: () => void }) {
  const { token } = useAuth()
  const addBullet = useAddWorkExperienceBulletMutation(token)
  const deleteBullet = useDeleteWorkExperienceBulletMutation(token)
  const [bullet, setBullet] = useState('')
  const id = useId()

  function handleAddBullet(e: FormEvent) {
    e.preventDefault()
    if (!bullet.trim()) return
    addBullet.mutate({ workExperienceId: entry.id, req: { bullet: bullet.trim() } }, { onSuccess: () => setBullet('') })
  }

  return (
    <div className="rounded-lg border border-border p-3">
      <div className="flex items-start justify-between">
        <div>
          <p className="text-sm font-semibold text-heading">
            {entry.job_title} · {entry.company}
          </p>
          <p className="text-xs text-muted-foreground">
            {jobTypeOptions.find((option) => option.value === entry.job_type)?.label ?? entry.job_type}
            {entry.location ? ` · ${entry.location}` : ''}
            {entry.start_date ? ` · ${entry.start_date} – ${entry.end_date ?? 'present'}` : ''}
          </p>
        </div>
        <Button type="button" variant="ghost" size="sm" onClick={onDelete}>
          Remove
        </Button>
      </div>
      {(entry.bullets ?? []).length > 0 && (
        <ul className="mt-2 space-y-1 text-sm">
          {(entry.bullets ?? []).map((item) => (
            <li key={item.id} className="flex items-center justify-between gap-2">
              <span>• {item.bullet}</span>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={() => deleteBullet.mutate({ workExperienceId: entry.id, id: item.id })}
              >
                Remove
              </Button>
            </li>
          ))}
        </ul>
      )}
      <form onSubmit={handleAddBullet} className="mt-2 flex gap-2">
        <Label htmlFor={id} className="sr-only">
          Add a bullet point
        </Label>
        <Input
          id={id}
          placeholder="Add a bullet point"
          value={bullet}
          onChange={(e) => setBullet(e.target.value)}
          className="flex-1"
        />
        <Button type="submit" disabled={addBullet.isPending}>
          Add
        </Button>
      </form>
    </div>
  )
}
