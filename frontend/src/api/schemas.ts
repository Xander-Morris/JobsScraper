import { z } from 'zod'
import { decodeHtmlEntities } from '../lib/html'

export const workplaceTypeSchema = z.enum(['unknown', 'remote', 'hybrid', 'in_person'])
export type WorkplaceType = z.infer<typeof workplaceTypeSchema>

const htmlDecodedString = z.string().trim().transform(decodeHtmlEntities)

export const jobSchema = z.object({
  id: z.number(),
  title: htmlDecodedString,
  company: htmlDecodedString,
  location: htmlDecodedString,
  workplace_type: workplaceTypeSchema,
  tags: z.array(z.string().trim()),
  salary_min: z.number().nullable(),
  salary_max: z.number().nullable(),
  posted_at: z.iso.datetime({ offset: true }),
  url: z.string().trim(),
  description: htmlDecodedString,
  match_score: z.number().nullable().optional(),
  applied: z.boolean().default(false)
})
export type Job = z.infer<typeof jobSchema>

export const jobSearchResponseSchema = z.object({
  jobs: z.array(jobSchema),
  total: z.number(),
  limit: z.number(),
  offset: z.number()
})
export type JobSearchResponse = z.infer<typeof jobSearchResponseSchema>

export const tagsResponseSchema = z.array(z.string().trim())

export const jobTypeSchema = z.enum(['unknown', 'contract', 'internship', 'part_time', 'full_time'])
export type JobType = z.infer<typeof jobTypeSchema>

export const educationSchema = z.object({
  id: z.number(),
  school_name: z.string().trim(),
  major: z.string().trim(),
  degree: z.string().trim(),
  gpa: z.number().nullable(),
  start_date: z.string().trim().nullable(),
  end_date: z.string().trim().nullable()
})
export type Education = z.infer<typeof educationSchema>

export const skillSchema = z.object({
  id: z.number(),
  skill: z.string().trim()
})
export type Skill = z.infer<typeof skillSchema>

export const workExperienceBulletSchema = z.object({
  id: z.number(),
  bullet: z.string().trim(),
  position: z.number()
})
export type WorkExperienceBullet = z.infer<typeof workExperienceBulletSchema>

export const workExperienceSchema = z.object({
  id: z.number(),
  company: z.string().trim(),
  job_title: z.string().trim(),
  job_type: jobTypeSchema,
  location: z.string().trim().nullable(),
  start_date: z.string().trim().nullable(),
  end_date: z.string().trim().nullable(),
  bullets: z.array(workExperienceBulletSchema).nullable()
})
export type WorkExperience = z.infer<typeof workExperienceSchema>

export const resumeSchema = z.object({
  id: z.number(),
  file_name: z.string().trim(),
  content_type: z.string().trim(),
  file_size: z.number(),
  is_active: z.boolean(),
  created_at: z.string().trim(),
  updated_at: z.string().trim()
})
export type Resume = z.infer<typeof resumeSchema>

export const resumeExtractionStatusSchema = z.enum(['pending', 'completed', 'failed', 'unsupported'])
export type ResumeExtractionStatus = z.infer<typeof resumeExtractionStatusSchema>

export const extractedEducationSchema = z.object({
  school_name: z.string().trim(),
  degree: z.string().trim(),
  major: z.string().trim(),
  start_date: z.string().trim(),
  end_date: z.string().trim()
})

export const extractedWorkExperienceSchema = z.object({
  company: z.string().trim(),
  job_title: z.string().trim(),
  location: z.string().trim(),
  start_date: z.string().trim(),
  end_date: z.string().trim(),
  bullets: z.array(z.string().trim()).nullable()
})

export const extractedProjectSchema = z.object({
  name: z.string().trim(),
  url: z.string().trim(),
  technologies: z.array(z.string().trim()).nullable(),
  bullets: z.array(z.string().trim()).nullable()
})

export const resumeExtractionSchema = z.object({
  resume_id: z.number(),
  status: resumeExtractionStatusSchema,
  full_name: z.string().trim(),
  email: z.string().trim(),
  phone: z.string().trim(),
  linked_in: z.string().trim(),
  github: z.string().trim(),
  portfolio: z.string().trim(),
  summary: z.string().trim(),
  skills: z.array(z.string().trim()).nullable(),
  education: z.array(extractedEducationSchema).nullable(),
  work_experience: z.array(extractedWorkExperienceSchema).nullable(),
  projects: z.array(extractedProjectSchema).nullable(),
  error: z.string().trim(),
  updated_at: z.string().trim()
})
export type ResumeExtraction = z.infer<typeof resumeExtractionSchema>

export const profileSchema = z.object({
  id: z.number(),
  email: z.string().trim(),
  name: z.string().trim(),
  address: z.string().trim(),
  linked_in: z.string().trim(),
  github: z.string().trim(),
  portfolio: z.string().trim(),
  email_notifications: z.boolean(),
  education: z.array(educationSchema).nullable(),
  skills: z.array(skillSchema).nullable(),
  work_experience: z.array(workExperienceSchema).nullable(),
  resumes: z.array(resumeSchema).nullable()
})
export type Profile = z.infer<typeof profileSchema>

export const authResponseSchema = z.object({
  token: z.string().trim()
})
export type AuthResponse = z.infer<typeof authResponseSchema>

export const generatedContentSchema = z.object({
  cover_letter: z.string(),
  tailored_bullets: z.array(z.string())
})
export type GeneratedContent = z.infer<typeof generatedContentSchema>

const list = <T extends z.ZodType>(item: T) =>
  z
    .array(item)
    .nullish()
    .transform((value) => value ?? [])

export const tailoredBulletSchema = z.object({
  text: z.string(),
  source: z.string()
})
export type TailoredBullet = z.infer<typeof tailoredBulletSchema>

export const tailoredWorkExperienceSchema = z.object({
  company: z.string(),
  job_title: z.string(),
  location: z.string(),
  start_date: z.string(),
  end_date: z.string(),
  bullets: list(tailoredBulletSchema)
})

export const tailoredProjectSchema = z.object({
  name: z.string(),
  url: z.string(),
  technologies: list(z.string()),
  bullets: list(tailoredBulletSchema)
})

export const tailoredResumeContentSchema = z.object({
  full_name: z.string(),
  email: z.string(),
  phone: z.string(),
  linked_in: z.string(),
  github: z.string(),
  portfolio: z.string(),
  summary: z.string(),
  skills: list(z.string()),
  education: list(extractedEducationSchema),
  work_experience: list(tailoredWorkExperienceSchema),
  projects: list(tailoredProjectSchema)
})
export type TailoredResumeContent = z.infer<typeof tailoredResumeContentSchema>

export const tailoredResumeSchema = z.object({
  job_id: z.number(),
  content: tailoredResumeContentSchema,
  stale: z.boolean(),
  updated_at: z.string()
})
export type TailoredResume = z.infer<typeof tailoredResumeSchema>
