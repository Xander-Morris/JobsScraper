import OpenAndToggle from '@/src/components/jobs/job-apply-panel/open-and-toggle'
import { createFileRoute, Link } from '@tanstack/react-router'
import { useJobQuery } from '../../api/jobs'
import JobApplyPanel from '../../components/jobs/job-apply-panel/job-apply-panel'
import { Badge } from '../../components/ui/badge'
import { Skeleton } from '../../components/ui/skeleton'
import { formatRelativeDate, formatSalary, formatWorkplaceType, matchFitLabel } from '../../lib/format'
import { cn } from '../../lib/utils'
import { useAuth } from '../../stores/profile-store'

export const Route = createFileRoute('/jobs/$jobId')({
  component: JobDetailPage
})

function JobDetailPage() {
  const { jobId } = Route.useParams()
  const { isAuthenticated, token } = useAuth()
  const { data: job, isLoading, isError, error } = useJobQuery(Number(jobId), token)
  const fitLabel = job ? matchFitLabel(job.match_score) : null

  return (
    <div className="mx-auto max-w-6xl px-6 py-10 text-left">
      <Link to="/jobs" className="text-sm text-muted-foreground no-underline hover:text-brand">
        ← Back to jobs
      </Link>

      {isLoading && (
        <div className="mt-6 space-y-3" aria-label="Loading job">
          <Skeleton className="h-8 w-2/3" />
          <Skeleton className="h-4 w-1/3" />
          <Skeleton className="h-24 w-full" />
        </div>
      )}
      {isError && (
        <p role="alert" className="mt-6 text-sm text-destructive">
          {error.message}
        </p>
      )}

      {job && (
        <article className="mt-4">
          <header>
            <h1 className="text-2xl font-semibold text-heading">{job.title}</h1>
            <p className="mt-1 text-muted-foreground">
              {job.company} · {job.location}
            </p>

            <div className="mt-4 flex flex-wrap items-center gap-1.5 pb-2">
              {fitLabel && <Badge variant="default">{fitLabel}</Badge>}
              {job.applied && <Badge variant="outline">Applied</Badge>}
              <Badge variant="secondary">{formatWorkplaceType(job.workplace_type)}</Badge>
              {formatSalary(job.salary_min, job.salary_max) && (
                <Badge variant="secondary">{formatSalary(job.salary_min, job.salary_max)}</Badge>
              )}
              <Badge variant="outline">{formatRelativeDate(job.posted_at)}</Badge>
              {job.tags.map((tag) => (
                <Badge key={tag} variant="outline">
                  {tag}
                </Badge>
              ))}
            </div>

            <OpenAndToggle job={job}/>
          </header>

          <div
            className={cn(
              'mt-8 grid items-start gap-8',
              isAuthenticated && token && 'lg:grid-cols-[minmax(0,1fr)_22rem]'
            )}
          >
            {isAuthenticated && token && (
              // Assist comes first on narrow screens and rides along in a
              // sticky rail on wide ones, so it never sits below the fold.
              <aside className="order-1 lg:order-2 lg:sticky lg:top-6 lg:max-h-[calc(100vh-3rem)] lg:overflow-y-auto">
                <JobApplyPanel job={job} />
              </aside>
            )}

            <section className="order-2 lg:order-1">
              <h2 className="text-sm font-semibold text-heading">Job description</h2>
              <p className="mt-2 max-w-3xl whitespace-pre-wrap text-sm leading-relaxed text-muted-foreground">
                {job.description}
              </p>
            </section>
          </div>
        </article>
      )}
    </div>
  )
}
