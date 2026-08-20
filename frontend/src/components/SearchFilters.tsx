import { useEffect, useId, useState } from 'react'
import { ChevronDownIcon, XIcon } from 'lucide-react'
import { useTagsQuery } from '../api/tags'
import type { JobSearchState } from '../lib/jobSearch'
import { badgeVariants } from './ui/badge'
import { buttonVariants } from './ui/button'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from './ui/dropdown-menu'
import { Input } from './ui/input'
import { Label } from './ui/label'
import { cn } from '../lib/utils'

const MAX_TAG_MATCHES = 40

const WORKPLACE_TYPE_OPTIONS: { value: JobSearchState['workplaceType'] | ''; label: string }[] = [
  { value: '', label: 'Any workplace' },
  { value: 'remote', label: 'Remote' },
  { value: 'hybrid', label: 'Hybrid' },
  { value: 'in_person', label: 'In person' },
]

const SORT_OPTIONS: { value: NonNullable<JobSearchState['sort']>; label: string }[] = [
  { value: 'relevance', label: 'Sort: relevance' },
  { value: 'date', label: 'Sort: newest' },
]

export function SearchFilters({
  search,
  onChange,
}: {
  search: JobSearchState
  onChange: (next: JobSearchState) => void
}) {
  const { data: tags } = useTagsQuery()
  const [q, setQ] = useState(search.q ?? '')
  const [tagFilter, setTagFilter] = useState('')
  const [tagMenuOpen, setTagMenuOpen] = useState(false)
  const tagListId = useId()

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

  const matches = tagFilter.trim()
    ? (tags ?? []).filter((tag) => tag.toLowerCase().includes(tagFilter.trim().toLowerCase()))
    : (tags ?? [])
  const visibleMatches = matches.slice(0, MAX_TAG_MATCHES)

  const workplaceLabel =
    WORKPLACE_TYPE_OPTIONS.find((opt) => opt.value === (search.workplaceType ?? ''))?.label ?? 'Any workplace'
  const sortLabel = SORT_OPTIONS.find((opt) => opt.value === (search.sort ?? 'relevance'))?.label

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
        <DropdownMenu>
          <DropdownMenuTrigger className={cn(buttonVariants({ variant: 'outline' }), 'w-40 justify-between font-normal')}>
            {workplaceLabel}
            <ChevronDownIcon className="opacity-50" />
          </DropdownMenuTrigger>
          <DropdownMenuContent>
            {WORKPLACE_TYPE_OPTIONS.map((opt) => (
              <DropdownMenuItem
                key={opt.value || 'any'}
                onClick={() =>
                  onChange({ ...search, workplaceType: (opt.value || undefined) as JobSearchState['workplaceType'] })
                }
              >
                {opt.label}
              </DropdownMenuItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>

        <Input
          type="number"
          inputMode="numeric"
          placeholder="Min salary"
          aria-label="Minimum salary"
          value={search.minSalary ?? ''}
          onChange={(e) => onChange({ ...search, minSalary: e.target.value ? Number(e.target.value) : undefined })}
          className="w-28"
        />
        <Input
          type="number"
          inputMode="numeric"
          placeholder="Max salary"
          aria-label="Maximum salary"
          value={search.maxSalary ?? ''}
          onChange={(e) => onChange({ ...search, maxSalary: e.target.value ? Number(e.target.value) : undefined })}
          className="w-28"
        />

        <DropdownMenu>
          <DropdownMenuTrigger className={cn(buttonVariants({ variant: 'outline' }), 'w-40 justify-between font-normal')}>
            {sortLabel}
            <ChevronDownIcon className="opacity-50" />
          </DropdownMenuTrigger>
          <DropdownMenuContent>
            {SORT_OPTIONS.map((opt) => (
              <DropdownMenuItem key={opt.value} onClick={() => onChange({ ...search, sort: opt.value })}>
                {opt.label}
              </DropdownMenuItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>
      </div>

      {tags && tags.length > 0 && (
        <div>
          {selectedTags.size > 0 && (
            <ul className="mb-2 flex flex-wrap gap-1.5">
              {Array.from(selectedTags).map((tag) => (
                <li key={tag}>
                  <button
                    type="button"
                    onClick={() => toggleTag(tag)}
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
                        onClick={() => toggleTag(tag)}
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
      )}
    </div>
  )
}
