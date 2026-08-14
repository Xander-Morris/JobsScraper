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
