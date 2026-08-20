import { createFileRoute } from '@tanstack/react-router'
import { useId, useState, type FormEvent } from 'react'
import { ChevronDownIcon, XIcon } from 'lucide-react'
import {
  createProfile,
  loginProfile,
  useAddEducationMutation,
  useAddSkillMutation,
  useAddWorkExperienceBulletMutation,
  useAddWorkExperienceMutation,
  useDeleteEducationMutation,
  useDeleteSkillMutation,
  useDeleteWorkExperienceBulletMutation,
  useDeleteWorkExperienceMutation,
  useProfileQuery,
  useUpdateProfileMutation,
} from '../../api/profile'
import type { JobType, WorkExperience } from '../../api/schemas'
import { useAuth } from '../../stores/profile-store'
import { badgeVariants } from '../../components/ui/badge'
import { Button, buttonVariants } from '../../components/ui/button'
import { Card, CardContent, CardFooter, CardHeader } from '../../components/ui/card'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '../../components/ui/dropdown-menu'
import { Input } from '../../components/ui/input'
import { Label } from '../../components/ui/label'
import { cn } from '../../lib/utils'

export const Route = createFileRoute('/profile/')({
  component: RouteComponent,
})

const JOB_TYPE_OPTIONS: { value: JobType; label: string }[] = [
  { value: 'internship', label: 'Internship' },
  { value: 'full_time', label: 'Full-time' },
  { value: 'part_time', label: 'Part-time' },
  { value: 'contract', label: 'Contract' },
]

function RouteComponent() {
  const { token, isAuthenticated } = useAuth()

  if (!isAuthenticated) return <AuthForms />

  return <ProfileView token={token!} />
}

function AuthForms() {
  const { login } = useAuth()
  const [mode, setMode] = useState<'login' | 'signup'>('login')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const emailId = useId()
  const passwordId = useId()

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    setSubmitting(true)

    try {
      const { token } = mode === 'login' ? await loginProfile(email, password) : await createProfile(email, password)
      login(token)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="mx-auto mt-10 max-w-sm text-left">
      <div role="group" aria-label="Authentication mode" className="mb-4 flex gap-2 text-sm">
        <Button
          type="button"
          variant={mode === 'login' ? 'secondary' : 'ghost'}
          size="sm"
          aria-pressed={mode === 'login'}
          onClick={() => setMode('login')}
        >
          Log in
        </Button>
        <Button
          type="button"
          variant={mode === 'signup' ? 'secondary' : 'ghost'}
          size="sm"
          aria-pressed={mode === 'signup'}
          onClick={() => setMode('signup')}
        >
          Sign up
        </Button>
      </div>

      <form onSubmit={handleSubmit} className="space-y-3">
        <div className="space-y-1.5">
          <Label htmlFor={emailId} className="sr-only">
            Email
          </Label>
          <Input
            id={emailId}
            type="email"
            required
            autoComplete="email"
            placeholder="Email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor={passwordId} className="sr-only">
            Password
          </Label>
          <Input
            id={passwordId}
            type="password"
            required
            autoComplete={mode === 'login' ? 'current-password' : 'new-password'}
            placeholder="Password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
        </div>

        {error && (
          <p role="alert" className="text-sm text-destructive">
            {error}
          </p>
        )}

        <Button type="submit" disabled={submitting}>
          {mode === 'login' ? 'Log in' : 'Sign up'}
        </Button>
      </form>
    </div>
  )
}

function ProfileView({ token }: { token: string }) {
  const { logout } = useAuth()
  const { data: profile, isLoading } = useProfileQuery(token)

  if (isLoading || !profile) return <p className="mt-10 text-muted-foreground">Loading profile…</p>

  return (
    <div className="mx-auto mt-8 max-w-2xl space-y-6 text-left mb-4">
      <div className="flex items-center justify-between">
        <h2>{profile.email}</h2>
        <Button type="button" variant="ghost" size="sm" onClick={logout}>
          Log out
        </Button>
      </div>

      <BasicInfoSection token={token} profile={profile} />
      <EducationSection token={token} education={profile.education ?? []} />
      <SkillsSection token={token} skills={profile.skills ?? []} />
      <WorkExperienceSection token={token} workExperience={profile.work_experience ?? []} />
    </div>
  )
}

function BasicInfoSection({
  token,
  profile,
}: {
  token: string
  profile: { name: string; address: string; linked_in: string; github: string; portfolio: string }
}) {
  const updateProfile = useUpdateProfileMutation(token)
  const [name, setName] = useState(profile.name)
  const [address, setAddress] = useState(profile.address)
  const [linkedIn, setLinkedIn] = useState(profile.linked_in)
  const [github, setGithub] = useState(profile.github)
  const [portfolio, setPortfolio] = useState(profile.portfolio)
  const id = useId()

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    updateProfile.mutate({ name, address, linked_in: linkedIn, github, portfolio })
  }

  return (
    <Card>
      <CardHeader>
        <h3 className="text-sm font-semibold text-heading">Basic info</h3>
      </CardHeader>
      <form onSubmit={handleSubmit}>
        <CardContent className="grid grid-cols-2 gap-3">
          <div className="space-y-1.5">
            <Label htmlFor={`${id}-name`}>Name</Label>
            <Input id={`${id}-name`} value={name} onChange={(e) => setName(e.target.value)} />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor={`${id}-address`}>Address</Label>
            <Input id={`${id}-address`} value={address} onChange={(e) => setAddress(e.target.value)} />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor={`${id}-linkedin`}>LinkedIn URL</Label>
            <Input
              id={`${id}-linkedin`}
              type="url"
              value={linkedIn}
              onChange={(e) => setLinkedIn(e.target.value)}
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor={`${id}-github`}>GitHub URL</Label>
            <Input id={`${id}-github`} type="url" value={github} onChange={(e) => setGithub(e.target.value)} />
          </div>
          <div className="col-span-2 space-y-1.5">
            <Label htmlFor={`${id}-portfolio`}>Portfolio URL</Label>
            <Input
              id={`${id}-portfolio`}
              type="url"
              value={portfolio}
              onChange={(e) => setPortfolio(e.target.value)}
            />
          </div>
        </CardContent>
        <CardFooter>
          <Button type="submit" disabled={updateProfile.isPending}>
            Save
          </Button>
        </CardFooter>
      </form>
    </Card>
  )
}

function EducationSection({
  token,
  education,
}: {
  token: string
  education: { id: number; school_name: string; major: string; degree: string }[]
}) {
  const addEducation = useAddEducationMutation(token)
  const deleteEducation = useDeleteEducationMutation(token)
  const [schoolName, setSchoolName] = useState('')
  const [major, setMajor] = useState('')
  const [degree, setDegree] = useState('')
  const id = useId()

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    addEducation.mutate(
      { school_name: schoolName, major, degree },
      {
        onSuccess: () => {
          setSchoolName('')
          setMajor('')
          setDegree('')
        },
      },
    )
  }

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
                <span>
                  {entry.school_name} — {entry.major}, {entry.degree}
                </span>
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  onClick={() => deleteEducation.mutate(entry.id)}
                >
                  Remove
                </Button>
              </li>
            ))}
          </ul>
        )}

        <form onSubmit={handleSubmit} className="flex flex-wrap items-end gap-2">
          <div className="space-y-1.5">
            <Label htmlFor={`${id}-school`} className="sr-only">
              School
            </Label>
            <Input
              id={`${id}-school`}
              required
              placeholder="School"
              value={schoolName}
              onChange={(e) => setSchoolName(e.target.value)}
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor={`${id}-major`} className="sr-only">
              Major
            </Label>
            <Input
              id={`${id}-major`}
              required
              placeholder="Major"
              value={major}
              onChange={(e) => setMajor(e.target.value)}
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor={`${id}-degree`} className="sr-only">
              Degree
            </Label>
            <Input
              id={`${id}-degree`}
              required
              placeholder="Degree"
              value={degree}
              onChange={(e) => setDegree(e.target.value)}
            />
          </div>
          <Button type="submit" disabled={addEducation.isPending}>
            Add
          </Button>
        </form>
      </CardContent>
    </Card>
  )
}

function SkillsSection({ token, skills }: { token: string; skills: { id: number; skill: string }[] }) {
  const addSkill = useAddSkillMutation(token)
  const deleteSkill = useDeleteSkillMutation(token)
  const [skill, setSkill] = useState('')
  const id = useId()

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (!skill.trim()) return
    addSkill.mutate({ skill: skill.trim() }, { onSuccess: () => setSkill('') })
  }

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
                  className={cn(badgeVariants({ variant: 'secondary' }), 'gap-1 hover:bg-destructive/10 hover:text-destructive')}
                >
                  {entry.skill}
                  <XIcon aria-hidden="true" />
                </button>
              </li>
            ))}
          </ul>
        )}

        <form onSubmit={handleSubmit} className="flex gap-2">
          <Label htmlFor={id} className="sr-only">
            Add a skill
          </Label>
          <Input id={id} placeholder="Add a skill" value={skill} onChange={(e) => setSkill(e.target.value)} />
          <Button type="submit" disabled={addSkill.isPending}>
            Add
          </Button>
        </form>
      </CardContent>
    </Card>
  )
}

function WorkExperienceSection({ token, workExperience }: { token: string; workExperience: WorkExperience[] }) {
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
        },
      },
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
              <WorkExperienceEntry
                key={entry.id}
                token={token}
                entry={entry}
                onDelete={() => deleteWorkExperience.mutate(entry.id)}
              />
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
              {JOB_TYPE_OPTIONS.find((opt) => opt.value === jobType)?.label}
              <ChevronDownIcon className="opacity-50" />
            </DropdownMenuTrigger>
            <DropdownMenuContent>
              {JOB_TYPE_OPTIONS.map((opt) => (
                <DropdownMenuItem key={opt.value} onClick={() => setJobType(opt.value)}>
                  {opt.label}
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

function WorkExperienceEntry({
  token,
  entry,
  onDelete,
}: {
  token: string
  entry: WorkExperience
  onDelete: () => void
}) {
  const addBullet = useAddWorkExperienceBulletMutation(token)
  const deleteBullet = useDeleteWorkExperienceBulletMutation(token)
  const [bullet, setBullet] = useState('')
  const id = useId()

  function handleAddBullet(e: FormEvent) {
    e.preventDefault()
    if (!bullet.trim()) return
    addBullet.mutate(
      { workExperienceId: entry.id, req: { bullet: bullet.trim() } },
      { onSuccess: () => setBullet('') },
    )
  }

  return (
    <div className="rounded-lg border border-border p-3">
      <div className="flex items-start justify-between">
        <div>
          <p className="text-sm font-semibold text-heading">
            {entry.job_title} — {entry.company}
          </p>
          <p className="text-xs text-muted-foreground">
            {JOB_TYPE_OPTIONS.find((opt) => opt.value === entry.job_type)?.label ?? entry.job_type}
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
          {(entry.bullets ?? []).map((b) => (
            <li key={b.id} className="flex items-center justify-between gap-2">
              <span>• {b.bullet}</span>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={() => deleteBullet.mutate({ workExperienceId: entry.id, id: b.id })}
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
