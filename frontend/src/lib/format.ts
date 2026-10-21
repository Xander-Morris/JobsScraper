import type { ApplyResumeExtractionResult } from '@/src/api/profile-merge'

// Matches the backend's MinSalary; anything lower would render as "$0k".
const MIN_SALARY = 1000

export function formatSalary(rawMin: number | null, rawMax: number | null): string | null {
  const min = rawMin != null && rawMin >= MIN_SALARY ? rawMin : null
  const max = rawMax != null && rawMax >= MIN_SALARY ? rawMax : null
  if (min == null && max == null) return null

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

// match_percent is the share of open jobs a listing out-matches for the caller's resume.
export const STRONG_MATCH = 85

export function matchLabel(percent: number): string {
  if (percent >= STRONG_MATCH) return 'Strong match'
  if (percent >= 65) return 'Good match'
  if (percent >= 40) return 'Fair match'
  return 'Low match'
}

const EARLIEST_POSTED_AT = Date.UTC(2000, 0, 1)
const DAY_MS = 24 * 60 * 60 * 1000

// Returns null for a missing or implausible date (e.g. year 1 from an unset Go time), so callers show nothing.
export function formatRelativeDate(iso: string | undefined): string | null {
  if (!iso) return null

  const date = new Date(iso)
  const diffMs = Date.now() - date.getTime()
  if (Number.isNaN(diffMs) || date.getTime() < EARLIEST_POSTED_AT || diffMs < -DAY_MS) return null

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

export function summarizeApplyResult(result: ApplyResumeExtractionResult): string {
  const parts: string[] = []
  if (result.addedEducation)
    parts.push(`${result.addedEducation} education entr${result.addedEducation === 1 ? 'y' : 'ies'} added`)
  if (result.updatedEducation)
    parts.push(`${result.updatedEducation} education entr${result.updatedEducation === 1 ? 'y' : 'ies'} updated`)
  if (result.addedSkills) parts.push(`${result.addedSkills} skill${result.addedSkills === 1 ? '' : 's'} added`)
  if (result.addedWorkExperience)
    parts.push(
      `${result.addedWorkExperience} work experience entr${result.addedWorkExperience === 1 ? 'y' : 'ies'} added`
    )
  if (result.updatedWorkExperience)
    parts.push(
      `${result.updatedWorkExperience} work experience entr${result.updatedWorkExperience === 1 ? 'y' : 'ies'} updated`
    )
  if (result.updatedBasicInfo) parts.push('basic info updated')

  return parts.length > 0 ? `${parts.join(', ')}.` : 'Nothing new to add, profile already has this information.'
}
