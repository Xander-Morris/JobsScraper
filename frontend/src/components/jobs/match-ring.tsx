import { matchLabel, STRONG_MATCH } from '../../lib/format'
import { cn } from '../../lib/utils'

const SIZE = 52
const STROKE = 4
const RADIUS = (SIZE - STROKE) / 2
const CIRCUMFERENCE = 2 * Math.PI * RADIUS

function arcClass(percent: number): string {
  if (percent >= STRONG_MATCH) return 'stroke-primary'
  if (percent >= 65) return 'stroke-chart-2'
  if (percent >= 40) return 'stroke-chart-3'
  return 'stroke-muted-foreground/40'
}

export function MatchRing({ percent, className }: { percent: number; className?: string }) {
  const label = matchLabel(percent)

  return (
    <div
      role="img"
      aria-label={`${percent}% match: ${label}`}
      title={`Better fit for your resume than ${percent}% of open jobs`}
      className={cn('flex flex-col items-center gap-1', className)}
    >
      <div aria-hidden="true" className="relative size-13">
        <svg viewBox={`0 0 ${SIZE} ${SIZE}`} className="size-full -rotate-90">
          <circle cx={SIZE / 2} cy={SIZE / 2} r={RADIUS} fill="none" strokeWidth={STROKE} className="stroke-border" />
          {percent > 0 && (
            <circle
              cx={SIZE / 2}
              cy={SIZE / 2}
              r={RADIUS}
              fill="none"
              strokeWidth={STROKE}
              strokeLinecap="round"
              strokeDasharray={CIRCUMFERENCE}
              strokeDashoffset={CIRCUMFERENCE * (1 - percent / 100)}
              className={arcClass(percent)}
            />
          )}
        </svg>
        <span className="absolute inset-0 flex items-center justify-center text-sm font-semibold text-heading tabular-nums">
          {percent}
          <span className="text-[10px] font-medium text-muted-foreground">%</span>
        </span>
      </div>
      <span
        aria-hidden="true"
        className="text-[10px] font-medium tracking-wide whitespace-nowrap text-muted-foreground uppercase"
      >
        {label}
      </span>
    </div>
  )
}
