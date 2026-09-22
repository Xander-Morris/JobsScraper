import type { JobSearchParams } from './jobs'

// One place to spell the React Query keys, so an invalidate and the query it
// means to hit can't drift apart.
export const queryKeys = {
  jobs: ['jobs'] as const,
  // Keyed on auth state, not the token: it rotates every reload. Logout clears ['jobs'].
  jobSearch: (params: JobSearchParams, isAuthenticated: boolean) => ['jobs', params, isAuthenticated] as const,
  job: (id: number, isAuthenticated: boolean) => ['jobs', id, isAuthenticated] as const,
  tailoredResume: (jobId: number) => ['jobs', jobId, 'tailored-resume'] as const,
  profile: ['profile'] as const,
  resumeExtraction: (resumeId: number) => ['profile', 'resume', resumeId, 'extraction'] as const,
  tags: ['tags'] as const
}
