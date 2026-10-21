import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  formatRelativeDate,
  formatSalary,
  formatWorkplaceType,
  matchLabel,
  STRONG_MATCH,
  summarizeApplyResult
} from '@/src/lib/format'

describe('formatSalary', () => {
  it.each([
    [120000, 150000, '$120k – $150k'],
    [120000, null, '$120k+'],
    [null, 90000, 'up to $90k'],
    [null, null, null],
    [500, 900, null],
    [999, 80000, 'up to $80k'],
    [85500, null, '$86k+'],
    [1000, 1000, '$1k – $1k']
  ])('(%s, %s) -> %s', (min, max, want) => {
    expect(formatSalary(min, max)).toBe(want)
  })
})

describe('formatWorkplaceType', () => {
  it.each([
    ['remote', 'Remote'],
    ['hybrid', 'Hybrid'],
    ['in_person', 'In person'],
    ['unknown', 'Unknown'],
    ['spaceship', 'Unknown']
  ])('%s -> %s', (type, want) => {
    expect(formatWorkplaceType(type)).toBe(want)
  })
})

describe('matchLabel', () => {
  it.each([
    [100, 'Strong match'],
    [STRONG_MATCH, 'Strong match'],
    [STRONG_MATCH - 1, 'Good match'],
    [65, 'Good match'],
    [64, 'Fair match'],
    [40, 'Fair match'],
    [39, 'Low match'],
    [0, 'Low match']
  ])('%i -> %s', (percent, want) => {
    expect(matchLabel(percent)).toBe(want)
  })
})

describe('formatRelativeDate', () => {
  const now = new Date('2026-06-15T12:00:00Z')

  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(now)
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  const ago = (ms: number) => new Date(now.getTime() - ms).toISOString()
  const MIN = 60_000
  const HOUR = 60 * MIN
  const DAY = 24 * HOUR

  it.each([
    [ago(10_000), 'just now'],
    [ago(MIN), '1 minute ago'],
    [ago(5 * MIN), '5 minutes ago'],
    [ago(HOUR), '1 hour ago'],
    [ago(3 * HOUR), '3 hours ago'],
    [ago(HOUR + MIN), '1 hour, 1 minute ago'],
    [ago(2 * HOUR + 30 * MIN), '2 hours, 30 minutes ago'],
    [ago(DAY), 'yesterday'],
    [ago(5 * DAY), '5d ago'],
    [ago(29 * DAY), '29d ago'],
    [ago(60 * DAY), '2mo ago'],
    [ago(400 * DAY), '1y ago'],
    [ago(-HOUR), 'just now']
  ])('%s -> %s', (iso, want) => {
    expect(formatRelativeDate(iso)).toBe(want)
  })

  it.each([undefined, '', 'not a date', '0001-01-01T00:00:00Z', '1999-12-31T00:00:00Z', ago(-2 * DAY)])(
    'returns null for %s',
    (iso) => {
      expect(formatRelativeDate(iso)).toBeNull()
    }
  )
})

describe('summarizeApplyResult', () => {
  const none = {
    updatedBasicInfo: false,
    addedEducation: 0,
    updatedEducation: 0,
    addedSkills: 0,
    addedWorkExperience: 0,
    updatedWorkExperience: 0
  }

  it('says when nothing changed', () => {
    expect(summarizeApplyResult(none)).toBe('Nothing new to add, profile already has this information.')
  })

  it('uses singular counts', () => {
    expect(
      summarizeApplyResult({
        updatedBasicInfo: true,
        addedEducation: 1,
        updatedEducation: 1,
        addedSkills: 1,
        addedWorkExperience: 1,
        updatedWorkExperience: 1
      })
    ).toBe(
      '1 education entry added, 1 education entry updated, 1 skill added, 1 work experience entry added, 1 work experience entry updated, basic info updated.'
    )
  })

  it('uses plural counts', () => {
    expect(
      summarizeApplyResult({
        ...none,
        addedEducation: 2,
        updatedEducation: 3,
        addedSkills: 4,
        addedWorkExperience: 5,
        updatedWorkExperience: 6
      })
    ).toBe(
      '2 education entries added, 3 education entries updated, 4 skills added, 5 work experience entries added, 6 work experience entries updated.'
    )
  })
})
