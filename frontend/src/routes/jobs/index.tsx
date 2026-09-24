import { useQueryClient } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { useEffect, useMemo } from 'react'
import { jobsQueryOptions, useJobsQuery } from '@/src/hooks/use-jobs'
import { type JobSearchParams } from '@/src/api/jobs'
import { AuthForms } from '../../components/auth/AuthForms'
import { JobCard } from '../../components/jobs/job-card'
import Pagination from '../../components/pagination'
import { SearchFilters } from '../../components/search-filters'
import { Skeleton } from '../../components/ui/skeleton'
import { jobSearchSchema, type JobSearchState } from '../../lib/job-search'
import { cn } from '../../lib/utils'
import { useAuth } from '@/src/stores/auth-store'

const PAGE_SIZE = 20

function toJobSearchParams(search: JobSearchState): JobSearchParams {
  return {
    q: search.q,
    workplaceType: search.workplaceType,
    jobType: search.jobType,
    minSalary: search.minSalary,
    maxSalary: search.maxSalary,
    datePosted: search.datePosted,
    tags: search.tags,
    sort: search.sort,
    limit: PAGE_SIZE,
    offset: ((search.page ?? 1) - 1) * PAGE_SIZE
  }
}

export const Route = createFileRoute('/jobs/')({
  validateSearch: jobSearchSchema,
  loaderDeps: ({ search }) => search,
  loader: ({ context: { auth, queryClient }, deps }) => {
    if (auth.isAuthenticated) void queryClient.prefetchQuery(jobsQueryOptions(toJobSearchParams(deps), true))
  },
  component: JobsPage
})

function JobsPage() {
  const { isAuthenticated, isInitializing } = useAuth()
  const search = Route.useSearch()
  const navigate = Route.useNavigate()
  const queryClient = useQueryClient()

  const params = useMemo(() => toJobSearchParams(search), [search])

  const { data, isLoading, isError, error, isPlaceholderData } = useJobsQuery(params, { enabled: isAuthenticated })

  const hasNextPage = !!data && !isPlaceholderData && (params.offset ?? 0) + PAGE_SIZE < data.total

  // Next page is usually the next click.
  useEffect(() => {
    if (!hasNextPage) return
    void queryClient.prefetchQuery(
      jobsQueryOptions({ ...params, offset: (params.offset ?? 0) + PAGE_SIZE }, isAuthenticated)
    )
  }, [hasNextPage, isAuthenticated, params, queryClient])

  function updateFilters(next: JobSearchState) {
    navigate({ search: { ...next, page: 1 } })
  }

  const bestMatchScore = Math.max(0, ...(data?.jobs.map((job) => job.match_score ?? 0) ?? []))

  if (isInitializing) {
    return (
      <div className="mx-auto max-w-3xl px-6 py-8">
        <h1 className="text-2xl font-semibold text-heading">Jobs</h1>
        <Skeleton className="mt-6 h-24 w-full rounded-xl" />
      </div>
    )
  }

  if (!isAuthenticated) {
    return (
      <div className="mx-auto max-w-3xl px-6 py-8">
        <h1 className="text-2xl font-semibold text-heading">Jobs</h1>
        <AuthForms prompt="Log in to view job listings." />
      </div>
    )
  }

  return (
    <div className="mx-auto max-w-3xl px-6 py-8">
      <h1 className="text-2xl font-semibold text-heading">Jobs</h1>

      <SearchFilters search={search} onChange={updateFilters} />

      {isLoading && (
        <ul className="mt-6 space-y-2" aria-label="Loading jobs">
          {Array.from({ length: 5 }).map((_, i) => (
            <li key={i}>
              <Skeleton className="h-24 w-full rounded-xl" />
            </li>
          ))}
        </ul>
      )}
      {isError && (
        <p role="alert" className="mt-6 text-sm text-destructive">
          {error.message}
        </p>
      )}

      {data && data.jobs.length === 0 && (
        <p className="mt-6 text-sm text-muted-foreground">No jobs match those filters.</p>
      )}

      {data && data.jobs.length > 0 && (
        <>
          <p className="mt-6 text-xs text-muted-foreground">
            {data.total} result{data.total === 1 ? '' : 's'}
          </p>
          <ul
            className={cn('mt-2 space-y-2 transition-opacity', isPlaceholderData && 'opacity-60')}
            aria-busy={isPlaceholderData}
          >
            {data.jobs.map((job) => (
              <JobCard key={job.id} job={job} bestMatchScore={bestMatchScore} />
            ))}
          </ul>

          <Pagination
            page={search.page ?? 1}
            total={data.total}
            pageSize={PAGE_SIZE}
            onPageChange={(page) => navigate({ search: { ...search, page } })}
          />
        </>
      )}
    </div>
  )
}
