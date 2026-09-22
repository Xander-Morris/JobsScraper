export function formatSalary(min: number | null, max: number | null): string | null {
  if (!min && !max) return null

  const fmt = (n: number) => `$${Math.round(n / 1000)}k`

  if (min != null && max != null) return `${fmt(min)} – ${fmt(max)}`
  if (min != null) return `${fmt(min)}+`
  return `up to ${fmt(max!)}`
}

export function formatWorkplaceType(type: string): string {
  switch (type) {
    case 'remote':
      return 'Remote'
    case 'hybrid':
      return 'Hybrid'
    case 'in_person':
      return 'In person'
    default:
      return 'Unknown'
  }
}

// matchFitLabel turns a raw match_score into a badge label. The score isn't on
// a fixed scale, so when `best` (top score in the set) is available, bucket
// relative to that instead of showing a misleading absolute percentage.
export function matchFitLabel(score: number | null | undefined, best?: number): string | null {
  if (score == null || score <= 0) return null

  if (best && best > 0) {
    const ratio = score / best
    if (ratio >= 0.7) return 'Great fit'
    if (ratio >= 0.4) return 'Good fit'
    return null
  }

  return 'Matches your resume'
}

export function formatRelativeDate(iso: string): string {
  const date = new Date(iso)
  const diffMs = Date.now() - date.getTime()
  const diffMinutes = Math.floor(diffMs / (1000 * 60))
  const diffDays = Math.floor(diffMinutes / (60 * 24))

  if (diffDays <= 0) {
    if (diffMinutes < 1) return 'just now'
    const hours = Math.floor(diffMinutes / 60)
    const minutes = diffMinutes % 60
    const plural = (n: number, unit: string) => `${n} ${unit}${n === 1 ? '' : 's'}`
    if (hours === 0) return `${plural(minutes, 'minute')} ago`
    if (minutes === 0) return `${plural(hours, 'hour')} ago`
    return `${plural(hours, 'hour')}, ${plural(minutes, 'minute')} ago`
  }
  if (diffDays === 1) return 'yesterday'
  if (diffDays < 30) return `${diffDays}d ago`
  if (diffDays < 365) return `${Math.floor(diffDays / 30)}mo ago`
  return `${Math.floor(diffDays / 365)}y ago`
}
