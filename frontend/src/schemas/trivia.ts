import { z } from 'zod'
import { list } from './common'

export const triviaSourceSchema = z.enum(['resume', 'prompt'])
export type TriviaSource = z.infer<typeof triviaSourceSchema>

export const triviaDifficultySchema = z.enum(['easy', 'medium', 'hard'])
export type TriviaDifficulty = z.infer<typeof triviaDifficultySchema>

export const triviaQuestionSchema = z.object({
  question: z.string(),
  options: z.array(z.string()),
  correct_index: z.number(),
  explanation: z.string(),
  topic: z.string()
})
export type TriviaQuestion = z.infer<typeof triviaQuestionSchema>

const triviaQuizBase = {
  id: z.number(),
  source: triviaSourceSchema,
  topic: z.string(),
  difficulty: triviaDifficultySchema,
  score: z.number(),
  completed_at: z.string().nullable(),
  created_at: z.string()
}

export const triviaQuizSchema = z.object({
  ...triviaQuizBase,
  questions: list(triviaQuestionSchema),
  answers: list(z.number())
})
export type TriviaQuiz = z.infer<typeof triviaQuizSchema>

export const triviaQuizSummarySchema = z.object({
  ...triviaQuizBase,
  question_count: z.number(),
  answered_count: z.number()
})
export type TriviaQuizSummary = z.infer<typeof triviaQuizSummarySchema>
