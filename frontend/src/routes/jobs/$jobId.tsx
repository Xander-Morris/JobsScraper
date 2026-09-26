import OpenAndToggle from '@/src/components/jobs/job-apply-panel/open-and-toggle'
import { createFileRoute, Link } from '@tanstack/react-router'
import { ArrowLeftIcon, MapPinIcon } from 'lucide-react'
import { jobQueryOptions, useJobQuery } from '@/src/hooks/use-jobs'
import { profileQueryOptions } from '@/src/hooks/use-profile'
import JobApplyPanel from '../../components/jobs/job-apply-panel/job-apply-panel'
import { Badge } from '../../components/ui/badge'
import { Skeleton } from '../../components/ui/skeleton'
import { formatRelativeDate, formatSalary, formatWorkplaceType, matchFitLabel } from '../../lib/format'
import { cn } from '../../lib/utils'
import { useAuth } from '@/src/stores/auth-store'

export const Route = createFileRoute('/jobs/$jobId')({
  loader: ({ context: { auth, queryClient }, params }) => {
    if (auth.isInitializing) return
    void queryClient.prefetchQuery(jobQueryOptions(Number(params.jobId), auth.isAuthenticated))
    if (auth.isAuthenticated) void queryClient.prefetchQuery(profileQueryOptions)
  },
  component: JobDetailPage
})

function JobDetailPage() {
  const { jobId } = Route.useParams()
  const { isAuthenticated } = useAuth()
  const { data: job, isPending, isError, error } = useJobQuery(Number(jobId))
  const fitLabel = job ? matchFitLabel(job.match_score) : null

  return (
    <div className="mx-auto max-w-6xl px-4 py-10 sm:px-6">
      <Link
        to="/jobs"
        className="inline-flex items-center gap-1.5 text-sm text-muted-foreground no-underline hover:text-heading"
      >
        <ArrowLeftIcon aria-hidden="true" className="size-4" />
        Back to jobs
      </Link>

      {isPending && (
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
        <article className="mt-6">
          <header className="border-b border-border pb-8">
            <h1 className="max-w-3xl text-3xl font-medium tracking-[-0.03em] sm:text-4xl">{job.title}</h1>
            <p className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-1 text-muted-foreground">
              <span className="text-secondary-foreground">{job.company}</span>
              {job.location && (
                <span className="inline-flex items-center gap-1.5">
                  <MapPinIcon aria-hidden="true" className="size-4 opacity-70" />
                  {job.location}
                </span>
              )}
            </p>

            <div className="mt-5 mb-6 flex flex-wrap items-center gap-1.5">
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

            <OpenAndToggle job={job} />
          </header>

          <div className={cn('mt-8 grid items-start gap-10', isAuthenticated && 'lg:grid-cols-[minmax(0,1fr)_22rem]')}>
            {isAuthenticated && (
              <aside className="order-1 lg:sticky lg:top-20 lg:order-2 lg:max-h-[calc(100vh-6rem)] lg:overflow-y-auto">
                <JobApplyPanel job={job} />
              </aside>
            )}

            <section className="order-2 lg:order-1">
              <h2 className="text-sm font-medium text-muted-foreground">Job description</h2>
              <p className="mt-3 max-w-[68ch] text-[15px] leading-7 whitespace-pre-wrap text-secondary-foreground">
                {job.description}
              </p>
            </section>
          </div>
        </article>
      )}
    </div>
  )
}
