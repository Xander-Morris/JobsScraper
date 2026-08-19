import { createFileRoute } from '@tanstack/react-router'
import { useState, type FormEvent } from 'react'
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

export const Route = createFileRoute('/profile/')({
  component: RouteComponent,
})

const inputClass =
  'rounded border border-border bg-transparent px-3 py-1.5 text-sm text-heading placeholder:text-muted focus:border-accent-border focus:outline-none'
const buttonClass = 'rounded bg-accent px-3 py-1.5 text-sm text-white disabled:opacity-50'
const deleteButtonClass = 'text-xs text-muted hover:text-heading'

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
      <div className="mb-4 flex gap-4 text-sm">
        <button
          type="button"
          onClick={() => setMode('login')}
          className={mode === 'login' ? 'font-semibold text-heading' : 'text-muted'}
        >
          Log in
        </button>
        <button
          type="button"
          onClick={() => setMode('signup')}
          className={mode === 'signup' ? 'font-semibold text-heading' : 'text-muted'}
        >
          Sign up
        </button>
      </div>

      <form onSubmit={handleSubmit} className="space-y-3">
        <input
          type="email"
          required
          placeholder="Email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          className={`w-full ${inputClass}`}
        />
        <input
          type="password"
          required
          placeholder="Password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          className={`w-full ${inputClass}`}
        />

        {error && <p className="text-sm text-red-500">{error}</p>}

        <button type="submit" disabled={submitting} className={buttonClass}>
          {mode === 'login' ? 'Log in' : 'Sign up'}
        </button>
      </form>
    </div>
  )
}

function ProfileView({ token }: { token: string }) {
  const { logout } = useAuth()
  const { data: profile, isLoading } = useProfileQuery(token)

  if (isLoading || !profile) return <p className="mt-10 text-muted">Loading profile…</p>

  return (
    <div className="mx-auto mt-8 max-w-2xl space-y-10 text-left">
      <div className="flex items-center justify-between">
        <h2>{profile.email}</h2>
        <button type="button" onClick={logout} className={deleteButtonClass}>
          Log out
        </button>
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

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    updateProfile.mutate({ name, address, linked_in: linkedIn, github, portfolio })
  }

  return (
    <section>
      <h3 className="mb-2 text-sm font-semibold text-heading">Basic info</h3>
      <form onSubmit={handleSubmit} className="grid grid-cols-2 gap-2">
        <input placeholder="Name" value={name} onChange={(e) => setName(e.target.value)} className={inputClass} />
        <input
          placeholder="Address"
          value={address}
          onChange={(e) => setAddress(e.target.value)}
          className={inputClass}
        />
        <input
          placeholder="LinkedIn URL"
          value={linkedIn}
          onChange={(e) => setLinkedIn(e.target.value)}
          className={inputClass}
        />
        <input
          placeholder="GitHub URL"
          value={github}
          onChange={(e) => setGithub(e.target.value)}
          className={inputClass}
        />
        <input
          placeholder="Portfolio URL"
          value={portfolio}
          onChange={(e) => setPortfolio(e.target.value)}
          className={`col-span-2 ${inputClass}`}
        />
        <button type="submit" disabled={updateProfile.isPending} className={`${buttonClass} col-span-2 w-fit`}>
          Save
        </button>
      </form>
    </section>
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
    <section>
      <h3 className="mb-2 text-sm font-semibold text-heading">Education</h3>

      <ul className="mb-3 space-y-1">
        {education.map((entry) => (
          <li key={entry.id} className="flex items-center justify-between text-sm">
            <span>
              {entry.school_name} — {entry.major}, {entry.degree}
            </span>
            <button type="button" onClick={() => deleteEducation.mutate(entry.id)} className={deleteButtonClass}>
              Remove
            </button>
          </li>
        ))}
      </ul>

      <form onSubmit={handleSubmit} className="flex flex-wrap gap-2">
        <input
          required
          placeholder="School"
          value={schoolName}
          onChange={(e) => setSchoolName(e.target.value)}
          className={inputClass}
        />
        <input
          required
          placeholder="Major"
          value={major}
          onChange={(e) => setMajor(e.target.value)}
          className={inputClass}
        />
        <input
          required
          placeholder="Degree"
          value={degree}
          onChange={(e) => setDegree(e.target.value)}
          className={inputClass}
        />
        <button type="submit" disabled={addEducation.isPending} className={buttonClass}>
          Add
        </button>
      </form>
    </section>
  )
}

function SkillsSection({ token, skills }: { token: string; skills: { id: number; skill: string }[] }) {
  const addSkill = useAddSkillMutation(token)
  const deleteSkill = useDeleteSkillMutation(token)
  const [skill, setSkill] = useState('')

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    if (!skill.trim()) return
    addSkill.mutate({ skill: skill.trim() }, { onSuccess: () => setSkill('') })
  }

  return (
    <section>
      <h3 className="mb-2 text-sm font-semibold text-heading">Skills</h3>

      <div className="mb-3 flex flex-wrap gap-1.5 font-mono text-xs">
        {skills.map((entry) => (
          <button
            key={entry.id}
            type="button"
            onClick={() => deleteSkill.mutate(entry.id)}
            className="rounded bg-code-bg px-2 py-1 text-muted hover:text-heading"
            title="Remove"
          >
            {entry.skill} ×
          </button>
        ))}
      </div>

      <form onSubmit={handleSubmit} className="flex gap-2">
        <input
          placeholder="Add a skill"
          value={skill}
          onChange={(e) => setSkill(e.target.value)}
          className={inputClass}
        />
        <button type="submit" disabled={addSkill.isPending} className={buttonClass}>
          Add
        </button>
      </form>
    </section>
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
    <section>
      <h3 className="mb-2 text-sm font-semibold text-heading">Work experience</h3>

      <div className="mb-3 space-y-4">
        {workExperience.map((entry) => (
          <WorkExperienceEntry key={entry.id} token={token} entry={entry} onDelete={() => deleteWorkExperience.mutate(entry.id)} />
        ))}
      </div>

      <form onSubmit={handleSubmit} className="flex flex-wrap gap-2">
        <input
          required
          placeholder="Company"
          value={company}
          onChange={(e) => setCompany(e.target.value)}
          className={inputClass}
        />
        <input
          required
          placeholder="Job title"
          value={jobTitle}
          onChange={(e) => setJobTitle(e.target.value)}
          className={inputClass}
        />
        <select value={jobType} onChange={(e) => setJobType(e.target.value as JobType)} className={inputClass}>
          {JOB_TYPE_OPTIONS.map((opt) => (
            <option key={opt.value} value={opt.value}>
              {opt.label}
            </option>
          ))}
        </select>
        <input
          placeholder="Location"
          value={location}
          onChange={(e) => setLocation(e.target.value)}
          className={inputClass}
        />
        <input
          type="date"
          value={startDate}
          onChange={(e) => setStartDate(e.target.value)}
          className={inputClass}
        />
        <input type="date" value={endDate} onChange={(e) => setEndDate(e.target.value)} className={inputClass} />
        <button type="submit" disabled={addWorkExperience.isPending} className={buttonClass}>
          Add
        </button>
      </form>
    </section>
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

  function handleAddBullet(e: FormEvent) {
    e.preventDefault()
    if (!bullet.trim()) return
    addBullet.mutate(
      { workExperienceId: entry.id, req: { bullet: bullet.trim() } },
      { onSuccess: () => setBullet('') },
    )
  }

  return (
    <div className="rounded border border-border p-3">
      <div className="flex items-start justify-between">
        <div>
          <p className="text-sm font-semibold text-heading">
            {entry.job_title} — {entry.company}
          </p>
          <p className="text-xs text-muted">
            {JOB_TYPE_OPTIONS.find((opt) => opt.value === entry.job_type)?.label ?? entry.job_type}
            {entry.location ? ` · ${entry.location}` : ''}
            {entry.start_date ? ` · ${entry.start_date} – ${entry.end_date ?? 'present'}` : ''}
          </p>
        </div>
        <button type="button" onClick={onDelete} className={deleteButtonClass}>
          Remove
        </button>
      </div>

      <ul className="mt-2 space-y-1 text-sm">
        {(entry.bullets ?? []).map((b) => (
          <li key={b.id} className="flex items-center justify-between gap-2">
            <span>• {b.bullet}</span>
            <button
              type="button"
              onClick={() => deleteBullet.mutate({ workExperienceId: entry.id, id: b.id })}
              className={deleteButtonClass}
            >
              Remove
            </button>
          </li>
        ))}
      </ul>

      <form onSubmit={handleAddBullet} className="mt-2 flex gap-2">
        <input
          placeholder="Add a bullet point"
          value={bullet}
          onChange={(e) => setBullet(e.target.value)}
          className={`flex-1 ${inputClass}`}
        />
        <button type="submit" disabled={addBullet.isPending} className={buttonClass}>
          Add
        </button>
      </form>
    </div>
  )
}
