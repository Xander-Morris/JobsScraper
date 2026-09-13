import { createFileRoute, Link, useBlocker } from '@tanstack/react-router'
import { PrinterIcon, SaveIcon, SparklesIcon } from 'lucide-react'
import { useEffect, useState } from 'react'
import {
  useGenerateTailoredResumeMutation,
  useJobQuery,
  useSaveTailoredResumeMutation,
  useTailoredResumeQuery
} from '@/src/api/jobs'
import { useProfileQuery, useResumeExtractionQuery } from '@/src/api/profile'
import type { TailoredResumeContent } from '@/src/api/schemas'
import ResumeEditor from '@/src/components/jobs/tailored-resume/resume-editor'
import ResumePreview from '@/src/components/jobs/tailored-resume/resume-preview'
import { Button } from '@/src/components/ui/button'
import { Skeleton } from '@/src/components/ui/skeleton'
import slugify from '@/src/lib/slugify'
import { useAuth } from '@/src/stores/profile-store'

export const Route = createFileRoute('/jobs/$jobId_/resume')({
  component: TailoredResumePage
})

function TailoredResumePage() {
  const { jobId: rawJobId } = Route.useParams()
  const jobId = Number(rawJobId)
  const { token } = useAuth()
  const { data: job } = useJobQuery(jobId, token)
  const { data: tailored, isLoading, isError, error } = useTailoredResumeQuery(token, jobId)
  const { data: profile } = useProfileQuery(token)
  const activeResume = profile?.resumes?.find((resume) => resume.is_active) ?? null
  const { data: extraction } = useResumeExtractionQuery(token, activeResume?.id ?? 0, {
    enabled: activeResume != null
  })
  const save = useSaveTailoredResumeMutation(token, jobId)
  const regenerate = useGenerateTailoredResumeMutation(token)
  const [draft, setDraft] = useState<TailoredResumeContent | null>(null)

  // Saving or regenerating replaces the query data, which resets the draft.
  useEffect(() => {
    if (tailored) setDraft(tailored.content)
  }, [tailored])

  const dirty = !!tailored && draft !== null && draft !== tailored.content
  const actionError = [save.error, regenerate.error].find((err) => err instanceof Error)?.message

  useBlocker({
    shouldBlockFn: () => dirty && !window.confirm('Discard unsaved changes?'),
    enableBeforeUnload: () => dirty
  })

  function handleRegenerate() {
    const message = dirty
      ? 'Regenerating replaces this resume, including unsaved changes. Continue?'
      : 'Regenerating replaces your edits to this resume. Continue?'
    if (!window.confirm(message)) return
    regenerate.mutate(jobId)
  }

  function handlePrint() {
    const previousTitle = document.title
    document.title = [draft?.full_name, job?.company, 'resume'].filter(Boolean).map((part) => slugify(part!)).join('-')
    window.print()
    document.title = previousTitle
  }

  return (
    <div className="mx-auto max-w-6xl px-6 py-10 text-left print:m-0 print:max-w-none print:p-0">
      <div className="print:hidden">
        <Link
          to="/jobs/$jobId"
          params={{ jobId: rawJobId }}
          className="text-sm text-muted-foreground no-underline hover:text-brand"
        >
          ← Back to job
        </Link>

        <div className="mt-4 flex flex-wrap items-end justify-between gap-3">
          <div>
            <h1 className="text-2xl font-semibold text-heading">Tailored resume</h1>
            {job && (
              <p className="mt-1 text-sm text-muted-foreground">
                {job.title} · {job.company}
              </p>
            )}
          </div>

          {draft && (
            <div className="flex flex-wrap items-center gap-2">
              {dirty && <span className="text-xs text-muted-foreground">Unsaved changes</span>}
              <Button type="button" variant="ghost" size="sm" onClick={handleRegenerate} disabled={regenerate.isPending}>
                <SparklesIcon aria-hidden="true" /> {regenerate.isPending ? 'Tailoring…' : 'Regenerate'}
              </Button>
              <Button type="button" variant="outline" size="sm" onClick={handlePrint}>
                <PrinterIcon aria-hidden="true" /> Download PDF
              </Button>
              <Button type="button" size="sm" onClick={() => save.mutate(draft)} disabled={!dirty || save.isPending}>
                <SaveIcon aria-hidden="true" /> {save.isPending ? 'Saving…' : 'Save'}
              </Button>
            </div>
          )}
        </div>

        {tailored?.stale && (
          <p role="status" className="mt-4 rounded-md border border-border bg-muted p-3 text-sm">
            Your resume changed since this was tailored. Regenerate to pick up the changes.
          </p>
        )}

        {actionError && (
          <p role="alert" className="mt-4 text-sm text-destructive">
            {actionError}
          </p>
        )}

        {isError && (
          <p role="alert" className="mt-6 text-sm text-destructive">
            {error.message}
          </p>
        )}

        {!token && <p className="mt-6 text-sm text-muted-foreground">Log in to view your tailored resume.</p>}

        {tailored === null && (
          <p className="mt-6 text-sm text-muted-foreground">
            No tailored resume for this job yet. Go back to the job and tailor one from apply assist.
          </p>
        )}

        {isLoading && (
          <div className="mt-6 space-y-3" aria-label="Loading tailored resume">
            <Skeleton className="h-24 w-full" />
            <Skeleton className="h-64 w-full" />
          </div>
        )}
      </div>

      {draft && (
        <div className="mt-6 grid items-start gap-8 lg:grid-cols-2 print:mt-0 print:block">
          <div className="print:hidden">
            <ResumeEditor
              content={draft}
              onChange={setDraft}
              extraction={tailored?.stale ? undefined : extraction}
            />
          </div>
          <div className="lg:sticky lg:top-6">
            <ResumePreview content={draft} />
          </div>
        </div>
      )}
    </div>
  )
}
