import { useQuery } from '@tanstack/react-query'
import { apiFetch } from './client'
import { tagsResponseSchema } from './schemas'

export function fetchTags(): Promise<string[]> {
  return apiFetch('/api/tags', tagsResponseSchema)
}

export function useTagsQuery() {
  return useQuery({ queryKey: ['tags'], queryFn: fetchTags })
}
