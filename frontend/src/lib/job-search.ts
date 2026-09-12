import { z } from 'zod'

export const datePostedOptions = ['24h', '3d', 'week', 'month'] as const

export const jobSearchSchema = z.object({
  q: z.string().optional(),
  workplaceType: z.enum(['remote', 'hybrid', 'in_person']).optional(),
  minSalary: z.number().optional(),
  maxSalary: z.number().optional(),
  datePosted: z.enum(datePostedOptions).optional(),
  tags: z.array(z.string()).optional(),
  sort: z.enum(['relevance', 'date']).optional(),
  page: z.number().optional()
})

export type JobSearchState = z.infer<typeof jobSearchSchema>
