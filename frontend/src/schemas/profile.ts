import { z } from 'zod'

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
