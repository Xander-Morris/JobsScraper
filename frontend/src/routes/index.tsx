import { createFileRoute } from '@tanstack/react-router'
import { useJobsQuery } from '../api/jobs'

export const Route = createFileRoute('/')({
  component: HomePage,
})

function HomePage() {
  const { data, isLoading, isError, error } = useJobsQuery()

  return (
    <div className="mx-auto max-w-3xl px-4 py-8">
      <h1 className="text-2xl font-semibold">Crawler &amp; Indexer</h1>

      {isLoading && <p className="mt-4 text-sm text-gray-500">Loading jobs…</p>}
      {isError && <p className="mt-4 text-sm text-red-500">{error.message}</p>}

      {data && (
        <ul className="mt-4 space-y-2 text-left">
          {data.jobs.map((job) => (
            <li key={job.id} className="rounded border border-gray-200 p-3 dark:border-gray-700">
              <p className="font-medium">{job.title}</p>
              <p className="text-sm text-gray-500">{job.company}</p>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
