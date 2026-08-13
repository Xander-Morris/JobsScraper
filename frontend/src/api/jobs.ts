import { useQuery } from '@tanstack/react-query'
import { apiFetch } from './client'
import { jobSchema, jobSearchResponseSchema, type Job, type JobSearchResponse } from './schemas'

export interface JobSearchParams {
  q?: string
  workplaceType?: 'remote' | 'hybrid' | 'in_person'
  minSalary?: number
  maxSalary?: number
  tags?: string[]
  sort?: 'relevance' | 'date'
  limit?: number
  offset?: number
}

function buildJobSearchQuery(params: JobSearchParams): string {
  const search = new URLSearchParams()

  if (params.q) search.set('q', params.q)
  if (params.workplaceType) search.set('workplace_type', params.workplaceType)
  if (params.minSalary !== undefined) search.set('min_salary', String(params.minSalary))
  if (params.maxSalary !== undefined) search.set('max_salary', String(params.maxSalary))
  if (params.tags?.length) search.set('tags', params.tags.join(','))
  if (params.sort) search.set('sort', params.sort)
  if (params.limit !== undefined) search.set('limit', String(params.limit))
  if (params.offset !== undefined) search.set('offset', String(params.offset))

  return search.toString()
}

export function fetchJobs(params: JobSearchParams = {}): Promise<JobSearchResponse> {
  const qs = buildJobSearchQuery(params)
  return apiFetch(`/api/jobs${qs ? `?${qs}` : ''}`, jobSearchResponseSchema)
}

export function fetchJob(id: number): Promise<Job> {
  return apiFetch(`/api/jobs/${id}`, jobSchema)
}

export function useJobsQuery(params: JobSearchParams = {}) {
  return useQuery({
    queryKey: ['jobs', params],
    queryFn: () => fetchJobs(params),
  })
}

export function useJobQuery(id: number) {
  return useQuery({
    queryKey: ['jobs', id],
    queryFn: () => fetchJob(id),
    enabled: Number.isFinite(id),
  })
}
