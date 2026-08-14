import { createFileRoute, Link } from '@tanstack/react-router'
import { useJobQuery } from '../../api/jobs'
import { formatRelativeDate, formatSalary, formatWorkplaceType } from '../../lib/format'

export const Route = createFileRoute('/jobs/$jobId')({
  component: JobDetailPage,
})

function JobDetailPage() {
  const { jobId } = Route.useParams()
  const { data: job, isLoading, isError, error } = useJobQuery(Number(jobId))

  return (
    <div className="mx-auto max-w-3xl px-6 py-10 text-left">
      <Link to="/" className="text-sm text-muted no-underline hover:text-accent">
        ← Back to jobs
      </Link>

      {isLoading && <p className="mt-6 text-sm text-muted">Loading job…</p>}
      {isError && <p className="mt-6 text-sm text-red-500">{error.message}</p>}

      {job && (
        <article className="mt-4">
          <h1 className="text-2xl font-semibold text-heading">{job.title}</h1>
          <p className="mt-1 text-muted">
            {job.company} · {job.location}
          </p>

          <div className="mt-4 flex flex-wrap items-center gap-1.5 font-mono text-xs">
            <span className="rounded bg-code-bg px-2 py-1 text-heading">
              {formatWorkplaceType(job.workplace_type)}
            </span>
            {formatSalary(job.salary_min, job.salary_max) && (
              <span className="rounded bg-code-bg px-2 py-1 text-heading">
                {formatSalary(job.salary_min, job.salary_max)}
              </span>
            )}
            <span className="rounded bg-code-bg px-2 py-1 text-muted">
              {formatRelativeDate(job.posted_at)}
            </span>
            {job.tags.map((tag) => (
              <span key={tag} className="rounded bg-code-bg px-2 py-1 text-muted">
                {tag}
              </span>
            ))}
          </div>

          <p className="mt-6 whitespace-pre-wrap text-sm leading-relaxed text-muted">
            {job.description}
          </p>

          <a
            href={job.url}
            target="_blank"
            rel="noreferrer"
            className="mt-8 inline-block rounded bg-accent px-4 py-2 text-sm font-medium text-white no-underline"
          >
            View original posting →
          </a>
        </article>
      )}
    </div>
  )
}
