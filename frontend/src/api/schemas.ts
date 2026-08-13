import { z } from 'zod'

export const workplaceTypeSchema = z.enum(['unknown', 'remote', 'hybrid', 'in_person'])
export type WorkplaceType = z.infer<typeof workplaceTypeSchema>

export const jobSchema = z.object({
  id: z.number(),
  title: z.string(),
  company: z.string(),
  location: z.string(),
  workplace_type: workplaceTypeSchema,
  tags: z.array(z.string()),
  salary_min: z.number().nullable(),
  salary_max: z.number().nullable(),
  posted_at: z.iso.datetime({ offset: true }),
  url: z.string(),
  description: z.string(),
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
