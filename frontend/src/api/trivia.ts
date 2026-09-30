import { z } from 'zod'
import { apiFetch } from './client'
import {
  statusResponseSchema,
  triviaQuizSchema,
  triviaQuizSummarySchema,
  type TriviaDifficulty,
  type TriviaQuiz,
  type TriviaQuizSummary,
  type TriviaSource
} from './schemas'

export interface GenerateTriviaRequest {
  source: TriviaSource
  prompt?: string
  count: number
  difficulty: TriviaDifficulty
}

const jsonInit = (method: string, body: unknown): RequestInit => ({
  method,
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify(body)
})

export function generateTrivia(req: GenerateTriviaRequest): Promise<TriviaQuiz> {
  return apiFetch('/api/trivia', triviaQuizSchema, jsonInit('POST', req))
}

export function fetchTriviaQuizzes(): Promise<TriviaQuizSummary[]> {
  return apiFetch('/api/trivia', z.array(triviaQuizSummarySchema))
}

export function fetchTriviaQuiz(id: number): Promise<TriviaQuiz> {
  return apiFetch(`/api/trivia/${id}`, triviaQuizSchema)
}

export function saveTriviaAnswers(id: number, answers: number[]): Promise<TriviaQuiz> {
  return apiFetch(`/api/trivia/${id}/answers`, triviaQuizSchema, jsonInit('PUT', { answers }))
}

export function retakeTrivia(id: number): Promise<TriviaQuiz> {
  return apiFetch(`/api/trivia/${id}/retake`, triviaQuizSchema, { method: 'POST' })
}

export function deleteTrivia(id: number) {
  return apiFetch(`/api/trivia/${id}`, statusResponseSchema, { method: 'DELETE' })
}
