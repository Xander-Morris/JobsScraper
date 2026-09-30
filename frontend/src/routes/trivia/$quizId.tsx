import { createFileRoute, Link } from '@tanstack/react-router'
import { ArrowLeftIcon } from 'lucide-react'
import QuizPlayer from '@/src/components/trivia/quiz-player'
import { Skeleton } from '@/src/components/ui/skeleton'
import { useTriviaQuizQuery } from '@/src/hooks/use-trivia'

export const Route = createFileRoute('/trivia/$quizId')({ component: QuizPage })

function QuizPage() {
  const { quizId } = Route.useParams()
  const { data: quiz, isPending, isError, error } = useTriviaQuizQuery(Number(quizId))

  return (
    <div className="space-y-6">
      <Link
        to="/trivia"
        className="inline-flex items-center gap-1.5 text-sm text-muted-foreground no-underline hover:text-heading"
      >
        <ArrowLeftIcon aria-hidden="true" className="size-4" />
        Back to trivia
      </Link>

      {isPending && <Skeleton className="h-80 w-full rounded-xl" />}
      {isError && (
        <p role="alert" className="text-sm text-destructive">
          {error.message}
        </p>
      )}

      {quiz && (
        <>
          <div>
            <h1 className="truncate text-2xl font-medium tracking-[-0.03em]">{quiz.topic}</h1>
            <p className="mt-1 text-sm text-muted-foreground capitalize">{quiz.difficulty}</p>
          </div>
          <QuizPlayer key={quiz.id} quiz={quiz} />
        </>
      )}
    </div>
  )
}
