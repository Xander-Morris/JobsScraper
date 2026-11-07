import { z } from 'zod'

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
