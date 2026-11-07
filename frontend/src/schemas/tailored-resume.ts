import { z } from 'zod'
import { list } from './common'
import { extractedEducationSchema } from './resume-extraction'

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
