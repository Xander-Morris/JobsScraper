import type { TriviaQuizSummary } from '@/src/api/schemas'
import { Badge } from '@/src/components/ui/badge'
import { Button } from '@/src/components/ui/button'
import { Skeleton } from '@/src/components/ui/skeleton'
import { useDeleteTriviaMutation, useTriviaQuizzesQuery } from '@/src/hooks/use-trivia'
import { formatRelativeDate } from '@/src/lib/format'
import { Link } from '@tanstack/react-router'
import { FileTextIcon, SparklesIcon, Trash2Icon } from 'lucide-react'

export default function TriviaHistory() {
  const { data: quizzes, isPending, isError, error } = useTriviaQuizzesQuery()
  const deleteQuiz = useDeleteTriviaMutation()
  const deleteError = deleteQuiz.error instanceof Error ? deleteQuiz.error.message : null

  return (
    <section className="space-y-3">
      <h2 className="text-sm font-semibold text-heading">Past quizzes</h2>

      {isPending && <Skeleton className="h-24 w-full rounded-xl" />}
      {isError && (
        <p role="alert" className="text-sm text-destructive">
          {error.message}
        </p>
      )}
      {quizzes?.length === 0 && <p className="text-sm text-muted-foreground">No quizzes yet. Generate one above.</p>}

      {quizzes && quizzes.length > 0 && (
        <ul className="divide-y divide-border rounded-xl border border-border bg-card">
          {quizzes.map((quiz) => (
            <QuizRow
              key={quiz.id}
              quiz={quiz}
              onDelete={() => deleteQuiz.mutate(quiz.id)}
              deleting={deleteQuiz.isPending && deleteQuiz.variables === quiz.id}
            />
          ))}
        </ul>
      )}

      {deleteError && (
        <p role="alert" className="text-xs text-destructive">
          {deleteError}
        </p>
      )}
    </section>
  )
}

function QuizRow({ quiz, onDelete, deleting }: { quiz: TriviaQuizSummary; onDelete: () => void; deleting: boolean }) {
  const SourceIcon = quiz.source === 'resume' ? FileTextIcon : SparklesIcon
  const progress = quiz.completed_at
    ? `${quiz.score}/${quiz.question_count}`
    : `${quiz.answered_count}/${quiz.question_count} answered`

  return (
    <li className="flex items-center gap-3 px-4 py-3">
      <SourceIcon aria-hidden="true" className="size-4 shrink-0 text-muted-foreground" />
      <Link
        to="/trivia/$quizId"
        params={{ quizId: String(quiz.id) }}
        className="min-w-0 flex-1 text-sm text-heading no-underline hover:text-primary"
      >
        <span className="block truncate">{quiz.topic}</span>
        <span className="text-xs text-muted-foreground">{formatRelativeDate(quiz.created_at)}</span>
      </Link>
      <Badge variant="outline" className="capitalize">
        {quiz.difficulty}
      </Badge>
      <Badge variant={quiz.completed_at ? 'default' : 'secondary'}>{progress}</Badge>
      <Button
        type="button"
        variant="ghost"
        size="icon-sm"
        aria-label={`Delete quiz: ${quiz.topic}`}
        onClick={onDelete}
        disabled={deleting}
      >
        <Trash2Icon aria-hidden="true" />
      </Button>
    </li>
  )
}
