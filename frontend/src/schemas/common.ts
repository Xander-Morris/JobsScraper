import { z } from 'zod'

// The two shapes every mutating endpoint answers with.
export const idResponseSchema = z.object({ id: z.number() })
export const statusResponseSchema = z.object({ status: z.string() })

export const list = <T extends z.ZodType>(item: T) =>
  z
    .array(item)
    .nullish()
    .transform((value) => value ?? [])
