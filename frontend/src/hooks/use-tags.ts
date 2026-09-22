import { useQuery } from '@tanstack/react-query'
import { fetchTags } from '@/src/api/tags'
import { queryKeys } from '@/src/api/query-keys'

export function useTagsQuery() {
  return useQuery({ queryKey: queryKeys.tags, queryFn: fetchTags, staleTime: 10 * 60_000 })
}
