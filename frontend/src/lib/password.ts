import { z } from 'zod'

// The backend's bcrypt rejects passwords over 72 bytes.
const MAX_PASSWORD_BYTES = 72

export const newPasswordSchema = z
  .string()
  .min(8, 'At least 8 characters')
  .refine((p) => new TextEncoder().encode(p).length <= MAX_PASSWORD_BYTES, 'Too long (72 bytes max)')
