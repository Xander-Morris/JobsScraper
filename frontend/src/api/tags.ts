import { apiFetch } from './client'
import { tagsResponseSchema } from './schemas'

export function fetchTags(): Promise<string[]> {
  return apiFetch('/api/tags', tagsResponseSchema)
}
