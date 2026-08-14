import { Link } from '@tanstack/react-router'
import type { Job } from '../api/schemas'
import { formatRelativeDate, formatSalary, formatWorkplaceType } from '../lib/format'

export function JobCard({ job }: { job: Job }) {
  const salary = formatSalary(job.salary_min, job.salary_max)

  return (
    <li className="group rounded-lg border border-border p-4 text-left transition-colors hover:border-accent-border">
      <div className="flex items-start justify-between gap-4">
        <div>
          <Link
            to="/jobs/$jobId"
            params={{ jobId: String(job.id) }}
            className="font-medium text-heading no-underline group-hover:text-accent"
          >
            {job.title}
          </Link>
          <p className="mt-0.5 text-sm text-muted">
            {job.company} · {job.location}
          </p>
        </div>
        <span className="shrink-0 whitespace-nowrap text-xs text-muted">
          {formatRelativeDate(job.posted_at)}
        </span>
      </div>

      <div className="mt-3 flex flex-wrap items-center gap-1.5 font-mono text-xs">
        <span className="rounded bg-code-bg px-2 py-1 text-heading">
          {formatWorkplaceType(job.workplace_type)}
        </span>
        {salary && <span className="rounded bg-code-bg px-2 py-1 text-heading">{salary}</span>}
        {job.tags.map((tag) => (
          <span key={tag} className="rounded bg-code-bg px-2 py-1 text-muted">
            {tag}
          </span>
        ))}
      </div>
    </li>
  )
}
