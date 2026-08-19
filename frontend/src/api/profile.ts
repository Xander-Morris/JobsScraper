import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { z } from 'zod'
import { apiFetch } from './client'
import {
  authResponseSchema,
  profileSchema,
  type AuthResponse,
  type Profile,
} from './schemas'

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

export function useProfileQuery(token: string | null) {
  return useQuery({
    queryKey: ['profile'],
    queryFn: () => fetchProfile(token!),
    enabled: !!token,
  })
}

function useProfileMutation<TArgs>(mutationFn: (args: TArgs) => Promise<unknown>) {
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
