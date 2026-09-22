import { API_BASE_URL, ApiError, apiFetch, authHeaders } from './client'
import {
  authResponseSchema,
  idResponseSchema,
  statusResponseSchema,
  profileSchema,
  resumeExtractionSchema,
  type AuthResponse,
  type Profile,
  type ResumeExtraction
} from './schemas'

export interface UpdateProfileRequest {
  name: string
  address: string
  linked_in: string
  github: string
  portfolio: string
  email_notifications: boolean
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

export interface ProfileCredentials {
  email: string
  password: string
}

const jsonHeaders: HeadersInit = { 'Content-Type': 'application/json' }

export function createProfile(email: string, password: string): Promise<AuthResponse> {
  return apiFetch('/api/profile/create', authResponseSchema, {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ email, password })
  })
}

export function loginProfile(email: string, password: string): Promise<AuthResponse> {
  return apiFetch('/api/profile/login', authResponseSchema, {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ email, password })
  })
}

export function requestPasswordReset(email: string) {
  return apiFetch('/api/profile/password-reset', statusResponseSchema, {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ email })
  })
}

export function confirmPasswordReset(token: string, password: string): Promise<AuthResponse> {
  return apiFetch('/api/profile/password-reset/confirm', authResponseSchema, {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify({ token, password })
  })
}

export function fetchProfile(): Promise<Profile> {
  return apiFetch('/api/profile', profileSchema)
}

export function updateProfile(req: UpdateProfileRequest) {
  return apiFetch('/api/profile', statusResponseSchema, {
    method: 'PUT',
    headers: jsonHeaders,
    body: JSON.stringify(req)
  })
}

export function addEducation(req: AddEducationRequest) {
  return apiFetch('/api/profile/education', idResponseSchema, {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify(req)
  })
}

export function updateEducation(id: number, req: AddEducationRequest) {
  return apiFetch(`/api/profile/education/${id}`, statusResponseSchema, {
    method: 'PUT',
    headers: jsonHeaders,
    body: JSON.stringify(req)
  })
}

export function deleteEducation(id: number) {
  return apiFetch(`/api/profile/education/${id}`, statusResponseSchema, { method: 'DELETE' })
}

export function addSkill(req: AddSkillRequest) {
  return apiFetch('/api/profile/skills', idResponseSchema, {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify(req)
  })
}

export function deleteSkill(id: number) {
  return apiFetch(`/api/profile/skills/${id}`, statusResponseSchema, { method: 'DELETE' })
}

export function addWorkExperience(req: AddWorkExperienceRequest) {
  return apiFetch('/api/profile/work-experience', idResponseSchema, {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify(req)
  })
}

export function updateWorkExperience(id: number, req: AddWorkExperienceRequest) {
  return apiFetch(`/api/profile/work-experience/${id}`, statusResponseSchema, {
    method: 'PUT',
    headers: jsonHeaders,
    body: JSON.stringify(req)
  })
}

export function deleteWorkExperience(id: number) {
  return apiFetch(`/api/profile/work-experience/${id}`, statusResponseSchema, { method: 'DELETE' })
}

export function addWorkExperienceBullet(workExperienceId: number, req: AddWorkExperienceBulletRequest) {
  return apiFetch(`/api/profile/work-experience/${workExperienceId}/bullets`, idResponseSchema, {
    method: 'POST',
    headers: jsonHeaders,
    body: JSON.stringify(req)
  })
}

export function deleteWorkExperienceBullet(workExperienceId: number, id: number) {
  return apiFetch(`/api/profile/work-experience/${workExperienceId}/bullets/${id}`, statusResponseSchema, {
    method: 'DELETE'
  })
}

export function uploadResume(file: File) {
  const body = new FormData()
  body.append('resume', file)

  return apiFetch('/api/profile/resumes', idResponseSchema, { method: 'POST', body })
}

export function updateResume(id: number, update: File | string) {
  const body = new FormData()
  if (typeof update === 'string') {
    body.append('file_name', update)
  } else {
    body.append('resume', update)
  }

  return apiFetch(`/api/profile/resumes/${id}`, statusResponseSchema, { method: 'PUT', body })
}

export function deleteResume(id: number) {
  return apiFetch(`/api/profile/resumes/${id}`, statusResponseSchema, { method: 'DELETE' })
}

export function activateResume(id: number) {
  return apiFetch(`/api/profile/resumes/${id}/activate`, statusResponseSchema, { method: 'POST' })
}

export function fetchResumeExtraction(id: number): Promise<ResumeExtraction> {
  return apiFetch(`/api/profile/resumes/${id}/extraction`, resumeExtractionSchema)
}

export function triggerResumeExtraction(id: number) {
  return apiFetch(`/api/profile/resumes/${id}/extraction`, statusResponseSchema, { method: 'POST' })
}

// Downloads bypass apiFetch: the response is a file, not a schema-checked JSON body.
export async function downloadResume(id: number): Promise<Blob> {
  const response = await fetch(`${API_BASE_URL}/api/profile/resumes/${id}/download`, {
    credentials: 'include',
    headers: authHeaders()
  })

  if (!response.ok) {
    throw new ApiError(response.statusText || 'Unable to download resume', response.status)
  }

  return response.blob()
}
