import { Link } from '@tanstack/react-router'
import { MapPinIcon } from 'lucide-react'
import type { JobListItem } from '../../api/schemas'
import { formatRelativeDate, formatSalary, formatWorkplaceType, matchFitLabel } from '../../lib/format'
import { cn } from '../../lib/utils'
import { Badge } from '../ui/badge'

const MAX_TAGS = 5

export function JobCard({ job, bestMatchScore }: { job: JobListItem; bestMatchScore?: number }) {
  const salary = formatSalary(job.salary_min, job.salary_max)
  const fitLabel = matchFitLabel(job.match_score, bestMatchScore)
  const greatFit = fitLabel === 'Great fit'
  const extraTags = job.tags.length - MAX_TAGS

  return (
    <li>
      <Link
        to="/jobs/$jobId"
        params={{ jobId: String(job.id) }}
        className={cn(
          'group block px-4 py-4 no-underline transition-colors outline-none hover:bg-white/[0.025] focus-visible:bg-white/[0.04] focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset sm:px-5',
          greatFit && 'shadow-[inset_2px_0_0_var(--primary)]'
        )}
      >
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0">
            <h3 className="text-[15px] leading-snug font-medium text-heading transition-colors group-hover:text-primary">
              {job.title}
            </h3>
            <p className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-0.5 text-sm text-muted-foreground">
              <span className="text-secondary-foreground">{job.company}</span>
              {job.location && (
                <span className="inline-flex items-center gap-1">
                  <MapPinIcon aria-hidden="true" className="size-3.5 opacity-70" />
                  {job.location}
                </span>
              )}
            </p>
          </div>
          <div className="shrink-0 text-right tabular-nums">
            {salary && <p className="text-sm font-medium text-heading">{salary}</p>}
            <p className="mt-0.5 text-xs whitespace-nowrap text-muted-foreground">
              {formatRelativeDate(job.posted_at)}
            </p>
          </div>
        </div>

        <div className="mt-3 flex flex-wrap items-center gap-1.5">
          {fitLabel && <Badge variant={greatFit ? 'default' : 'outline'}>{fitLabel}</Badge>}
          {job.applied && <Badge variant="secondary">Applied</Badge>}
          <Badge variant="secondary">{formatWorkplaceType(job.workplace_type)}</Badge>
          {job.tags.slice(0, MAX_TAGS).map((tag) => (
            <Badge key={tag} variant="outline">
              {tag}
            </Badge>
          ))}
          {extraTags > 0 && <span className="px-1 text-xs text-muted-foreground">+{extraTags}</span>}
        </div>
      </Link>
    </li>
  )
}
