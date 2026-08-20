import { Link } from '@tanstack/react-router'
import type { Job } from '../api/schemas'
import { formatRelativeDate, formatSalary, formatWorkplaceType } from '../lib/format'
import { Badge } from './ui/badge'
import { Card, CardContent } from './ui/card'

export function JobCard({ job }: { job: Job }) {
  const salary = formatSalary(job.salary_min, job.salary_max)

  return (
    <li>
      <Card className="group text-left transition-colors hover:ring-accent-border">
        <CardContent className="flex items-start justify-between gap-4">
          <div>
            <Link
              to="/jobs/$jobId"
              params={{ jobId: String(job.id) }}
              className="font-medium text-heading no-underline group-hover:text-brand"
            >
              {job.title}
            </Link>
            <p className="mt-0.5 text-sm text-muted-foreground">
              {job.company} · {job.location}
            </p>
          </div>
          <span className="shrink-0 whitespace-nowrap text-xs text-muted-foreground">
            {formatRelativeDate(job.posted_at)}
          </span>
        </CardContent>

        <CardContent className="flex flex-wrap items-center gap-1.5">
          <Badge variant="secondary">{formatWorkplaceType(job.workplace_type)}</Badge>
          {salary && <Badge variant="secondary">{salary}</Badge>}
          {job.tags.map((tag) => (
            <Badge key={tag} variant="outline">
              {tag}
            </Badge>
          ))}
        </CardContent>
      </Card>
    </li>
  )
}
