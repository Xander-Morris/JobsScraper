import { CheckIcon, ClipboardIcon, DownloadIcon, SparklesIcon } from 'lucide-react'
import { useState } from 'react'
import { useGenerateApplicationContentMutation, useMarkJobAppliedMutation, useUnmarkJobAppliedMutation } from '../api/jobs'
import { useDownloadResumeMutation, useProfileQuery, useResumeExtractionQuery } from '../api/profile'
import type { Job } from '../api/schemas'
import { Button } from './ui/button'
import { Card, CardContent, CardHeader } from './ui/card'

export function JobApplyPanel({ token, job }: { token: string; job: Job }) {
  const { data: profile, isLoading: profileLoading } = useProfileQuery(token)
  const activeResume = profile?.resumes?.find((resume) => resume.is_active) ?? null
  const { data: extraction } = useResumeExtractionQuery(token, activeResume?.id ?? 0, {
    enabled: activeResume != null,
  })
  const markApplied = useMarkJobAppliedMutation(token)
  const unmarkApplied = useUnmarkJobAppliedMutation(token)
  const downloadResume = useDownloadResumeMutation(token)
  const generateContent = useGenerateApplicationContentMutation(token)
  const [copied, setCopied] = useState(false)
  const [copiedBulletIndex, setCopiedBulletIndex] = useState<number | null>(null)
  const downloadError = downloadResume.error instanceof Error ? downloadResume.error.message : null
  const generateError = generateContent.error instanceof Error ? generateContent.error.message : null
  const resumeReady = extraction?.status === 'completed'

  if (profileLoading || !profile) return null

  const fields = [
    { label: 'Name', value: profile.name },
    { label: 'Email', value: profile.email },
    { label: 'Phone', value: extraction?.phone },
    { label: 'LinkedIn', value: profile.linked_in },
    { label: 'GitHub', value: profile.github },
    { label: 'Portfolio', value: profile.portfolio },
  ].filter((field): field is { label: string; value: string } => !!field.value)

  async function handleCopy() {
    const text = fields.map((field) => `${field.label}: ${field.value}`).join('\n')
    await navigator.clipboard.writeText(text)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  function handleDownload() {
    if (!activeResume) return
    downloadResume.mutate(activeResume.id, {
      onSuccess: (file) => {
        const url = URL.createObjectURL(file)
        const anchor = document.createElement('a')
        anchor.href = url
        anchor.download = activeResume.file_name
        anchor.click()
        URL.revokeObjectURL(url)
      },
    })
  }

  async function handleCopyBullet(bullet: string, index: number) {
    await navigator.clipboard.writeText(bullet)
    setCopiedBulletIndex(index)
    setTimeout(() => setCopiedBulletIndex(null), 2000)
  }

  return (
    <Card className="mt-6">
      <CardHeader>
        <h2 className="text-sm font-semibold text-heading">Apply assist</h2>
        <p className="text-xs text-muted-foreground">
          Your info, ready to paste into {job.company}&rsquo;s application form.
        </p>
      </CardHeader>
      <CardContent className="space-y-3 text-sm">
        {fields.length > 0 ? (
          <div className="space-y-0.5">
            {fields.map((field) => (
              <p key={field.label}>
                <span className="text-muted-foreground">{field.label}:</span> {field.value}
              </p>
            ))}
          </div>
        ) : (
          <p className="text-muted-foreground">Fill in your profile to use application assist.</p>
        )}

        <div className="flex flex-wrap items-center gap-2 pt-1">
          <Button type="button" variant="outline" size="sm" onClick={() => void handleCopy()} disabled={fields.length === 0}>
            <ClipboardIcon aria-hidden="true" /> {copied ? 'Copied!' : 'Copy info'}
          </Button>

          {activeResume ? (
            <Button type="button" variant="outline" size="sm" onClick={handleDownload} disabled={downloadResume.isPending}>
              <DownloadIcon aria-hidden="true" /> Download resume
            </Button>
          ) : (
            <span className="text-xs text-muted-foreground">No active resume set.</span>
          )}

          {resumeReady ? (
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => generateContent.mutate(job.id)}
              disabled={generateContent.isPending}
            >
              <SparklesIcon aria-hidden="true" /> {generateContent.isPending ? 'Generating…' : 'Generate cover letter'}
            </Button>
          ) : null}

          {job.applied ? (
            <Button
              type="button"
              variant="secondary"
              size="sm"
              onClick={() => unmarkApplied.mutate(job.id)}
              disabled={unmarkApplied.isPending}
            >
              <CheckIcon aria-hidden="true" /> Applied
            </Button>
          ) : (
            <Button
              type="button"
              variant="default"
              size="sm"
              onClick={() => markApplied.mutate(job.id)}
              disabled={markApplied.isPending}
            >
              Mark as applied
            </Button>
          )}
        </div>

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

        {generateContent.data && (
          <div className="space-y-4 border-t border-border pt-3">
            <div className="space-y-1.5">
              <p className="text-xs font-medium text-heading">Cover letter</p>
              <textarea
                readOnly
                value={generateContent.data.cover_letter}
                rows={10}
                className="w-full resize-y rounded-md border border-border bg-background p-2 text-xs"
              />
            </div>

            {generateContent.data.tailored_bullets.length > 0 && (
              <div className="space-y-1.5">
                <p className="text-xs font-medium text-heading">Tailored bullets</p>
                <ul className="space-y-1.5">
                  {generateContent.data.tailored_bullets.map((bullet, index) => (
                    <li key={index} className="flex items-start justify-between gap-2 text-xs">
                      <span>{bullet}</span>
                      <Button
                        type="button"
                        variant="ghost"
                        size="icon-xs"
                        onClick={() => void handleCopyBullet(bullet, index)}
                        aria-label="Copy bullet"
                      >
                        {copiedBulletIndex === index ? <CheckIcon aria-hidden="true" /> : <ClipboardIcon aria-hidden="true" />}
                      </Button>
                    </li>
                  ))}
                </ul>
              </div>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  )
}
