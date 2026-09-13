import { Link } from '@tanstack/react-router'
import { FileTextIcon, SparklesIcon } from 'lucide-react'
import { useGenerateTailoredResumeMutation, useTailoredResumeQuery } from '@/src/api/jobs'
import type { Job } from '@/src/api/schemas'
import { formatRelativeDate } from '@/src/lib/format'
import { cn } from '@/src/lib/utils'
import { Badge } from '../../ui/badge'
import { Button, buttonVariants } from '../../ui/button'
import SectionHeading from './section-heading'

export default function TailoredResumeSection({
  job,
  token,
  hasActiveResume,
  resumeReady
}: {
  job: Job
  token: string | null
  hasActiveResume: boolean
  resumeReady: boolean
}) {
  const { data: tailored, isLoading } = useTailoredResumeQuery(token, job.id)
  const generate = useGenerateTailoredResumeMutation(token)
  const error = generate.error instanceof Error ? generate.error.message : null

  function handleGenerate() {
    if (tailored && !window.confirm('Regenerating replaces your edits to this tailored resume. Continue?')) return
    generate.mutate(job.id)
  }

  return (
    <section className="space-y-2 border-t border-border pt-3">
      <SectionHeading step={3} title="Tailored resume" />

      {!resumeReady && !tailored && (
        <p className="text-xs text-muted-foreground">
          {hasActiveResume
            ? 'Waiting on resume parsing before it can be tailored.'
            : 'Upload and activate a resume to tailor it for this role.'}
        </p>
      )}

      {tailored && (
        <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
          <span>Updated {formatRelativeDate(tailored.updated_at)}</span>
          {tailored.stale && <Badge variant="outline">Resume changed</Badge>}
        </div>
      )}

      <div className="flex flex-wrap gap-2">
        {tailored && (
          <Link
            to="/jobs/$jobId/resume"
            params={{ jobId: String(job.id) }}
            className={cn(buttonVariants({ variant: 'outline', size: 'sm' }), 'no-underline')}
          >
            <FileTextIcon aria-hidden="true" /> Open editor
          </Link>
        )}
        {resumeReady && !isLoading && (
          <Button
            type="button"
            variant={tailored ? 'ghost' : 'outline'}
            size="sm"
            onClick={handleGenerate}
            disabled={generate.isPending}
          >
            <SparklesIcon aria-hidden="true" />
            {generate.isPending ? 'Tailoring…' : tailored ? 'Regenerate' : 'Tailor resume for this role'}
          </Button>
        )}
      </div>

      {error && (
        <p role="alert" className="text-xs text-destructive">
          {error}
        </p>
      )}
    </section>
  )
}
