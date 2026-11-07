import { z } from 'zod'

export const authResponseSchema = z.object({
  token: z.string().trim()
})
export type AuthResponse = z.infer<typeof authResponseSchema>
