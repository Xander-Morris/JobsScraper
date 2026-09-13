import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { z } from 'zod'
import { ApiError, apiFetch } from './client'
import {
  generatedContentSchema,
  jobSchema,
  jobSearchResponseSchema,
  tailoredResumeSchema,
  type GeneratedContent,
  type Job,
  type JobSearchResponse,
  type TailoredResume,
  type TailoredResumeContent
} from './schemas'

const statusResponseSchema = z.object({ status: z.string() })

export interface JobSearchParams {
  q?: string
  workplaceType?: 'remote' | 'hybrid' | 'in_person'
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
  if (params.minSalary !== undefined) search.set('min_salary', String(params.minSalary))
  if (params.maxSalary !== undefined) search.set('max_salary', String(params.maxSalary))
  if (params.datePosted) search.set('date_posted', params.datePosted)
  if (params.tags?.length) search.set('tags', params.tags.join(','))
  if (params.sort) search.set('sort', params.sort)
  if (params.limit !== undefined) search.set('limit', String(params.limit))
  if (params.offset !== undefined) search.set('offset', String(params.offset))

  return search.toString()
}

export function fetchJobs(params: JobSearchParams = {}, token?: string | null): Promise<JobSearchResponse> {
  const qs = buildJobSearchQuery(params)
  return apiFetch(`/api/jobs${qs ? `?${qs}` : ''}`, jobSearchResponseSchema, {
    headers: token ? { Authorization: `Bearer ${token}` } : undefined
  })
}

export function fetchJob(id: number, token?: string | null): Promise<Job> {
  return apiFetch(`/api/jobs/${id}`, jobSchema, {
    headers: token ? { Authorization: `Bearer ${token}` } : undefined
  })
}

export function useJobsQuery(params: JobSearchParams = {}, options: { enabled?: boolean; token?: string | null } = {}) {
  return useQuery({
    queryKey: ['jobs', params, options.token],
    queryFn: () => fetchJobs(params, options.token),
    enabled: options.enabled
  })
}

export function useJobQuery(id: number, token?: string | null) {
  return useQuery({
    queryKey: ['jobs', id, token],
    queryFn: () => fetchJob(id, token),
    enabled: Number.isFinite(id)
  })
}

export function markJobApplied(token: string, id: number) {
  return apiFetch(`/api/jobs/${id}/apply`, statusResponseSchema, {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}` }
  })
}

export function unmarkJobApplied(token: string, id: number) {
  return apiFetch(`/api/jobs/${id}/apply`, statusResponseSchema, {
    method: 'DELETE',
    headers: { Authorization: `Bearer ${token}` }
  })
}

function useJobAppliedMutation(mutationFn: (id: number) => Promise<{ status: string }>) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['jobs'] })
    }
  })
}

export function useMarkJobAppliedMutation(token: string | null) {
  return useJobAppliedMutation((id: number) => markJobApplied(token!, id))
}

export function useUnmarkJobAppliedMutation(token: string | null) {
  return useJobAppliedMutation((id: number) => unmarkJobApplied(token!, id))
}

export function generateApplicationContent(token: string, jobId: number): Promise<GeneratedContent> {
  return apiFetch(`/api/jobs/${jobId}/generate`, generatedContentSchema, {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}` }
  })
}

export function useGenerateApplicationContentMutation(token: string | null) {
  return useMutation({
    mutationFn: (jobId: number) => generateApplicationContent(token!, jobId)
  })
}

const tailoredResumeKey = (jobId: number) => ['jobs', jobId, 'tailored-resume']

export async function fetchTailoredResume(token: string, jobId: number): Promise<TailoredResume | null> {
  try {
    return await apiFetch(`/api/jobs/${jobId}/tailored-resume`, tailoredResumeSchema, {
      headers: { Authorization: `Bearer ${token}` }
    })
  } catch (error) {
    if (error instanceof ApiError && error.status === 404) return null
    throw error
  }
}

export function generateTailoredResume(token: string, jobId: number): Promise<TailoredResume> {
  return apiFetch(`/api/jobs/${jobId}/tailored-resume`, tailoredResumeSchema, {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}` }
  })
}

export function saveTailoredResume(token: string, jobId: number, content: TailoredResumeContent): Promise<TailoredResume> {
  return apiFetch(`/api/jobs/${jobId}/tailored-resume`, tailoredResumeSchema, {
    method: 'PUT',
    headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
    body: JSON.stringify(content)
  })
}

export function useTailoredResumeQuery(token: string | null, jobId: number) {
  return useQuery({
    queryKey: tailoredResumeKey(jobId),
    queryFn: () => fetchTailoredResume(token!, jobId),
    enabled: !!token && Number.isFinite(jobId)
  })
}

export function useGenerateTailoredResumeMutation(token: string | null) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (jobId: number) => generateTailoredResume(token!, jobId),
    onSuccess: (data, jobId) => queryClient.setQueryData(tailoredResumeKey(jobId), data)
  })
}

export function useSaveTailoredResumeMutation(token: string | null, jobId: number) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (content: TailoredResumeContent) => saveTailoredResume(token!, jobId, content),
    onSuccess: (data) => queryClient.setQueryData(tailoredResumeKey(jobId), data)
  })
}
