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
})
export type Profile = z.infer<typeof profileSchema>

export const authResponseSchema = z.object({
  token: z.string(),
})
export type AuthResponse = z.infer<typeof authResponseSchema>
