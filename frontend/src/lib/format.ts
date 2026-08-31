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

// matchFitLabel turns a raw ts_rank-based match_score into a badge label. Raw
// text-relevance scores aren't calibrated to a meaningful absolute scale, so when
// `best` (the top score among a set of jobs) is available we bucket relative to
// it instead of showing a misleading absolute percentage.
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
  const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24))

  if (diffDays <= 0) return 'today'
  if (diffDays === 1) return 'yesterday'
  if (diffDays < 30) return `${diffDays}d ago`
  if (diffDays < 365) return `${Math.floor(diffDays / 30)}mo ago`
  return `${Math.floor(diffDays / 365)}y ago`
}
