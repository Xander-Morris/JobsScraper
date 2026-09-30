import type { TriviaDifficulty, TriviaSource } from '@/src/api/schemas'
import { Card, CardContent, CardFooter, CardHeader } from '@/src/components/ui/card'
import { Label } from '@/src/components/ui/label'
import { Skeleton } from '@/src/components/ui/skeleton'
import { useAppForm } from '@/src/hooks/use-app-form'
import { useProfileQuery, useResumeExtractionQuery } from '@/src/hooks/use-profile'
import { useGenerateTriviaMutation } from '@/src/hooks/use-trivia'
import { revalidateLogic } from '@tanstack/react-form'
import { useNavigate } from '@tanstack/react-router'
import { useId } from 'react'
import { z } from 'zod'
import SegmentedControl from './segmented-control'

const triviaFormSchema = z
  .object({
    source: z.enum(['resume', 'prompt']),
    prompt: z.string(),
    count: z.number(),
    difficulty: z.enum(['easy', 'medium', 'hard'])
  })
  .superRefine((value, ctx) => {
    const length = value.prompt.trim().length
    if (value.source === 'prompt' && (length < 3 || length > 300)) {
      ctx.addIssue({ code: 'custom', path: ['prompt'], message: 'Enter a topic of 3-300 characters' })
    }
  })

const countOptions = [5, 10, 15].map((value) => ({ value, label: String(value) }))

const difficultyOptions: { value: TriviaDifficulty; label: string }[] = [
  { value: 'easy', label: 'Easy' },
  { value: 'medium', label: 'Medium' },
  { value: 'hard', label: 'Hard' }
]

export default function TriviaGenerator() {
  const { data: profile, isLoading: profileLoading } = useProfileQuery()
  const activeResume = profile?.resumes?.find((resume) => resume.is_active) ?? null
  const { data: extraction, isLoading: extractionLoading } = useResumeExtractionQuery(activeResume?.id ?? 0, {
    enabled: activeResume != null
  })

  // Wait for resume status so the form's default source is right from the first render.
  if (profileLoading || (activeResume && extractionLoading)) return <Skeleton className="h-72 w-full rounded-xl" />

  return (
    <GeneratorForm
      resumeReady={extraction?.status === 'completed'}
      resumeHint={activeResume ? 'Your resume is still being parsed.' : 'Upload and activate a resume to use it.'}
    />
  )
}

function GeneratorForm({ resumeReady, resumeHint }: { resumeReady: boolean; resumeHint: string }) {
  const generate = useGenerateTriviaMutation()
  const navigate = useNavigate()
  const id = useId()
  const error = generate.error instanceof Error ? generate.error.message : null

  const form = useAppForm({
    defaultValues: {
      source: (resumeReady ? 'resume' : 'prompt') as TriviaSource,
      prompt: '',
      count: 10,
      difficulty: 'medium' as TriviaDifficulty
    },
    validationLogic: revalidateLogic(),
    validators: { onDynamic: triviaFormSchema },
    onSubmit: async ({ value }) => {
      const quiz = await generate.mutateAsync({
        source: value.source,
        prompt: value.source === 'prompt' ? value.prompt.trim() : undefined,
        count: value.count,
        difficulty: value.difficulty
      })
      await navigate({ to: '/trivia/$quizId', params: { quizId: String(quiz.id) } })
    }
  })

  return (
    <Card>
      <CardHeader>
        <h2 className="text-sm font-semibold text-heading">New quiz</h2>
        <p className="text-xs text-muted-foreground">
          Test yourself on the skills in your resume, or on any topic you name.
        </p>
      </CardHeader>
      <form.AppForm>
        <form.Form>
          <CardContent className="space-y-4">
            <form.Field name="source">
              {(f) => (
                <div className="space-y-1.5">
                  <SegmentedControl
                    label="Quiz me on"
                    value={f.state.value}
                    onChange={f.handleChange}
                    options={[
                      { value: 'resume', label: 'My resume', disabled: !resumeReady },
                      { value: 'prompt', label: 'A topic' }
                    ]}
                  />
                  {!resumeReady && <p className="text-xs text-muted-foreground">{resumeHint}</p>}
                </div>
              )}
            </form.Field>

            <form.Subscribe selector={(state) => state.values.source}>
              {(source) =>
                source === 'prompt' && (
                  <form.Field name="prompt">
                    {(f) => {
                      const fieldError = (f.state.meta.errors[0] as { message?: string } | undefined)?.message

                      return (
                        <div className="space-y-1.5">
                          <Label htmlFor={`${id}-prompt`}>Topic</Label>
                          <textarea
                            id={`${id}-prompt`}
                            name={f.name}
                            rows={3}
                            maxLength={300}
                            placeholder="e.g. React hooks and rendering, or Postgres indexing"
                            value={f.state.value}
                            onChange={(e) => f.handleChange(e.target.value)}
                            onBlur={f.handleBlur}
                            aria-invalid={!!fieldError}
                            aria-describedby={fieldError ? `${id}-prompt-error` : undefined}
                            className="w-full min-w-0 resize-y rounded-lg border border-input bg-input/30 px-3 py-2 text-base transition-colors outline-none placeholder:text-muted-foreground/70 hover:border-white/15 focus-visible:border-primary/60 focus-visible:ring-3 focus-visible:ring-primary/15 aria-invalid:border-destructive/50 aria-invalid:ring-3 aria-invalid:ring-destructive/40 md:text-sm"
                          />
                          {fieldError && (
                            <p id={`${id}-prompt-error`} className="text-xs text-destructive">
                              {fieldError}
                            </p>
                          )}
                        </div>
                      )
                    }}
                  </form.Field>
                )
              }
            </form.Subscribe>

            <div className="grid gap-4 sm:grid-cols-2">
              <form.Field name="count">
                {(f) => (
                  <SegmentedControl
                    label="Questions"
                    value={f.state.value}
                    onChange={f.handleChange}
                    options={countOptions}
                  />
                )}
              </form.Field>
              <form.Field name="difficulty">
                {(f) => (
                  <SegmentedControl
                    label="Difficulty"
                    value={f.state.value}
                    onChange={f.handleChange}
                    options={difficultyOptions}
                  />
                )}
              </form.Field>
            </div>

            {error && (
              <p role="alert" className="text-xs text-destructive">
                {error}
              </p>
            )}
          </CardContent>
          <CardFooter className="mt-(--card-spacing) flex-wrap gap-3">
            <form.SubmitButton>{generate.isPending ? 'Generating…' : 'Generate quiz'}</form.SubmitButton>
            {generate.isPending && <p className="text-xs text-muted-foreground">This can take up to a minute.</p>}
          </CardFooter>
        </form.Form>
      </form.AppForm>
    </Card>
  )
}
