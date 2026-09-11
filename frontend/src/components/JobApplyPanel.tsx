import {
  CheckIcon,
  ClipboardIcon,
  CornerDownLeftIcon,
  DownloadIcon,
  ExternalLinkIcon,
  SparklesIcon,
} from 'lucide-react'
import { useEffect, useRef, useState, type ReactNode } from 'react'
import {
  useGenerateApplicationContentMutation,
  useMarkJobAppliedMutation,
  useUnmarkJobAppliedMutation,
} from '../api/jobs'
import { useDownloadResumeMutation, useProfileQuery, useResumeExtractionQuery } from '../api/profile'
import type { GeneratedContent, Job, Resume } from '../api/schemas'
import { matchSkills } from '../lib/skills'
import { Badge } from './ui/badge'
import { Button } from './ui/button'
import { Card, CardContent, CardHeader } from './ui/card'

type ContactField = { label: string; value: string }

function downloadBlob(blob: Blob, fileName: string) {
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = fileName
  anchor.click()
  URL.revokeObjectURL(url)
}

function slugify(value: string): string {
  return value.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || 'job'
}

// useCopied wraps the clipboard write plus the short-lived "Copied!" flag so
// every copy affordance on the panel behaves the same way.
function useCopied() {
  const [copied, setCopied] = useState(false)
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => () => void (timer.current && clearTimeout(timer.current)), [])

  async function copy(text: string) {
    await navigator.clipboard.writeText(text)
    setCopied(true)
    if (timer.current) clearTimeout(timer.current)
    timer.current = setTimeout(() => setCopied(false), 2000)
  }

  return { copied, copy }
}

// CopyField is one row of the application form: click it and only that value
// lands on the clipboard, so filling a form is one click per field instead of
// pasting a blob and editing it back apart.
function CopyField({ label, value }: ContactField) {
  const { copied, copy } = useCopied()

  return (
    <button
      type="button"
      onClick={() => void copy(value)}
      aria-label={`Copy ${label}`}
      className="flex w-full items-center justify-between gap-3 rounded-md px-2 py-1 text-left transition-colors hover:bg-muted"
    >
      <span className="min-w-0">
        <span className="block text-[0.7rem] uppercase tracking-wide text-muted-foreground">{label}</span>
        <span className="block truncate text-xs">{value}</span>
      </span>
      {copied ? (
        <CheckIcon aria-hidden="true" className="size-3.5 shrink-0 text-brand" />
      ) : (
        <ClipboardIcon aria-hidden="true" className="size-3.5 shrink-0 text-muted-foreground" />
      )}
    </button>
  )
}

function SectionHeading({ step, title, action }: { step: number; title: string; action?: ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-2">
      <p className="text-xs font-medium text-heading">
        <span className="text-muted-foreground">{step}.</span> {title}
      </p>
      {action}
    </div>
  )
}

function ContactSection({ fields }: { fields: ContactField[] }) {
  const { copied, copy } = useCopied()
  const allText = fields.map((field) => `${field.label}: ${field.value}`).join('\n')

  return (
    <section className="space-y-1">
      <SectionHeading
        step={1}
        title="Your details"
        action={
          <Button type="button" variant="ghost" size="xs" onClick={() => void copy(allText)}>
            {copied ? <CheckIcon aria-hidden="true" /> : <ClipboardIcon aria-hidden="true" />} Copy all
          </Button>
        }
      />
      <p className="px-2 text-[0.7rem] text-muted-foreground">Click a field to copy just that value.</p>
      <div className="space-y-0.5">
        {fields.map((field) => (
          <CopyField key={field.label} {...field} />
        ))}
      </div>
    </section>
  )
}

function ResumeSection({
  activeResume,
  downloadResume,
}: {
  activeResume: Resume | null
  downloadResume: ReturnType<typeof useDownloadResumeMutation>
}) {
  function handleDownload() {
    if (!activeResume) return
    downloadResume.mutate(activeResume.id, {
      onSuccess: (file) => downloadBlob(file, activeResume.file_name),
    })
  }

  return (
    <section className="space-y-1.5 border-t border-border pt-3">
      <SectionHeading step={2} title="Resume" />
      {activeResume ? (
        <div className="flex items-center justify-between gap-2">
          <span className="min-w-0 truncate text-xs text-muted-foreground">{activeResume.file_name}</span>
          <Button
            type="button"
            variant="outline"
            size="xs"
            onClick={handleDownload}
            disabled={downloadResume.isPending}
          >
            <DownloadIcon aria-hidden="true" /> Download
          </Button>
        </div>
      ) : (
        <p className="text-xs text-muted-foreground">No active resume. Set one in your profile to attach it here.</p>
      )}
    </section>
  )
}

// FitSection turns the posting's requirements into something actionable: what
// to lead with, and what the application will likely probe that the resume
// doesn't cover.
function FitSection({ matched, missing }: { matched: string[]; missing: string[] }) {
  if (matched.length === 0 && missing.length === 0) return null

  return (
    <section className="space-y-2 border-t border-border pt-3">
      <p className="text-xs font-medium text-heading">Fit check</p>

      {matched.length > 0 && (
        <div className="space-y-1">
          <p className="text-[0.7rem] text-muted-foreground">On your resume, lead with these</p>
          <div className="flex flex-wrap gap-1">
            {matched.slice(0, 12).map((skill) => (
              <Badge key={skill} variant="secondary">
                {skill}
              </Badge>
            ))}
          </div>
        </div>
      )}

      {missing.length > 0 && (
        <div className="space-y-1">
          <p className="text-[0.7rem] text-muted-foreground">Asked for, not on your resume</p>
          <div className="flex flex-wrap gap-1">
            {missing.slice(0, 12).map((skill) => (
              <Badge key={skill} variant="outline">
                {skill}
              </Badge>
            ))}
          </div>
        </div>
      )}
    </section>
  )
}

function CoverLetterSection({
  job,
  data,
  onRegenerate,
  isPending,
}: {
  job: Job
  data: GeneratedContent
  onRegenerate: () => void
  isPending: boolean
}) {
  const [draft, setDraft] = useState(data.cover_letter)
  const { copied, copy } = useCopied()

  // A regenerate replaces the draft; edits only survive until then.
  useEffect(() => setDraft(data.cover_letter), [data.cover_letter])

  function appendBullet(bullet: string) {
    setDraft((current) => `${current.trimEnd()}\n\n• ${bullet}`)
  }

  return (
    <div className="space-y-3">
      <textarea
        value={draft}
        onChange={(event) => setDraft(event.target.value)}
        rows={12}
        aria-label="Cover letter draft"
        className="w-full resize-y rounded-md border border-border bg-background p-2 text-xs leading-relaxed"
      />

      <div className="flex flex-wrap gap-2">
        <Button type="button" variant="outline" size="xs" onClick={() => void copy(draft)}>
          {copied ? <CheckIcon aria-hidden="true" /> : <ClipboardIcon aria-hidden="true" />}{' '}
          {copied ? 'Copied!' : 'Copy letter'}
        </Button>
        <Button
          type="button"
          variant="outline"
          size="xs"
          onClick={() =>
            downloadBlob(
              new Blob([draft], { type: 'text/plain;charset=utf-8' }),
              `${slugify(job.company)}-${slugify(job.title)}-cover-letter.txt`,
            )
          }
        >
          <DownloadIcon aria-hidden="true" /> Download .txt
        </Button>
        <Button type="button" variant="ghost" size="xs" onClick={onRegenerate} disabled={isPending}>
          <SparklesIcon aria-hidden="true" /> {isPending ? 'Generating…' : 'Regenerate'}
        </Button>
      </div>

      {data.tailored_bullets.length > 0 && (
        <div className="space-y-1.5">
          <p className="text-[0.7rem] text-muted-foreground">
            Tailored bullets: add one to the letter or paste it into the form
          </p>
          <ul className="space-y-1.5">
            {data.tailored_bullets.map((bullet, index) => (
              <li key={index} className="flex items-start justify-between gap-2 text-xs">
                <span>{bullet}</span>
                <span className="flex shrink-0 gap-0.5">
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-xs"
                    onClick={() => appendBullet(bullet)}
                    aria-label="Add bullet to letter"
                  >
                    <CornerDownLeftIcon aria-hidden="true" />
                  </Button>
                  <BulletCopyButton bullet={bullet} />
                </span>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  )
}

function BulletCopyButton({ bullet }: { bullet: string }) {
  const { copied, copy } = useCopied()

  return (
    <Button
      type="button"
      variant="ghost"
      size="icon-xs"
      onClick={() => void copy(bullet)}
      aria-label="Copy bullet"
    >
      {copied ? <CheckIcon aria-hidden="true" /> : <ClipboardIcon aria-hidden="true" />}
    </Button>
  )
}

// JobAppliedToggle lives in the page header so the applied state is reachable
// without scrolling past the description.
export function JobAppliedToggle({ token, job }: { token: string; job: Job }) {
  const markApplied = useMarkJobAppliedMutation(token)
  const unmarkApplied = useUnmarkJobAppliedMutation(token)

  return job.applied ? (
    <Button
      type="button"
      variant="secondary"
      onClick={() => unmarkApplied.mutate(job.id)}
      disabled={unmarkApplied.isPending}
    >
      <CheckIcon aria-hidden="true" /> Applied
    </Button>
  ) : (
    <Button type="button" variant="outline" onClick={() => markApplied.mutate(job.id)} disabled={markApplied.isPending}>
      Mark as applied
    </Button>
  )
}

export function JobApplyPanel({ token, job }: { token: string; job: Job }) {
  const { data: profile, isLoading: profileLoading } = useProfileQuery(token)
  const activeResume = profile?.resumes?.find((resume) => resume.is_active) ?? null
  const { data: extraction } = useResumeExtractionQuery(token, activeResume?.id ?? 0, {
    enabled: activeResume != null,
  })
  const markApplied = useMarkJobAppliedMutation(token)
  const downloadResume = useDownloadResumeMutation(token)
  const generateContent = useGenerateApplicationContentMutation(token)
  const downloadError = downloadResume.error instanceof Error ? downloadResume.error.message : null
  const generateError = generateContent.error instanceof Error ? generateContent.error.message : null
  const resumeReady = extraction?.status === 'completed'
  const { matched, missing } = matchSkills(job.tags, job.description, extraction?.skills)

  if (profileLoading || !profile) return null

  const fields: ContactField[] = [
    { label: 'Name', value: profile.name },
    { label: 'Email', value: profile.email },
    { label: 'Phone', value: extraction?.phone },
    { label: 'LinkedIn', value: profile.linked_in },
    { label: 'GitHub', value: profile.github },
    { label: 'Portfolio', value: profile.portfolio },
  ].filter((field): field is ContactField => !!field.value)

  // Opening the posting is the moment the application actually starts, so the
  // applied flag is set here rather than left as a chore for later.
  function openAndTrack() {
    window.open(job.url, '_blank', 'noopener,noreferrer')
    if (!job.applied) markApplied.mutate(job.id)
  }

  return (
    <Card>
      <CardHeader>
        <h2 className="text-sm font-semibold text-heading">Apply assist</h2>
        <p className="text-xs text-muted-foreground">
          Everything {job.company}&rsquo;s form asks for, one click away.
        </p>
      </CardHeader>
      <CardContent className="space-y-3 text-sm">
        <Button type="button" variant="default" className="w-full" onClick={openAndTrack}>
          <ExternalLinkIcon aria-hidden="true" />
          {job.applied ? 'Open application' : 'Open application & mark applied'}
        </Button>

        {fields.length > 0 ? (
          <ContactSection fields={fields} />
        ) : (
          <p className="text-xs text-muted-foreground">Fill in your profile to use apply assist.</p>
        )}

        <ResumeSection activeResume={activeResume} downloadResume={downloadResume} />

        <FitSection matched={matched} missing={missing} />

        <section className="space-y-2 border-t border-border pt-3">
          <SectionHeading step={3} title="Cover letter" />
          {!resumeReady && (
            <p className="text-xs text-muted-foreground">
              {activeResume
                ? 'Waiting on resume parsing before a letter can be drafted.'
                : 'Upload and activate a resume to draft a letter for this role.'}
            </p>
          )}
          {resumeReady && !generateContent.data && (
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => generateContent.mutate(job.id)}
              disabled={generateContent.isPending}
            >
              <SparklesIcon aria-hidden="true" />
              {generateContent.isPending ? 'Generating…' : 'Draft letter for this role'}
            </Button>
          )}
          {generateContent.data && (
            <CoverLetterSection
              job={job}
              data={generateContent.data}
              onRegenerate={() => generateContent.mutate(job.id)}
              isPending={generateContent.isPending}
            />
          )}
        </section>

        {downloadError && (
          <p role="alert" className="text-xs text-destructive">
            {downloadError}
          </p>
        )}

        {generateError && (
          <p role="alert" className="text-xs text-destructive">
            {generateError}
          </p>
        )}
      </CardContent>
    </Card>
  )
}
