import { z } from 'zod'
import { decodeHtmlEntities } from '../lib/html'

export const workplaceTypeSchema = z.enum(['unknown', 'remote', 'hybrid', 'in_person'])
export type WorkplaceType = z.infer<typeof workplaceTypeSchema>

const htmlDecodedString = z.string().transform(decodeHtmlEntities)

export const jobSchema = z.object({
  id: z.number(),
  title: htmlDecodedString,
  company: htmlDecodedString,
  location: htmlDecodedString,
  workplace_type: workplaceTypeSchema,
  tags: z.array(z.string()),
  salary_min: z.number().nullable(),
  salary_max: z.number().nullable(),
  posted_at: z.iso.datetime({ offset: true }),
  url: z.string(),
  description: htmlDecodedString,
})
export type Job = z.infer<typeof jobSchema>

export const jobSearchResponseSchema = z.object({
  jobs: z.array(jobSchema),
  total: z.number(),
  limit: z.number(),
  offset: z.number(),
})
export type JobSearchResponse = z.infer<typeof jobSearchResponseSchema>

export const tagsResponseSchema = z.array(z.string())

export const jobTypeSchema = z.enum(['unknown', 'contract', 'internship', 'part_time', 'full_time'])
export type JobType = z.infer<typeof jobTypeSchema>

export const educationSchema = z.object({
  id: z.number(),
  school_name: z.string(),
  major: z.string(),
  degree: z.string(),
  gpa: z.number().nullable(),
  start_date: z.string().nullable(),
  end_date: z.string().nullable(),
})
export type Education = z.infer<typeof educationSchema>

export const skillSchema = z.object({
  id: z.number(),
  skill: z.string(),
})
export type Skill = z.infer<typeof skillSchema>

export const workExperienceBulletSchema = z.object({
  id: z.number(),
  bullet: z.string(),
  position: z.number(),
})
export type WorkExperienceBullet = z.infer<typeof workExperienceBulletSchema>

export const workExperienceSchema = z.object({
  id: z.number(),
  company: z.string(),
  job_title: z.string(),
  job_type: jobTypeSchema,
  location: z.string().nullable(),
  start_date: z.string().nullable(),
  end_date: z.string().nullable(),
  bullets: z.array(workExperienceBulletSchema).nullable(),
})
export type WorkExperience = z.infer<typeof workExperienceSchema>

export const resumeSchema = z.object({
  id: z.number(),
  file_name: z.string(),
  content_type: z.string(),
  file_size: z.number(),
  is_active: z.boolean(),
  created_at: z.string(),
  updated_at: z.string(),
})
export type Resume = z.infer<typeof resumeSchema>

export const resumeExtractionStatusSchema = z.enum(['pending', 'completed', 'failed', 'unsupported'])
export type ResumeExtractionStatus = z.infer<typeof resumeExtractionStatusSchema>

export const extractedEducationSchema = z.object({
  school_name: z.string(),
  degree: z.string(),
  major: z.string(),
  start_date: z.string(),
  end_date: z.string(),
})

export const extractedWorkExperienceSchema = z.object({
  company: z.string(),
  job_title: z.string(),
  location: z.string(),
  start_date: z.string(),
  end_date: z.string(),
  bullets: z.array(z.string()).nullable(),
})

export const extractedProjectSchema = z.object({
  name: z.string(),
  url: z.string(),
  technologies: z.array(z.string()).nullable(),
  bullets: z.array(z.string()).nullable(),
})

export const resumeExtractionSchema = z.object({
  resume_id: z.number(),
  status: resumeExtractionStatusSchema,
  full_name: z.string(),
  email: z.string(),
  phone: z.string(),
  linked_in: z.string(),
  github: z.string(),
  portfolio: z.string(),
  summary: z.string(),
  skills: z.array(z.string()).nullable(),
  education: z.array(extractedEducationSchema).nullable(),
  work_experience: z.array(extractedWorkExperienceSchema).nullable(),
  projects: z.array(extractedProjectSchema).nullable(),
  error: z.string(),
  updated_at: z.string(),
})
export type ResumeExtraction = z.infer<typeof resumeExtractionSchema>

export const profileSchema = z.object({
  id: z.number(),
  email: z.string(),
  name: z.string(),
  address: z.string(),
  linked_in: z.string(),
  github: z.string(),
  portfolio: z.string(),
  education: z.array(educationSchema).nullable(),
  skills: z.array(skillSchema).nullable(),
  work_experience: z.array(workExperienceSchema).nullable(),
  resumes: z.array(resumeSchema).nullable(),
})
export type Profile = z.infer<typeof profileSchema>

export const authResponseSchema = z.object({
  token: z.string(),
})
export type AuthResponse = z.infer<typeof authResponseSchema>
