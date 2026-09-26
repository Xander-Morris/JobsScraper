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
      <div className="mx-auto max-w-4xl px-4 py-10 sm:px-6">
        <PageTitle />
        <Skeleton className="mt-8 h-11 w-full rounded-lg" />
      </div>
    )
  }

  if (!isAuthenticated) return <AuthForms prompt="Log in to browse job listings." />

  return (
    <div className="mx-auto max-w-4xl px-4 py-10 sm:px-6">
      <div className="flex items-end justify-between gap-4">
        <PageTitle />
        {data && (
          <p className="pb-1 text-sm text-muted-foreground tabular-nums">
            {data.total.toLocaleString()} result{data.total === 1 ? '' : 's'}
          </p>
        )}
      </div>

      <SearchFilters search={search} onChange={updateFilters} />

      {isLoading && (
        <ul
          className="mt-6 divide-y divide-border overflow-hidden rounded-xl border border-border"
          aria-label="Loading jobs"
        >
          {Array.from({ length: 5 }).map((_, i) => (
            <li key={i} className="space-y-2.5 px-5 py-4">
              <Skeleton className="h-4 w-1/2" />
              <Skeleton className="h-3.5 w-1/3" />
              <Skeleton className="h-5 w-2/3" />
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
        <div className="mt-6 rounded-xl border border-dashed border-input px-6 py-12 text-center">
          <p className="font-medium text-heading">No jobs match those filters</p>
          <p className="mt-1 text-sm text-muted-foreground">Try widening the salary range or removing a tag.</p>
        </div>
      )}

      {data && data.jobs.length > 0 && (
        <>
          <ul
            className={cn(
              'mt-6 divide-y divide-border overflow-hidden rounded-xl border border-border bg-card/50 transition-opacity',
              isPlaceholderData && 'opacity-60'
            )}
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

function PageTitle() {
  return <h1 className="text-3xl font-medium tracking-[-0.03em]">Jobs</h1>
}
