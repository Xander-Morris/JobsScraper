import { createFileRoute } from '@tanstack/react-router'
import { useJobsQuery } from '../api/jobs'
import { JobCard } from '../components/JobCard'
import { Pagination } from '../components/Pagination'
import { SearchFilters } from '../components/SearchFilters'
import { Skeleton } from '../components/ui/skeleton'
import { jobSearchSchema, type JobSearchState } from '../lib/jobSearch'

const PAGE_SIZE = 20

export const Route = createFileRoute('/')({
  validateSearch: jobSearchSchema,
  component: HomePage,
})

function HomePage() {
  const search = Route.useSearch()
  const navigate = Route.useNavigate()

  const { data, isLoading, isError, error } = useJobsQuery({
    q: search.q,
    workplaceType: search.workplaceType,
    minSalary: search.minSalary,
    maxSalary: search.maxSalary,
    tags: search.tags,
    sort: search.sort,
    limit: PAGE_SIZE,
    offset: ((search.page ?? 1) - 1) * PAGE_SIZE,
  })

  function updateFilters(next: JobSearchState) {
    navigate({ search: { ...next, page: 1 } })
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
          <ul className="mt-2 space-y-2">
            {data.jobs.map((job) => (
              <JobCard key={job.id} job={job} />
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
