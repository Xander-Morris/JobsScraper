import { queryOptions, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ApiError } from '@/src/api/client'
import {
  activateResume,
  addEducation,
  addSkill,
  addWorkExperience,
  addWorkExperienceBullet,
  confirmPasswordReset,
  createProfile,
  deleteEducation,
  deleteResume,
  deleteSkill,
  deleteWorkExperience,
  deleteWorkExperienceBullet,
  downloadResume,
  fetchProfile,
  fetchResumeExtraction,
  loginProfile,
  requestPasswordReset,
  triggerResumeExtraction,
  updateEducation,
  updateProfile,
  updateResume,
  updateWorkExperienceBullet,
  uploadResume,
  type AddEducationRequest,
  type AddSkillRequest,
  type AddWorkExperienceBulletRequest,
  type AddWorkExperienceRequest,
  type ProfileCredentials,
  type UpdateProfileRequest
} from '@/src/api/profile'
import { applyResumeExtractionToProfile } from '@/src/api/profile-merge'
import { queryKeys } from '@/src/api/query-keys'
import type { Profile, ResumeExtraction } from '@/src/api/schemas'
import { useAuth } from '@/src/stores/auth-store'

export function useCreateProfileMutation() {
  return useMutation({
    mutationFn: ({ email, password }: ProfileCredentials) => createProfile(email, password)
  })
}

export function useLoginMutation() {
  return useMutation({
    mutationFn: ({ email, password }: ProfileCredentials) => loginProfile(email, password)
  })
}

export function useRequestPasswordResetMutation() {
  return useMutation({ mutationFn: (email: string) => requestPasswordReset(email) })
}

export function useConfirmPasswordResetMutation() {
  return useMutation({
    mutationFn: ({ token, password }: { token: string; password: string }) => confirmPasswordReset(token, password)
  })
}

export const profileQueryOptions = queryOptions({
  queryKey: queryKeys.profile,
  queryFn: fetchProfile,
  retry: (failureCount, error) =>
    !(error instanceof ApiError && (error.status === 401 || error.status === 404)) && failureCount < 2
})

export function useProfileQuery() {
  const { isAuthenticated } = useAuth()

  return useQuery({ ...profileQueryOptions, enabled: isAuthenticated })
}

// useProfileMutation refetches the profile on success, since every one of these
// edits a slice of what the profile query returns.
export function useProfileMutation<TArgs, TResult = unknown>(mutationFn: (args: TArgs) => Promise<TResult>) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.profile })
    }
  })
}

export function useUpdateProfileMutation() {
  return useProfileMutation((req: UpdateProfileRequest) => updateProfile(req))
}

export function useAddEducationMutation() {
  return useProfileMutation((req: AddEducationRequest) => addEducation(req))
}

export function useUpdateEducationMutation() {
  return useProfileMutation(({ id, req }: { id: number; req: AddEducationRequest }) => updateEducation(id, req))
}

export function useDeleteEducationMutation() {
  return useProfileMutation((id: number) => deleteEducation(id))
}

export function useAddSkillMutation() {
  return useProfileMutation((req: AddSkillRequest) => addSkill(req))
}

export function useDeleteSkillMutation() {
  return useProfileMutation((id: number) => deleteSkill(id))
}

export function useAddWorkExperienceMutation() {
  return useProfileMutation((req: AddWorkExperienceRequest) => addWorkExperience(req))
}

export function useDeleteWorkExperienceMutation() {
  return useProfileMutation((id: number) => deleteWorkExperience(id))
}

export function useAddWorkExperienceBulletMutation() {
  return useProfileMutation(
    ({ workExperienceId, req }: { workExperienceId: number; req: AddWorkExperienceBulletRequest }) =>
      addWorkExperienceBullet(workExperienceId, req)
  )
}

export function useUpdateWorkExperienceBulletMutation() {
  return useProfileMutation(
    ({ workExperienceId, id, bullet }: { workExperienceId: number; id: number; bullet: string }) =>
      updateWorkExperienceBullet(workExperienceId, id, bullet)
  )
}

export function useDeleteWorkExperienceBulletMutation() {
  return useProfileMutation(({ workExperienceId, id }: { workExperienceId: number; id: number }) =>
    deleteWorkExperienceBullet(workExperienceId, id)
  )
}

export function useUploadResumeMutation() {
  return useProfileMutation((file: File) => uploadResume(file))
}

export function useUpdateResumeMutation() {
  return useProfileMutation(({ id, update }: { id: number; update: File | string }) => updateResume(id, update))
}

export function useActivateResumeMutation() {
  return useProfileMutation((id: number) => activateResume(id))
}

export function useDeleteResumeMutation() {
  return useProfileMutation((id: number) => deleteResume(id))
}

export function useDownloadResumeMutation() {
  return useMutation({ mutationFn: (id: number) => downloadResume(id) })
}

export function useApplyResumeExtractionMutation() {
  return useProfileMutation(({ profile, extraction }: { profile: Profile; extraction: ResumeExtraction }) =>
    applyResumeExtractionToProfile(profile, extraction)
  )
}

export function useResumeExtractionQuery(resumeId: number, options: { enabled: boolean }) {
  const { isAuthenticated } = useAuth()

  return useQuery({
    queryKey: queryKeys.resumeExtraction(resumeId),
    queryFn: () => fetchResumeExtraction(resumeId),
    enabled: options.enabled && isAuthenticated,
    retry: (failureCount, error) => error instanceof ApiError && error.status === 404 && failureCount < 30,
    retryDelay: 2000,
    refetchInterval: (query) => (query.state.data?.status === 'pending' ? 2000 : false)
  })
}

export function useTriggerResumeExtractionMutation() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (resumeId: number) => triggerResumeExtraction(resumeId),
    onSuccess: (_data, resumeId) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.resumeExtraction(resumeId) })
    }
  })
}
