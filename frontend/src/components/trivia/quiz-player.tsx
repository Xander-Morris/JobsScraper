import type { TriviaQuestion, TriviaQuiz } from '@/src/api/schemas'
import { Badge } from '@/src/components/ui/badge'
import { Button, buttonVariants } from '@/src/components/ui/button'
import { Card, CardContent, CardFooter, CardHeader } from '@/src/components/ui/card'
import { useRetakeTriviaMutation, useSaveTriviaAnswersMutation } from '@/src/hooks/use-trivia'
import { cn } from '@/src/lib/utils'
import { Link, useNavigate } from '@tanstack/react-router'
import { CheckIcon, RotateCcwIcon, XIcon } from 'lucide-react'
import { useState } from 'react'

const optionLetters = ['A', 'B', 'C', 'D']

// Keyed by quiz id, so a retake starts from fresh state.
export default function QuizPlayer({ quiz }: { quiz: TriviaQuiz }) {
  const [answers, setAnswers] = useState(quiz.answers)
  // Resume a partly answered quiz at the first unanswered question.
  const [index, setIndex] = useState(Math.min(quiz.answers.length, quiz.questions.length))
  const saveAnswers = useSaveTriviaAnswersMutation()
  const saveError = saveAnswers.error instanceof Error ? saveAnswers.error.message : null
  const total = quiz.questions.length

  if (index >= total) return <QuizResults quiz={quiz} answers={answers} />

  const question = quiz.questions[index]
  const chosen = answers[index] as number | undefined
  const revealed = chosen !== undefined

  function choose(option: number) {
    if (revealed) return

    const next = [...answers.slice(0, index), option]
    setAnswers(next)
    saveAnswers.mutate({ id: quiz.id, answers: next })
  }

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between gap-3 text-xs text-muted-foreground">
          <span>
            Question {index + 1} / {total}
          </span>
          {question.topic && <Badge variant="secondary">{question.topic}</Badge>}
        </div>
        <div className="mt-2 h-1 overflow-hidden rounded-full bg-white/[0.06]">
          <div className="h-full bg-primary transition-all" style={{ width: `${(index / total) * 100}%` }} />
        </div>
        <h2 className="mt-4 text-lg font-medium text-heading">{question.question}</h2>
      </CardHeader>

      <CardContent className="space-y-2">
        {question.options.map((option, i) => (
          <OptionButton
            key={option}
            letter={optionLetters[i]}
            text={option}
            state={optionState(question, i, chosen)}
            disabled={revealed}
            onClick={() => choose(i)}
          />
        ))}

        {revealed && (
          <p aria-live="polite" className="rounded-lg bg-white/[0.03] p-3 text-sm text-secondary-foreground">
            <span
              className={cn('font-medium', chosen === question.correct_index ? 'text-success' : 'text-destructive')}
            >
              {chosen === question.correct_index ? 'Correct. ' : 'Not quite. '}
            </span>
            {question.explanation}
          </p>
        )}

        {saveError && (
          <p role="alert" className="text-xs text-destructive">
            Couldn&rsquo;t save your answer: {saveError}
          </p>
        )}
      </CardContent>

      <CardFooter className="mt-(--card-spacing) justify-end">
        <Button type="button" disabled={!revealed} onClick={() => setIndex(index + 1)}>
          {index + 1 === total ? 'See results' : 'Next'}
        </Button>
      </CardFooter>
    </Card>
  )
}

type OptionState = 'idle' | 'correct' | 'wrong' | 'dimmed'

function optionState(question: TriviaQuestion, option: number, chosen: number | undefined): OptionState {
  if (chosen === undefined) return 'idle'
  if (option === question.correct_index) return 'correct'
  if (option === chosen) return 'wrong'

  return 'dimmed'
}

function OptionButton({
  letter,
  text,
  state,
  disabled,
  onClick
}: {
  letter: string
  text: string
  state: OptionState
  disabled: boolean
  onClick: () => void
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className={cn(
        'flex w-full items-start gap-3 rounded-lg border px-3 py-2.5 text-left text-sm transition-colors outline-none focus-visible:ring-3 focus-visible:ring-ring/50',
        state === 'idle' && 'border-input bg-white/[0.02] text-heading hover:border-white/15 hover:bg-white/[0.05]',
        state === 'correct' && 'border-success/50 bg-success/10 text-heading',
        state === 'wrong' && 'border-destructive/50 bg-destructive/10 text-heading',
        state === 'dimmed' && 'border-border text-muted-foreground'
      )}
    >
      <span className="w-4 shrink-0 font-medium text-muted-foreground">{letter}</span>
      <span className="flex-1">{text}</span>
      {state === 'correct' && <CheckIcon aria-label="Correct answer" className="size-4 shrink-0 text-success" />}
      {state === 'wrong' && <XIcon aria-label="Your answer" className="size-4 shrink-0 text-destructive" />}
    </button>
  )
}

function QuizResults({ quiz, answers }: { quiz: TriviaQuiz; answers: number[] }) {
  const retake = useRetakeTriviaMutation()
  const navigate = useNavigate()
  const retakeError = retake.error instanceof Error ? retake.error.message : null
  const score = quiz.questions.filter((question, i) => answers[i] === question.correct_index).length

  function handleRetake() {
    retake.mutate(quiz.id, {
      onSuccess: (next) => void navigate({ to: '/trivia/$quizId', params: { quizId: String(next.id) } })
    })
  }

  return (
    <div className="space-y-6">
      <Card>
        <CardHeader>
          <p className="text-xs text-muted-foreground">Your score</p>
          <p className="text-4xl font-medium tracking-[-0.03em] text-heading">
            {score} <span className="text-muted-foreground">/ {quiz.questions.length}</span>
          </p>
        </CardHeader>
        <CardFooter className="mt-(--card-spacing) flex-wrap gap-2">
          <Button type="button" variant="outline" onClick={handleRetake} disabled={retake.isPending}>
            <RotateCcwIcon aria-hidden="true" /> {retake.isPending ? 'Starting…' : 'Retake'}
          </Button>
          <Link to="/trivia" className={cn(buttonVariants(), 'no-underline')}>
            New quiz
          </Link>
          {retakeError && (
            <p role="alert" className="w-full text-xs text-destructive">
              {retakeError}
            </p>
          )}
        </CardFooter>
      </Card>

      <ol className="space-y-3">
        {quiz.questions.map((question, i) => {
          const correct = answers[i] === question.correct_index

          return (
            <li key={i} className="rounded-xl border border-border bg-card p-4 text-sm">
              <p className="flex gap-2 font-medium text-heading">
                {correct ? (
                  <CheckIcon aria-label="Correct" className="mt-0.5 size-4 shrink-0 text-success" />
                ) : (
                  <XIcon aria-label="Wrong" className="mt-0.5 size-4 shrink-0 text-destructive" />
                )}
                {question.question}
              </p>
              {!correct && answers[i] !== undefined && (
                <p className="mt-2 text-muted-foreground">
                  Your answer: <span className="text-destructive">{question.options[answers[i]]}</span>
                </p>
              )}
              <p className="mt-1 text-muted-foreground">
                Correct answer: <span className="text-success">{question.options[question.correct_index]}</span>
              </p>
              <p className="mt-2 text-secondary-foreground">{question.explanation}</p>
            </li>
          )
        })}
      </ol>
    </div>
  )
}
