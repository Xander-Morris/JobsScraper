import { createFileRoute, Link } from '@tanstack/react-router'
import { useJobQuery } from '../../api/jobs'
import { formatRelativeDate, formatSalary, formatWorkplaceType } from '../../lib/format'
import { Badge } from '../../components/ui/badge'
import { buttonVariants } from '../../components/ui/button'
import { Skeleton } from '../../components/ui/skeleton'
import { cn } from '../../lib/utils'

export const Route = createFileRoute('/jobs/$jobId')({
  component: JobDetailPage,
})

function JobDetailPage() {
  const { jobId } = Route.useParams()
  const { data: job, isLoading, isError, error } = useJobQuery(Number(jobId))

  return (
    <div className="mx-auto max-w-3xl px-6 py-10 text-left">
      <Link to="/" className="text-sm text-muted-foreground no-underline hover:text-brand">
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
          <h1 className="text-2xl font-semibold text-heading">{job.title}</h1>
          <p className="mt-1 text-muted-foreground">
            {job.company} · {job.location}
          </p>

          <div className="mt-4 flex flex-wrap items-center gap-1.5">
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

          <p className="mt-6 whitespace-pre-wrap text-sm leading-relaxed text-muted-foreground">
            {job.description}
          </p>

          <a
            href={job.url}
            target="_blank"
            rel="noreferrer"
            className={cn(buttonVariants({ variant: 'default' }), 'mt-8')}
          >
            View original posting →
          </a>
        </article>
      )}
    </div>
  )
}
