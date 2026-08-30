import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { z } from 'zod'
import { API_BASE_URL, ApiError, apiFetch } from './client'
import {
  authResponseSchema,
  profileSchema,
  resumeExtractionSchema,
  type AuthResponse,
  type Profile,
  type ResumeExtraction,
} from './schemas'

const isoDatePattern = /^\d{4}-\d{2}-\d{2}$/

const idResponseSchema = z.object({ id: z.number() })
const statusResponseSchema = z.object({ status: z.string() })

export interface UpdateProfileRequest {
  name: string
  address: string
  linked_in: string
  github: string
  portfolio: string
}

export interface AddEducationRequest {
  school_name: string
  major: string
  degree: string
  gpa?: number | null
  start_date?: string
  end_date?: string
}

export interface AddSkillRequest {
  skill: string
}

export interface AddWorkExperienceRequest {
  company: string
  job_title: string
  job_type: string
  location?: string
  start_date?: string
  end_date?: string
}

export interface AddWorkExperienceBulletRequest {
  bullet: string
  position?: number
}

export interface ApplyResumeExtractionResult {
  updatedBasicInfo: boolean
  addedEducation: number
  addedSkills: number
  addedWorkExperience: number
}

function authHeaders(token: string): HeadersInit {
  return { Authorization: `Bearer ${token}` }
}

function jsonHeaders(token?: string): HeadersInit {
  return {
    'Content-Type': 'application/json',
    ...(token ? authHeaders(token) : {}),
  }
}

export function createProfile(email: string, password: string): Promise<AuthResponse> {
  return apiFetch('/api/profile/create', authResponseSchema, {
    method: 'POST',
    headers: jsonHeaders(),
    body: JSON.stringify({ email, password }),
  })
}

export function loginProfile(email: string, password: string): Promise<AuthResponse> {
  return apiFetch('/api/profile/login', authResponseSchema, {
    method: 'POST',
    headers: jsonHeaders(),
    body: JSON.stringify({ email, password }),
  })
}

export function fetchProfile(token: string): Promise<Profile> {
  return apiFetch('/api/profile', profileSchema, { headers: authHeaders(token) })
}

export function updateProfile(token: string, req: UpdateProfileRequest) {
  return apiFetch('/api/profile', statusResponseSchema, {
    method: 'PUT',
    headers: jsonHeaders(token),
    body: JSON.stringify(req),
  })
}

export function addEducation(token: string, req: AddEducationRequest) {
  return apiFetch('/api/profile/education', idResponseSchema, {
    method: 'POST',
    headers: jsonHeaders(token),
    body: JSON.stringify(req),
  })
}

export function deleteEducation(token: string, id: number) {
  return apiFetch(`/api/profile/education/${id}`, statusResponseSchema, {
    method: 'DELETE',
    headers: authHeaders(token),
  })
}

export function addSkill(token: string, req: AddSkillRequest) {
  return apiFetch('/api/profile/skills', idResponseSchema, {
    method: 'POST',
    headers: jsonHeaders(token),
    body: JSON.stringify(req),
  })
}

export function deleteSkill(token: string, id: number) {
  return apiFetch(`/api/profile/skills/${id}`, statusResponseSchema, {
    method: 'DELETE',
    headers: authHeaders(token),
  })
}

export function addWorkExperience(token: string, req: AddWorkExperienceRequest) {
  return apiFetch('/api/profile/work-experience', idResponseSchema, {
    method: 'POST',
    headers: jsonHeaders(token),
    body: JSON.stringify(req),
  })
}

export function deleteWorkExperience(token: string, id: number) {
  return apiFetch(`/api/profile/work-experience/${id}`, statusResponseSchema, {
    method: 'DELETE',
    headers: authHeaders(token),
  })
}

export function addWorkExperienceBullet(
  token: string,
  workExperienceId: number,
  req: AddWorkExperienceBulletRequest,
) {
  return apiFetch(`/api/profile/work-experience/${workExperienceId}/bullets`, idResponseSchema, {
    method: 'POST',
    headers: jsonHeaders(token),
    body: JSON.stringify(req),
  })
}

export function deleteWorkExperienceBullet(token: string, workExperienceId: number, id: number) {
  return apiFetch(`/api/profile/work-experience/${workExperienceId}/bullets/${id}`, statusResponseSchema, {
    method: 'DELETE',
    headers: authHeaders(token),
  })
}

export function uploadResume(token: string, file: File) {
  const body = new FormData()
  body.append('resume', file)

  return apiFetch('/api/profile/resumes', idResponseSchema, {
    method: 'POST',
    headers: authHeaders(token),
    body,
  })
}

export function updateResume(token: string, id: number, update: File | string) {
  const body = new FormData()
  if (typeof update === 'string') {
    body.append('file_name', update)
  } else {
    body.append('resume', update)
  }

  return apiFetch(`/api/profile/resumes/${id}`, statusResponseSchema, {
    method: 'PUT',
    headers: authHeaders(token),
    body,
  })
}

export function deleteResume(token: string, id: number) {
  return apiFetch(`/api/profile/resumes/${id}`, statusResponseSchema, {
    method: 'DELETE',
    headers: authHeaders(token),
  })
}

export function activateResume(token: string, id: number) {
  return apiFetch(`/api/profile/resumes/${id}/activate`, statusResponseSchema, {
    method: 'POST',
    headers: authHeaders(token),
  })
}

export function fetchResumeExtraction(token: string, id: number): Promise<ResumeExtraction> {
  return apiFetch(`/api/profile/resumes/${id}/extraction`, resumeExtractionSchema, {
    headers: authHeaders(token),
  })
}

export function triggerResumeExtraction(token: string, id: number) {
  return apiFetch(`/api/profile/resumes/${id}/extraction`, statusResponseSchema, {
    method: 'POST',
    headers: authHeaders(token),
  })
}

export async function downloadResume(token: string, id: number): Promise<Blob> {
  const response = await fetch(`${API_BASE_URL}/api/profile/resumes/${id}/download`, {
    credentials: 'include',
    headers: authHeaders(token),
  })

  if (!response.ok) {
    throw new ApiError(response.statusText || 'Unable to download resume', response.status)
  }

  return response.blob()
}

function asDate(value: string): string | undefined {
  return isoDatePattern.test(value) ? value : undefined
}

export async function applyResumeExtractionToProfile(
  token: string,
  profile: Profile,
  extraction: ResumeExtraction,
): Promise<ApplyResumeExtractionResult> {
  const nextName = profile.name || extraction.full_name
  const nextLinkedIn = profile.linked_in || extraction.linked_in
  const nextGithub = profile.github || extraction.github
  const nextPortfolio = profile.portfolio || extraction.portfolio

  const updatedBasicInfo =
    nextName !== profile.name ||
    nextLinkedIn !== profile.linked_in ||
    nextGithub !== profile.github ||
    nextPortfolio !== profile.portfolio

  if (updatedBasicInfo) {
    await updateProfile(token, {
      name: nextName,
      address: profile.address,
      linked_in: nextLinkedIn,
      github: nextGithub,
      portfolio: nextPortfolio,
    })
  }

  const existingSkills = new Set((profile.skills ?? []).map((s) => s.skill.trim().toLowerCase()))
  let addedSkills = 0
  for (const raw of extraction.skills ?? []) {
    const skill = raw.trim()
    if (!skill || existingSkills.has(skill.toLowerCase())) continue
    existingSkills.add(skill.toLowerCase())
    await addSkill(token, { skill })
    addedSkills++
  }

  const existingEducation = new Set(
    (profile.education ?? []).map((e) => `${e.school_name.trim().toLowerCase()}|${e.degree.trim().toLowerCase()}`),
  )
  let addedEducation = 0
  for (const entry of extraction.education ?? []) {
    const schoolName = entry.school_name.trim()
    if (!schoolName) continue
    const key = `${schoolName.toLowerCase()}|${entry.degree.trim().toLowerCase()}`
    if (existingEducation.has(key)) continue
    existingEducation.add(key)
    await addEducation(token, {
      school_name: schoolName,
      major: entry.major,
      degree: entry.degree,
      start_date: asDate(entry.start_date),
      end_date: asDate(entry.end_date),
    })
    addedEducation++
  }

  const existingWorkExperience = new Set(
    (profile.work_experience ?? []).map((w) => `${w.company.trim().toLowerCase()}|${w.job_title.trim().toLowerCase()}`),
  )
  let addedWorkExperience = 0
  for (const entry of extraction.work_experience ?? []) {
    const company = entry.company.trim()
    const jobTitle = entry.job_title.trim()
    if (!company || !jobTitle) continue
    const key = `${company.toLowerCase()}|${jobTitle.toLowerCase()}`
    if (existingWorkExperience.has(key)) continue
    existingWorkExperience.add(key)

    const { id } = await addWorkExperience(token, {
      company,
      job_title: jobTitle,
      job_type: 'unknown',
      location: entry.location,
      start_date: asDate(entry.start_date),
      end_date: asDate(entry.end_date),
    })

    for (const raw of entry.bullets ?? []) {
      const bullet = raw.trim()
      if (!bullet) continue
      await addWorkExperienceBullet(token, id, { bullet })
    }

    addedWorkExperience++
  }

  return { updatedBasicInfo, addedEducation, addedSkills, addedWorkExperience }
}

export function useProfileQuery(token: string | null) {
  return useQuery({
    queryKey: ['profile'],
    queryFn: () => fetchProfile(token!),
    enabled: !!token,
    retry: (failureCount, error) =>
      !(error instanceof ApiError && (error.status === 401 || error.status === 404)) && failureCount < 2,
  })
}

function useProfileMutation<TArgs, TResult = unknown>(mutationFn: (args: TArgs) => Promise<TResult>) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['profile'] })
    },
  })
}

export function useUpdateProfileMutation(token: string | null) {
  return useProfileMutation((req: UpdateProfileRequest) => updateProfile(token!, req))
}

export function useAddEducationMutation(token: string | null) {
  return useProfileMutation((req: AddEducationRequest) => addEducation(token!, req))
}

export function useDeleteEducationMutation(token: string | null) {
  return useProfileMutation((id: number) => deleteEducation(token!, id))
}

export function useAddSkillMutation(token: string | null) {
  return useProfileMutation((req: AddSkillRequest) => addSkill(token!, req))
}

export function useDeleteSkillMutation(token: string | null) {
  return useProfileMutation((id: number) => deleteSkill(token!, id))
}

export function useAddWorkExperienceMutation(token: string | null) {
  return useProfileMutation((req: AddWorkExperienceRequest) => addWorkExperience(token!, req))
}

export function useDeleteWorkExperienceMutation(token: string | null) {
  return useProfileMutation((id: number) => deleteWorkExperience(token!, id))
}

export function useAddWorkExperienceBulletMutation(token: string | null) {
  return useProfileMutation(
    ({ workExperienceId, req }: { workExperienceId: number; req: AddWorkExperienceBulletRequest }) =>
      addWorkExperienceBullet(token!, workExperienceId, req),
  )
}

export function useDeleteWorkExperienceBulletMutation(token: string | null) {
  return useProfileMutation(
    ({ workExperienceId, id }: { workExperienceId: number; id: number }) =>
      deleteWorkExperienceBullet(token!, workExperienceId, id),
  )
}

export function useUploadResumeMutation(token: string | null) {
  return useProfileMutation((file: File) => uploadResume(token!, file))
}

export function useUpdateResumeMutation(token: string | null) {
  return useProfileMutation(({ id, update }: { id: number; update: File | string }) => updateResume(token!, id, update))
}

export function useActivateResumeMutation(token: string | null) {
  return useProfileMutation((id: number) => activateResume(token!, id))
}

export function useDeleteResumeMutation(token: string | null) {
  return useProfileMutation((id: number) => deleteResume(token!, id))
}

export function useResumeExtractionQuery(token: string | null, resumeId: number, options: { enabled: boolean }) {
  return useQuery({
    queryKey: ['profile', 'resume', resumeId, 'extraction'],
    queryFn: () => fetchResumeExtraction(token!, resumeId),
    enabled: options.enabled && !!token,
    retry: (failureCount, error) => error instanceof ApiError && error.status === 404 && failureCount < 30,
    retryDelay: 2000,
    refetchInterval: (query) => (query.state.data?.status === 'pending' ? 2000 : false),
  })
}

export function useTriggerResumeExtractionMutation(token: string | null) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (resumeId: number) => triggerResumeExtraction(token!, resumeId),
    onSuccess: (_data, resumeId) => {
      queryClient.invalidateQueries({ queryKey: ['profile', 'resume', resumeId, 'extraction'] })
    },
  })
}

export function useApplyResumeExtractionMutation(token: string | null) {
  return useProfileMutation(({ profile, extraction }: { profile: Profile; extraction: ResumeExtraction }) =>
    applyResumeExtractionToProfile(token!, profile, extraction),
  )
}
