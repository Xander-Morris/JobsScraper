import {
  downloadResume,
  useActivateResumeMutation,
  useDeleteResumeMutation,
  useResumeExtractionQuery,
  useTriggerResumeExtractionMutation,
  useUpdateResumeMutation,
  useUploadResumeMutation,
} from '@/src/api/profile'
import type { Resume } from '@/src/api/schemas'
import { badgeVariants } from '@/src/components/ui/badge'
import { Button } from '@/src/components/ui/button'
import { Card, CardContent, CardHeader } from '@/src/components/ui/card'
import { Input } from '@/src/components/ui/input'
import { Label } from '@/src/components/ui/label'
import { cn } from '@/src/lib/utils'
import { DownloadIcon, FileTextIcon } from 'lucide-react'
import { useId, useState, type FormEvent } from 'react'

const acceptedResumeTypes = '.pdf,.doc,.docx,application/pdf,application/msword,application/vnd.openxmlformats-officedocument.wordprocessingml.document'

export function ResumesSection({ token, resumes }: { token: string; resumes: Resume[] }) {
  const uploadResume = useUploadResumeMutation(token)
  const [file, setFile] = useState<File | null>(null)
  const [error, setError] = useState<string | null>(null)
  const id = useId()

  function handleUpload(e: FormEvent) {
    e.preventDefault()
    if (!file) return
    setError(null)
    uploadResume.mutate(file, { onSuccess: () => setFile(null), onError: (err) => setError(err instanceof Error ? err.message : 'Unable to upload resume') })
  }

  return <Card>
    <CardHeader>
      <h3 className="text-sm font-semibold text-heading">Resumes</h3>
      <p className="text-xs text-muted-foreground">Upload PDF, DOC, or DOCX files up to 10 MB. Your active resume is used to rank job search results by relevance.</p>
    </CardHeader>
    <CardContent className="space-y-3">
      {resumes.length > 0 ? <ul className="space-y-2">{resumes.map((resume) => <ResumeEntry key={resume.id} token={token} resume={resume} />)}</ul> : <p className="text-sm text-muted-foreground">No resumes uploaded yet.</p>}
      <form onSubmit={handleUpload} className="flex flex-wrap items-center gap-2">
        <Label htmlFor={id} className="sr-only">Resume file</Label>
        <Input id={id} type="file" accept={acceptedResumeTypes} onChange={(e) => setFile(e.target.files?.[0] ?? null)} className="max-w-sm" />
        <Button type="submit" disabled={!file || uploadResume.isPending}>{uploadResume.isPending ? 'Uploading...' : 'Upload resume'}</Button>
      </form>
      {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
    </CardContent>
  </Card>
}

function splitFileName(fileName: string): [string, string] {
  const dot = fileName.lastIndexOf('.')
  if (dot <= 0) return [fileName, '']
  return [fileName.slice(0, dot), fileName.slice(dot)]
}

function ResumeEntry({ token, resume }: { token: string; resume: Resume }) {
  const updateResume = useUpdateResumeMutation(token)
  const deleteResume = useDeleteResumeMutation(token)
  const activateResume = useActivateResumeMutation(token)
  const [baseName, extension] = splitFileName(resume.file_name)
  const [fileName, setFileName] = useState(baseName)
  const [error, setError] = useState<string | null>(null)
  const replaceId = useId()
  const newFileName = `${fileName.trim()}${extension}`

  async function handleDownload() {
    setError(null)
    try {
      const file = await downloadResume(token, resume.id)
      const url = URL.createObjectURL(file)
      const anchor = document.createElement('a')
      anchor.href = url
      anchor.download = resume.file_name
      anchor.click()
      URL.revokeObjectURL(url)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unable to download resume')
    }
  }

  function handleRename(e: FormEvent) {
    e.preventDefault()
    if (!fileName.trim() || newFileName === resume.file_name) return
    setError(null)
    updateResume.mutate({ id: resume.id, update: newFileName }, { onError: (err) => setError(err instanceof Error ? err.message : 'Unable to rename resume') })
  }

  function handleReplace(file: File | null) {
    if (!file) return
    setError(null)
    updateResume.mutate({ id: resume.id, update: file }, { onError: (err) => setError(err instanceof Error ? err.message : 'Unable to replace resume') })
  }

  return <li className="rounded-lg border border-border p-3">
    <div className="flex flex-wrap items-center gap-2">
      <FileTextIcon className="size-4 text-muted-foreground" aria-hidden="true" />
      <form onSubmit={handleRename} className="flex flex-1 items-center gap-2">
        <Label htmlFor={`${replaceId}-name`} className="sr-only">Resume name</Label>
        <Input id={`${replaceId}-name`} value={fileName} onChange={(e) => setFileName(e.target.value)} className="max-w-xs" />
        <span className="text-xs text-muted-foreground">{extension}</span>
        <Button type="submit" variant="outline" size="sm" disabled={updateResume.isPending || newFileName === resume.file_name}>Rename</Button>
      </form>
      <span className="text-xs text-muted-foreground">{formatFileSize(resume.file_size)}</span>
      {resume.is_active
        ? <span className={badgeVariants({ variant: 'default' })}>Active</span>
        : <Button type="button" variant="outline" size="sm" onClick={() => activateResume.mutate(resume.id)} disabled={activateResume.isPending}>Set active</Button>}
      <Button type="button" variant="outline" size="sm" onClick={() => void handleDownload()}><DownloadIcon aria-hidden="true" /> Download</Button>
      <Button type="button" variant="destructive" size="sm" onClick={() => deleteResume.mutate(resume.id)} disabled={deleteResume.isPending}>Delete</Button>
    </div>
    <div className="mt-2 flex items-center gap-2">
      <Label htmlFor={replaceId} className="text-xs text-muted-foreground">Replace file</Label>
      <Input id={replaceId} type="file" accept={acceptedResumeTypes} onChange={(e) => handleReplace(e.target.files?.[0] ?? null)} disabled={updateResume.isPending} className="max-w-sm text-xs" />
    </div>
    {error && <p role="alert" className="mt-2 text-sm text-destructive">{error}</p>}
    <div className="mt-3 border-t border-border pt-3">
      <ResumeExtractionPanel token={token} resumeId={resume.id} />
    </div>
  </li>
}

function ResumeExtractionPanel({ token, resumeId }: { token: string; resumeId: number }) {
  const { data, isLoading } = useResumeExtractionQuery(token, resumeId, { enabled: true })
  const retryExtraction = useTriggerResumeExtractionMutation(token)

  if (isLoading || !data || data.status === 'pending') {
    return <p className="text-sm text-muted-foreground">Extracting details...</p>
  }

  if (data.status === 'unsupported') {
    return <p className="text-sm text-muted-foreground">.doc files aren't automatically parsed. Upload a PDF or DOCX to see extracted details.</p>
  }

  if (data.status === 'failed') {
    return <div className="space-y-2">
      <p className="text-sm text-destructive">Couldn't extract details{data.error ? `: ${data.error}` : '.'}</p>
      <Button type="button" variant="outline" size="sm" onClick={() => retryExtraction.mutate(resumeId)} disabled={retryExtraction.isPending}>
        {retryExtraction.isPending ? 'Retrying...' : 'Retry'}
      </Button>
    </div>
  }

  const skills = data.skills ?? []
  const education = data.education ?? []
  const workExperience = data.work_experience ?? []
  const projects = data.projects ?? []
  const hasContactInfo = data.full_name || data.email || data.phone || data.linked_in || data.github || data.portfolio

  return <div className="space-y-3 text-sm">
    {hasContactInfo && <div className="space-y-0.5">
      {data.full_name && <p><span className="text-muted-foreground">Name:</span> {data.full_name}</p>}
      {data.email && <p><span className="text-muted-foreground">Email:</span> {data.email}</p>}
      {data.phone && <p><span className="text-muted-foreground">Phone:</span> {data.phone}</p>}
      {data.linked_in && <p><span className="text-muted-foreground">LinkedIn:</span> <a href={data.linked_in} target="_blank" rel="noreferrer" className="text-primary hover:underline">{data.linked_in}</a></p>}
      {data.github && <p><span className="text-muted-foreground">GitHub:</span> <a href={data.github} target="_blank" rel="noreferrer" className="text-primary hover:underline">{data.github}</a></p>}
      {data.portfolio && <p><span className="text-muted-foreground">Portfolio:</span> <a href={data.portfolio} target="_blank" rel="noreferrer" className="text-primary hover:underline">{data.portfolio}</a></p>}
    </div>}
    {data.summary && <p className="text-muted-foreground pb-2">{data.summary}</p>}
    {skills.length > 0 && <ul className="flex flex-wrap gap-1.5">
      {skills.map((skill) => <li key={skill} className={cn(badgeVariants({ variant: 'secondary' }), 'h-auto max-w-full items-start whitespace-normal break-words text-left')}>{skill}</li>)}
    </ul>}
    {education.length > 0 && <div className="space-y-2">
      <p className="text-xs font-semibold text-heading pb-2">Education</p>
      {education.map((entry, i) => <div key={i} className="rounded-lg border border-border p-2">
        <p className="font-medium">{entry.degree} in {entry.major} — {entry.school_name}</p>
        {(entry.start_date || entry.end_date) && <p className="text-xs text-muted-foreground">{entry.start_date} – {entry.end_date}</p>}
      </div>)}
    </div>}
    {workExperience.length > 0 && <div className="space-y-2">
      <p className="text-xs font-semibold text-heading pb-2">Work experience</p>
      {workExperience.map((entry, i) => <div key={i} className="rounded-lg border border-border p-2">
        <p className="font-medium">{entry.job_title} — {entry.company}</p>
        <p className="text-xs text-muted-foreground">
          {entry.location}{entry.start_date ? ` · ${entry.start_date} – ${entry.end_date || 'present'}` : ''}
        </p>
        {(entry.bullets ?? []).length > 0 && <ul className="mt-1 space-y-0.5 text-xs">
          {(entry.bullets ?? []).map((bullet, bi) => <li key={bi}>• {bullet}</li>)}
        </ul>}
      </div>)}
    </div>}
    {projects.length > 0 && <div className="space-y-2">
      <p className="text-xs font-semibold text-heading pb-2">Projects</p>
      {projects.map((entry, i) => <div key={i} className="rounded-lg border border-border p-2">
        {entry.url ? <a href={entry.url} target="_blank" rel="noreferrer" className="font-medium text-primary hover:underline">{entry.name}</a> : <p className="font-medium">{entry.name}</p>}
        {(entry.technologies ?? []).length > 0 && <p className="text-xs text-muted-foreground">{(entry.technologies ?? []).join(', ')}</p>}
        {(entry.bullets ?? []).length > 0 && <ul className="mt-1 space-y-0.5 text-xs">
          {(entry.bullets ?? []).map((bullet, bi) => <li key={bi}>• {bullet}</li>)}
        </ul>}
      </div>)}
    </div>}
    {!hasContactInfo && !data.summary && skills.length === 0 && education.length === 0 && workExperience.length === 0 && projects.length === 0 &&
      <p className="text-sm text-muted-foreground">No details were found in this resume.</p>}
  </div>
}

function formatFileSize(bytes: number) {
  if (bytes < 1024 * 1024) return `${Math.max(1, Math.ceil(bytes / 1024))} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}
