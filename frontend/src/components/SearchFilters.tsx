import { useEffect, useId, useState } from 'react'
import { ChevronDownIcon, XIcon } from 'lucide-react'
import { useTagsQuery } from '../api/tags'
import type { JobSearchState } from '../lib/jobSearch'
import { badgeVariants } from './ui/badge'
import { buttonVariants } from './ui/button'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from './ui/dropdown-menu'
import { Input } from './ui/input'
import { Label } from './ui/label'
import { Slider } from './ui/slider'
import { cn } from '../lib/utils'

const MAX_TAG_MATCHES = 40

const SALARY_MIN = 20000
const SALARY_MAX = 500000
const SALARY_STEP = 5000

const WORKPLACE_TYPE_OPTIONS: { value: JobSearchState['workplaceType'] | ''; label: string }[] = [
  { value: '', label: 'Any workplace' },
  { value: 'remote', label: 'Remote' },
]

const SORT_OPTIONS: { value: NonNullable<JobSearchState['sort']>; label: string }[] = [
  { value: 'relevance', label: 'Sort: relevance' },
  { value: 'date', label: 'Sort: newest' },
]

const DATE_POSTED_OPTIONS: { value: JobSearchState['datePosted'] | ''; label: string }[] = [
  { value: '', label: 'Any time' },
  { value: '24h', label: 'Past 24 hours' },
  { value: '3d', label: 'Past 3 days' },
  { value: 'week', label: 'Past week' },
  { value: 'month', label: 'Past month' },
]

function formatSalaryThousands(value: number): string {
  return value >= 1000 ? `$${Math.round(value / 1000)}k` : `$${value}`
}

function WorkplaceTypeFilter({
  value,
  onChange,
}: {
  value: JobSearchState['workplaceType']
  onChange: (value: JobSearchState['workplaceType']) => void
}) {
  const label = WORKPLACE_TYPE_OPTIONS.find((opt) => opt.value === (value ?? ''))?.label ?? 'Any workplace'

  return (
    <DropdownMenu>
      <DropdownMenuTrigger className={cn(buttonVariants({ variant: 'outline' }), 'w-40 justify-between font-normal')}>
        {label}
        <ChevronDownIcon className="opacity-50" />
      </DropdownMenuTrigger>
      <DropdownMenuContent>
        {WORKPLACE_TYPE_OPTIONS.map((opt) => (
          <DropdownMenuItem
            key={opt.value || 'any'}
            onClick={() => onChange((opt.value || undefined) as JobSearchState['workplaceType'])}
          >
            {opt.label}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function DatePostedFilter({
  value,
  onChange,
}: {
  value: JobSearchState['datePosted']
  onChange: (value: JobSearchState['datePosted']) => void
}) {
  const label = DATE_POSTED_OPTIONS.find((opt) => opt.value === (value ?? ''))?.label ?? 'Any time'

  return (
    <DropdownMenu>
      <DropdownMenuTrigger className={cn(buttonVariants({ variant: 'outline' }), 'w-40 justify-between font-normal')}>
        {label}
        <ChevronDownIcon className="opacity-50" />
      </DropdownMenuTrigger>
      <DropdownMenuContent>
        {DATE_POSTED_OPTIONS.map((opt) => (
          <DropdownMenuItem key={opt.value || 'any'} onClick={() => onChange(opt.value || undefined)}>
            {opt.label}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function SortFilter({
  value,
  onChange,
}: {
  value: JobSearchState['sort']
  onChange: (value: NonNullable<JobSearchState['sort']>) => void
}) {
  const label = SORT_OPTIONS.find((opt) => opt.value === (value ?? 'relevance'))?.label

  return (
    <DropdownMenu>
      <DropdownMenuTrigger className={cn(buttonVariants({ variant: 'outline' }), 'w-40 justify-between font-normal')}>
        {label}
        <ChevronDownIcon className="opacity-50" />
      </DropdownMenuTrigger>
      <DropdownMenuContent>
        {SORT_OPTIONS.map((opt) => (
          <DropdownMenuItem key={opt.value} onClick={() => onChange(opt.value)}>
            {opt.label}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function SalaryRangeFilter({
  minSalary,
  maxSalary,
  onCommit,
}: {
  minSalary: number | undefined
  maxSalary: number | undefined
  onCommit: (minSalary: number | undefined, maxSalary: number | undefined) => void
}) {
  const [salaryRange, setSalaryRange] = useState<number[]>([minSalary ?? SALARY_MIN, maxSalary ?? SALARY_MAX])

  useEffect(() => {
    setSalaryRange([minSalary ?? SALARY_MIN, maxSalary ?? SALARY_MAX])
  }, [minSalary, maxSalary])

  function commit(next: number[]) {
    onCommit(next[0] > SALARY_MIN ? next[0] : undefined, next[1] < SALARY_MAX ? next[1] : undefined)
  }

  return (
    <div className="w-72 space-y-1.5">
      <div className="flex items-center justify-between text-xs text-muted-foreground">
        <span>Salary</span>
        <span>
          {formatSalaryThousands(salaryRange[0])} – {formatSalaryThousands(salaryRange[1])}
          {salaryRange[1] >= SALARY_MAX ? '+' : ''}
        </span>
      </div>
      <Slider
        aria-label="Salary range"
        min={SALARY_MIN}
        max={SALARY_MAX}
        step={SALARY_STEP}
        value={salaryRange}
        onValueChange={(next: number[]) => setSalaryRange(next)}
        onValueCommitted={(next: number[]) => commit(next)}
      />
    </div>
  )
}

function TagFilter({
  tags,
  selectedTags,
  onToggle,
}: {
  tags: string[]
  selectedTags: Set<string>
  onToggle: (tag: string) => void
}) {
  const [tagFilter, setTagFilter] = useState('')
  const [tagMenuOpen, setTagMenuOpen] = useState(false)
  const tagListId = useId()

  const matches = tagFilter.trim()
    ? tags.filter((tag) => tag.toLowerCase().includes(tagFilter.trim().toLowerCase()))
    : tags
  const visibleMatches = matches.slice(0, MAX_TAG_MATCHES)

  return (
    <div>
      {selectedTags.size > 0 && (
        <ul className="mb-2 flex flex-wrap gap-1.5">
          {Array.from(selectedTags).map((tag) => (
            <li key={tag}>
              <button
                type="button"
                onClick={() => onToggle(tag)}
                aria-label={`Remove ${tag} filter`}
                className={cn(badgeVariants({ variant: 'default' }), 'gap-1')}
              >
                {tag}
                <XIcon aria-hidden="true" />
              </button>
            </li>
          ))}
        </ul>
      )}

      <div className="relative">
        <Label htmlFor={tagListId} className="sr-only">
          Filter tags
        </Label>
        <Input
          id={tagListId}
          type="text"
          role="combobox"
          aria-expanded={tagMenuOpen}
          aria-controls={`${tagListId}-listbox`}
          autoComplete="off"
          value={tagFilter}
          onChange={(e) => setTagFilter(e.target.value)}
          onFocus={() => setTagMenuOpen(true)}
          onBlur={() => setTimeout(() => setTagMenuOpen(false), 150)}
          onKeyDown={(e) => {
            if (e.key === 'Escape') setTagMenuOpen(false)
          }}
          placeholder={`Filter ${tags.length} tags…`}
        />

        {tagMenuOpen && (
          <div
            id={`${tagListId}-listbox`}
            role="listbox"
            aria-label="Matching tags"
            aria-multiselectable="true"
            className="absolute z-10 mt-1 max-h-48 w-full overflow-y-auto rounded-lg border border-border bg-popover p-2 text-popover-foreground shadow-md ring-1 ring-foreground/10"
          >
            {visibleMatches.length === 0 ? (
              <p className="px-1 py-1 text-xs text-muted-foreground">No matching tags.</p>
            ) : (
              <div className="flex flex-wrap gap-1.5">
                {visibleMatches.map((tag) => (
                  <button
                    key={tag}
                    type="button"
                    role="option"
                    aria-selected={selectedTags.has(tag)}
                    onMouseDown={(e) => e.preventDefault()}
                    onClick={() => onToggle(tag)}
                    className={cn(
                      badgeVariants({ variant: selectedTags.has(tag) ? 'default' : 'secondary' }),
                      !selectedTags.has(tag) && 'text-muted-foreground hover:text-heading',
                    )}
                  >
                    {tag}
                  </button>
                ))}
              </div>
            )}
            {matches.length > visibleMatches.length && (
              <p className="mt-2 px-1 text-xs text-muted-foreground">
                +{matches.length - visibleMatches.length} more — keep typing to narrow down
              </p>
            )}
          </div>
        )}
      </div>
    </div>
  )
}

export function SearchFilters({
  search,
  onChange,
}: {
  search: JobSearchState
  onChange: (next: JobSearchState) => void
}) {
  const { data: tags } = useTagsQuery()
  const [q, setQ] = useState(search.q ?? '')

  useEffect(() => {
    setQ(search.q ?? '')
  }, [search.q])

  useEffect(() => {
    const handle = setTimeout(() => {
      if (q !== (search.q ?? '')) onChange({ ...search, q: q || undefined })
    }, 300)
    return () => clearTimeout(handle)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [q])

  const selectedTags = new Set(search.tags ?? [])

  function toggleTag(tag: string) {
    const next = new Set(selectedTags)
    if (next.has(tag)) next.delete(tag)
    else next.add(tag)
    onChange({ ...search, tags: next.size ? Array.from(next) : undefined })
  }

  return (
    <div className="mt-6 space-y-3 text-left">
      <Input
        type="search"
        value={q}
        onChange={(e) => setQ(e.target.value)}
        placeholder="Search titles, companies, descriptions…"
        aria-label="Search jobs"
      />

      <div className="flex flex-wrap items-center gap-3 text-sm">
        <WorkplaceTypeFilter
          value={search.workplaceType}
          onChange={(workplaceType) => onChange({ ...search, workplaceType })}
        />
        <DatePostedFilter value={search.datePosted} onChange={(datePosted) => onChange({ ...search, datePosted })} />
        <SortFilter value={search.sort} onChange={(sort) => onChange({ ...search, sort })} />
      </div>

      <SalaryRangeFilter
        minSalary={search.minSalary}
        maxSalary={search.maxSalary}
        onCommit={(minSalary, maxSalary) => onChange({ ...search, minSalary, maxSalary })}
      />

      {tags && tags.length > 0 && <TagFilter tags={tags} selectedTags={selectedTags} onToggle={toggleTag} />}
    </div>
  )
}
