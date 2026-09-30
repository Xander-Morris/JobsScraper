import { useMutation, useQuery, useQueryClient, type QueryClient } from '@tanstack/react-query'
import { queryKeys } from '@/src/api/query-keys'
import type { TriviaQuiz } from '@/src/api/schemas'
import {
  deleteTrivia,
  fetchTriviaQuiz,
  fetchTriviaQuizzes,
  generateTrivia,
  retakeTrivia,
  saveTriviaAnswers,
  type GenerateTriviaRequest
} from '@/src/api/trivia'
import { useAuth } from '@/src/stores/auth-store'

// Caches the returned quiz and refreshes the history list.
function storeQuiz(queryClient: QueryClient, quiz: TriviaQuiz) {
  queryClient.setQueryData(queryKeys.triviaQuiz(quiz.id), quiz)
  void queryClient.invalidateQueries({ queryKey: queryKeys.trivia, exact: true })
}

export function useTriviaQuizzesQuery() {
  const { isAuthenticated } = useAuth()

  return useQuery({ queryKey: queryKeys.trivia, queryFn: fetchTriviaQuizzes, enabled: isAuthenticated })
}

export function useTriviaQuizQuery(id: number) {
  const { isAuthenticated } = useAuth()

  return useQuery({ queryKey: queryKeys.triviaQuiz(id), queryFn: () => fetchTriviaQuiz(id), enabled: isAuthenticated })
}

export function useGenerateTriviaMutation() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (req: GenerateTriviaRequest) => generateTrivia(req),
    onSuccess: (quiz) => storeQuiz(queryClient, quiz)
  })
}

export function useSaveTriviaAnswersMutation() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ id, answers }: { id: number; answers: number[] }) => saveTriviaAnswers(id, answers),
    onSuccess: (quiz) => storeQuiz(queryClient, quiz)
  })
}

export function useRetakeTriviaMutation() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (id: number) => retakeTrivia(id),
    onSuccess: (quiz) => storeQuiz(queryClient, quiz)
  })
}

export function useDeleteTriviaMutation() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (id: number) => deleteTrivia(id),
    onSuccess: (_data, id) => {
      queryClient.removeQueries({ queryKey: queryKeys.triviaQuiz(id) })
      void queryClient.invalidateQueries({ queryKey: queryKeys.trivia, exact: true })
    }
  })
}
