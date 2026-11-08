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
  // Left out when the source gave no post date.
  posted_at: z.iso.datetime({ offset: true }).optional(),
  url: z.string().trim(),
  match_score: z.number().nullable().optional(),
  match_percent: z.number().int().min(0).max(100).nullable().optional(),
  applied: z.boolean().default(false)
})
export type Job = z.infer<typeof jobSchema>

export const jobListItemSchema = jobSchema
export type JobListItem = Job

// Fetched live from the posting; null when the posting doesn't expose one.
export const jobDescriptionSchema = z.object({ description: htmlDecodedString.nullable() })
export type JobDescription = z.infer<typeof jobDescriptionSchema>

export const jobSearchResponseSchema = z.object({
  jobs: z.array(jobListItemSchema),
  total: z.number(),
  limit: z.number(),
  offset: z.number()
})
export type JobSearchResponse = z.infer<typeof jobSearchResponseSchema>

export const tagsResponseSchema = z.array(z.string().trim())
