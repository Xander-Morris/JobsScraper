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
