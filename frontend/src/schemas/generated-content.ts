import { z } from 'zod'

export const generatedContentSchema = z.object({
  cover_letter: z.string(),
  tailored_bullets: z.array(z.string())
})
export type GeneratedContent = z.infer<typeof generatedContentSchema>
