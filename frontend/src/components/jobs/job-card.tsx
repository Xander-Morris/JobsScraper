import { Link } from '@tanstack/react-router'
import type { Job } from '../../api/schemas'
import { formatRelativeDate, formatSalary, formatWorkplaceType, matchFitLabel } from '../../lib/format'
import { Badge } from '../ui/badge'
import { Card, CardContent } from '../ui/card'

export function JobCard({ job, bestMatchScore }: { job: Job; bestMatchScore?: number }) {
  const salary = formatSalary(job.salary_min, job.salary_max)
  const fitLabel = matchFitLabel(job.match_score, bestMatchScore)

  return (
    <li>
      <Link
        to="/jobs/$jobId"
        params={{ jobId: String(job.id) }}
        className="group block no-underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent-border"
      >
        <Card className="text-left transition-colors group-hover:ring-accent-border">
          <CardContent className="flex items-start justify-between gap-4">
            <div>
              <span className="font-medium text-heading group-hover:text-brand">{job.title}</span>
              <p className="mt-0.5 text-sm text-muted-foreground">
                {job.company} · {job.location}
              </p>
            </div>
            <span className="shrink-0 whitespace-nowrap text-xs text-muted-foreground">
              {formatRelativeDate(job.posted_at)}
            </span>
          </CardContent>

          <CardContent className="flex flex-wrap items-center gap-1.5">
            {fitLabel && <Badge variant={fitLabel === 'Great fit' ? 'default' : 'secondary'}>{fitLabel}</Badge>}
            {job.applied && <Badge variant="outline">Applied</Badge>}
            <Badge variant="secondary">{formatWorkplaceType(job.workplace_type)}</Badge>
            {salary && <Badge variant="secondary">{salary}</Badge>}
            {job.tags.map((tag) => (
              <Badge key={tag} variant="outline">
                {tag}
              </Badge>
            ))}
          </CardContent>
        </Card>
      </Link>
    </li>
  )
}
