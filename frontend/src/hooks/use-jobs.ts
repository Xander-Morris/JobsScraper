import { keepPreviousData, queryOptions, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  fetchJob,
  fetchJobs,
  fetchTailoredResume,
  generateApplicationContent,
  generateTailoredResume,
  markJobApplied,
  saveTailoredResume,
  unmarkJobApplied,
  type JobSearchParams
} from '@/src/api/jobs'
import { queryKeys } from '@/src/api/query-keys'
import type { TailoredResumeContent } from '@/src/api/schemas'
import { useAuth } from '@/src/stores/auth-store'

// The query options are exported separately so route prefetches can use the
// same key and fetcher the hooks do, without rendering a hook.
export function jobsQueryOptions(params: JobSearchParams, isAuthenticated: boolean) {
  return queryOptions({
    queryKey: queryKeys.jobSearch(params, isAuthenticated),
    queryFn: () => fetchJobs(params)
  })
}

export function jobQueryOptions(id: number, isAuthenticated: boolean) {
  return queryOptions({
    queryKey: queryKeys.job(id, isAuthenticated),
    queryFn: () => fetchJob(id)
  })
}

export function tailoredResumeQueryOptions(jobId: number) {
  return queryOptions({
    queryKey: queryKeys.tailoredResume(jobId),
    queryFn: () => fetchTailoredResume(jobId)
  })
}

export function useJobsQuery(params: JobSearchParams = {}, options: { enabled?: boolean } = {}) {
  const { isAuthenticated } = useAuth()

  return useQuery({
    ...jobsQueryOptions(params, isAuthenticated),
    enabled: options.enabled,
    placeholderData: keepPreviousData
  })
}

export function useJobQuery(id: number) {
  const { isAuthenticated, isInitializing } = useAuth()

  // Waits out auth init so the anonymous and authed keys don't both fetch.
  return useQuery({ ...jobQueryOptions(id, isAuthenticated), enabled: Number.isFinite(id) && !isInitializing })
}

function useJobAppliedMutation(mutationFn: (id: number) => Promise<{ status: string }>) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.jobs })
    }
  })
}

export function useMarkJobAppliedMutation() {
  return useJobAppliedMutation(markJobApplied)
}

export function useUnmarkJobAppliedMutation() {
  return useJobAppliedMutation(unmarkJobApplied)
}

export function useGenerateApplicationContentMutation() {
  return useMutation({ mutationFn: (jobId: number) => generateApplicationContent(jobId) })
}

export function useTailoredResumeQuery(jobId: number) {
  const { isAuthenticated } = useAuth()

  return useQuery({ ...tailoredResumeQueryOptions(jobId), enabled: isAuthenticated && Number.isFinite(jobId) })
}

export function useGenerateTailoredResumeMutation() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (jobId: number) => generateTailoredResume(jobId),
    onSuccess: (data, jobId) => queryClient.setQueryData(queryKeys.tailoredResume(jobId), data)
  })
}

export function useSaveTailoredResumeMutation(jobId: number) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (content: TailoredResumeContent) => saveTailoredResume(jobId, content),
    onSuccess: (data) => queryClient.setQueryData(queryKeys.tailoredResume(jobId), data)
  })
}
