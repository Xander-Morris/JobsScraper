import { z } from 'zod'

export const datePostedOptions = ['24h', '3d', 'week', 'month'] as const

export const jobSearchSchema = z.object({
  q: z.string().optional(),
  workplaceType: z.enum(['remote', 'hybrid', 'in_person']).optional(),
  jobType: z.enum(['intern', 'part_time', 'full_time']).optional(),
  minSalary: z.number().optional(),
  maxSalary: z.number().optional(),
  datePosted: z.enum(datePostedOptions).optional(),
  tags: z.array(z.string()).optional(),
  sort: z.enum(['relevance', 'date']).optional(),
  page: z.number().optional()
})

export type JobSearchState = z.infer<typeof jobSearchSchema>

const LAST_SEARCH_KEY = 'jobs-last-search'

// Lets links back to the board keep its filters.
export function rememberJobSearch(search: JobSearchState) {
  try {
    sessionStorage.setItem(LAST_SEARCH_KEY, JSON.stringify(search))
  } catch {
    // Storage blocked; links fall back to an unfiltered board.
  }
}

export function lastJobSearch(): JobSearchState {
  try {
    const parsed = jobSearchSchema.safeParse(JSON.parse(sessionStorage.getItem(LAST_SEARCH_KEY) ?? '{}'))
    return parsed.success ? parsed.data : {}
  } catch {
    return {}
  }
}
