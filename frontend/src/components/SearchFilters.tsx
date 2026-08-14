import { useEffect, useState } from 'react'
import { useTagsQuery } from '../api/tags'
import type { JobSearchState } from '../lib/jobSearch'

const MAX_TAG_MATCHES = 40

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

  return (
    <div className="mt-6 space-y-3 text-left">
      <input
        type="search"
        value={q}
        onChange={(e) => setQ(e.target.value)}
        placeholder="Search titles, companies, descriptions…"
        className="w-full rounded border border-border bg-transparent px-3 py-2 text-sm text-heading placeholder:text-muted focus:border-accent-border focus:outline-none"
      />

      <div className="flex flex-wrap items-center gap-3 text-sm">
        <select
          value={search.workplaceType ?? ''}
          onChange={(e) =>
            onChange({
              ...search,
              workplaceType: (e.target.value || undefined) as JobSearchState['workplaceType'],
            })
          }
          className="rounded border border-border bg-surface px-2 py-1.5 text-heading"
        >
          <option value="">Any workplace</option>
          <option value="remote">Remote</option>
          <option value="hybrid">Hybrid</option>
          <option value="in_person">In person</option>
        </select>

        <input
          type="number"
          inputMode="numeric"
          placeholder="Min salary"
          value={search.minSalary ?? ''}
          onChange={(e) =>
            onChange({ ...search, minSalary: e.target.value ? Number(e.target.value) : undefined })
          }
          className="w-28 rounded border border-border bg-transparent px-2 py-1.5 text-heading placeholder:text-muted"
        />
        <input
          type="number"
          inputMode="numeric"
          placeholder="Max salary"
          value={search.maxSalary ?? ''}
          onChange={(e) =>
            onChange({ ...search, maxSalary: e.target.value ? Number(e.target.value) : undefined })
          }
          className="w-28 rounded border border-border bg-transparent px-2 py-1.5 text-heading placeholder:text-muted"
        />

        <select
          value={search.sort ?? 'relevance'}
          onChange={(e) => onChange({ ...search, sort: e.target.value as JobSearchState['sort'] })}
          className="rounded border border-border bg-surface px-2 py-1.5 text-heading"
        >
          <option value="relevance">Sort: relevance</option>
          <option value="date">Sort: newest</option>
        </select>
      </div>

      {tags && tags.length > 0 && (
        <div>
          {selectedTags.size > 0 && (
            <div className="mb-2 flex flex-wrap gap-1.5 font-mono text-xs">
              {Array.from(selectedTags).map((tag) => (
                <button
                  key={tag}
                  type="button"
                  onClick={() => toggleTag(tag)}
                  className="rounded bg-accent px-2 py-1 text-white"
                >
                  {tag} ×
                </button>
              ))}
            </div>
          )}

          <div className="relative">
            <input
              type="text"
              value={tagFilter}
              onChange={(e) => setTagFilter(e.target.value)}
              onFocus={() => setTagMenuOpen(true)}
              onBlur={() => setTimeout(() => setTagMenuOpen(false), 150)}
              placeholder={`Filter ${tags.length} tags…`}
              className="w-full rounded border border-border bg-transparent px-3 py-1.5 text-sm text-heading placeholder:text-muted focus:border-accent-border focus:outline-none"
            />

            {tagMenuOpen && (
              <div className="absolute z-10 mt-1 max-h-48 w-full overflow-y-auto rounded border border-border bg-surface p-2 shadow-lg">
                {visibleMatches.length === 0 ? (
                  <p className="px-1 py-1 text-xs text-muted">No matching tags.</p>
                ) : (
                  <div className="flex flex-wrap gap-1.5 font-mono text-xs">
                    {visibleMatches.map((tag) => (
                      <button
                        key={tag}
                        type="button"
                        onMouseDown={(e) => e.preventDefault()}
                        onClick={() => toggleTag(tag)}
                        className={
                          selectedTags.has(tag)
                            ? 'rounded bg-accent px-2 py-1 text-white'
                            : 'rounded bg-code-bg px-2 py-1 text-muted hover:text-heading'
                        }
                      >
                        {tag}
                      </button>
                    ))}
                  </div>
                )}
                {matches.length > visibleMatches.length && (
                  <p className="mt-2 px-1 text-xs text-muted">
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
