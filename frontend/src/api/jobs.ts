import { ApiError, apiFetch } from './client'
import {
  generatedContentSchema,
  jobSchema,
  jobSearchResponseSchema,
  statusResponseSchema,
  tailoredResumeSchema,
  type GeneratedContent,
  type Job,
  type JobSearchResponse,
  type TailoredResume,
  type TailoredResumeContent
} from './schemas'

export interface JobSearchParams {
  q?: string
  workplaceType?: 'remote' | 'hybrid' | 'in_person'
  jobType?: 'intern' | 'part_time' | 'full_time'
  minSalary?: number
  maxSalary?: number
  datePosted?: '24h' | '3d' | 'week' | 'month'
  tags?: string[]
  sort?: 'relevance' | 'date'
  limit?: number
  offset?: number
}

function buildJobSearchQuery(params: JobSearchParams): string {
  const search = new URLSearchParams()

  if (params.q) search.set('q', params.q)
  if (params.workplaceType) search.set('workplace_type', params.workplaceType)
  if (params.jobType) search.set('job_type', params.jobType)
  if (params.minSalary !== undefined) search.set('min_salary', String(params.minSalary))
  if (params.maxSalary !== undefined) search.set('max_salary', String(params.maxSalary))
  if (params.datePosted) search.set('date_posted', params.datePosted)
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

export function markJobApplied(id: number) {
  return apiFetch(`/api/jobs/${id}/apply`, statusResponseSchema, { method: 'POST' })
}

export function unmarkJobApplied(id: number) {
  return apiFetch(`/api/jobs/${id}/apply`, statusResponseSchema, { method: 'DELETE' })
}

export function generateApplicationContent(jobId: number): Promise<GeneratedContent> {
  return apiFetch(`/api/jobs/${jobId}/generate`, generatedContentSchema, { method: 'POST' })
}

// A job with no tailored resume yet answers 404; that's an empty state, not an error.
export async function fetchTailoredResume(jobId: number): Promise<TailoredResume | null> {
  try {
    return await apiFetch(`/api/jobs/${jobId}/tailored-resume`, tailoredResumeSchema)
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) return null
    throw error
  }
}

export function generateTailoredResume(jobId: number): Promise<TailoredResume> {
  return apiFetch(`/api/jobs/${jobId}/tailored-resume`, tailoredResumeSchema, { method: 'POST' })
}

export function saveTailoredResume(jobId: number, content: TailoredResumeContent): Promise<TailoredResume> {
  return apiFetch(`/api/jobs/${jobId}/tailored-resume`, tailoredResumeSchema, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(content)
  })
}
