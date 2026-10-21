import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { ResumeExtraction } from '@/src/api/schemas'
import { signIn } from '@/src/test/auth-mock'
import { makeExtraction, makeProfile, makeQuiz, makeQuizSummary, makeResume } from '@/src/test/fixtures'
import { renderWithProviders } from '@/src/test/render'
import { gate } from '@/src/test/gate'
import { api, server } from '@/src/test/server'
import QuizPlayer from '@/src/components/trivia/quiz-player'
import SegmentedControl from '@/src/components/trivia/segmented-control'
import TriviaGenerator from '@/src/components/trivia/trivia-generator'
import TriviaHistory from '@/src/components/trivia/trivia-history'

vi.mock('@/src/stores/auth-store', () => import('@/src/test/auth-mock'))

beforeEach(() => signIn())

describe('SegmentedControl', () => {
  it('marks the selected option and reports changes', async () => {
    const onChange = vi.fn()
    render(
      <SegmentedControl
        label="Difficulty"
        value="easy"
        onChange={onChange}
        options={[
          { value: 'easy', label: 'Easy' },
          { value: 'hard', label: 'Hard' },
          { value: 'nope', label: 'Nope', disabled: true }
        ]}
      />
    )

    const group = screen.getByRole('group', { name: 'Difficulty' })
    expect(within(group).getByRole('button', { name: 'Easy' })).toHaveAttribute('aria-pressed', 'true')
    expect(within(group).getByRole('button', { name: 'Hard' })).toHaveAttribute('aria-pressed', 'false')
    expect(within(group).getByRole('button', { name: 'Nope' })).toBeDisabled()

    await userEvent.click(within(group).getByRole('button', { name: 'Hard' }))
    expect(onChange).toHaveBeenCalledWith('hard')
  })
})

describe('QuizPlayer', () => {
  function serveAnswers() {
    const saved: number[][] = []
    server.use(
      http.put(api('/api/trivia/5/answers'), async ({ request }) => {
        const { answers } = (await request.json()) as { answers: number[] }
        saved.push(answers)
        return HttpResponse.json(makeQuiz({ answers }))
      })
    )
    return saved
  }

  it('reveals each answer, saves progress and shows results', async () => {
    const saved = serveAnswers()
    await renderWithProviders(<QuizPlayer quiz={makeQuiz()} />)

    expect(screen.getByText('Question 1 / 2')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled()

    // Wrong answer first.
    await userEvent.click(screen.getByRole('button', { name: /A thread/ }))
    expect(screen.getByText('Not quite.')).toBeInTheDocument()
    expect(screen.getByText('Managed by the Go runtime.')).toBeInTheDocument()
    expect(screen.getByLabelText('Your answer')).toBeInTheDocument()
    expect(screen.getByLabelText('Correct answer')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /A lightweight thread/ })).toBeDisabled()
    await waitFor(() => expect(saved).toEqual([[0]]))

    await userEvent.click(screen.getByRole('button', { name: 'Next' }))
    expect(screen.getByText('Question 2 / 2')).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: /Runs later/ }))
    expect(screen.getByText('Correct.')).toBeInTheDocument()
    await waitFor(() => expect(saved.at(-1)).toEqual([0, 0]))

    await userEvent.click(screen.getByRole('button', { name: 'See results' }))

    expect(screen.getByText('Your score')).toBeInTheDocument()
    expect(screen.getByText('/ 2').parentElement).toHaveTextContent('1 / 2')
    expect(screen.getByText('A thread', { selector: 'span' })).toBeInTheDocument()
    expect(screen.getAllByLabelText('Correct')).toHaveLength(1)
    expect(screen.getAllByLabelText('Wrong')).toHaveLength(1)
    expect(screen.getByRole('link', { name: 'New quiz' })).toHaveAttribute('href', '/trivia')
  })

  it('resumes at the first unanswered question', async () => {
    await renderWithProviders(<QuizPlayer quiz={makeQuiz({ answers: [1] })} />)

    expect(screen.getByText('Question 2 / 2')).toBeInTheDocument()
  })

  it('opens finished quizzes on the results', async () => {
    await renderWithProviders(<QuizPlayer quiz={makeQuiz({ answers: [1, 0] })} />)

    expect(screen.getByText('/ 2').parentElement).toHaveTextContent('2 / 2')
  })

  it('shows a save error', async () => {
    server.use(
      http.put(api('/api/trivia/5/answers'), () => HttpResponse.json({ error: 'quiz not found' }, { status: 404 }))
    )
    await renderWithProviders(<QuizPlayer quiz={makeQuiz()} />)

    await userEvent.click(screen.getByRole('button', { name: /A thread/ }))

    expect(await screen.findByRole('alert')).toHaveTextContent('quiz not found')
  })

  it('retakes into a new quiz', async () => {
    server.use(http.post(api('/api/trivia/5/retake'), () => HttpResponse.json(makeQuiz({ id: 6 }))))
    const { router } = await renderWithProviders(<QuizPlayer quiz={makeQuiz({ answers: [1, 0] })} />, {
      path: '/trivia/5'
    })

    await userEvent.click(screen.getByRole('button', { name: /Retake/ }))

    await waitFor(() => expect(router.state.location.pathname).toBe('/trivia/6'))
  })

  it('shows retake progress', async () => {
    const hold = gate()
    server.use(
      http.post(api('/api/trivia/5/retake'), async () => {
        await hold.wait
        return HttpResponse.json(makeQuiz({ id: 6 }))
      })
    )
    await renderWithProviders(<QuizPlayer quiz={makeQuiz({ answers: [1, 0] })} />)

    await userEvent.click(screen.getByRole('button', { name: /Retake/ }))
    expect(await screen.findByRole('button', { name: /Starting…/ })).toBeDisabled()
    hold.release()
  })

  it('shows a retake error', async () => {
    server.use(
      http.post(api('/api/trivia/5/retake'), () =>
        HttpResponse.json({ error: 'failed to retake quiz' }, { status: 500 })
      )
    )
    await renderWithProviders(<QuizPlayer quiz={makeQuiz({ answers: [1, 0] })} />)

    await userEvent.click(screen.getByRole('button', { name: /Retake/ }))

    expect(await screen.findByRole('alert')).toHaveTextContent('failed to retake quiz')
  })
})

describe('TriviaGenerator', () => {
  function serveProfile({
    resume = true,
    status = 'completed'
  }: { resume?: boolean; status?: ResumeExtraction['status'] } = {}) {
    server.use(
      http.get(api('/api/profile'), () =>
        HttpResponse.json(makeProfile({ resumes: resume ? [makeResume({ id: 10 })] : [] }))
      ),
      http.get(api('/api/profile/resumes/10/extraction'), () => HttpResponse.json(makeExtraction({ status })))
    )
  }

  it('defaults to the resume when it is ready and navigates to the new quiz', async () => {
    serveProfile()
    let body: unknown
    server.use(
      http.post(api('/api/trivia'), async ({ request }) => {
        body = await request.json()
        return HttpResponse.json(makeQuiz({ id: 9 }))
      })
    )
    const { router } = await renderWithProviders(<TriviaGenerator />, { path: '/trivia' })

    const source = await screen.findByRole('group', { name: 'Quiz me on' })
    expect(within(source).getByRole('button', { name: 'My resume' })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.queryByLabelText('Topic')).not.toBeInTheDocument()

    await userEvent.click(within(screen.getByRole('group', { name: 'Questions' })).getByRole('button', { name: '15' }))
    await userEvent.click(
      within(screen.getByRole('group', { name: 'Difficulty' })).getByRole('button', { name: 'Hard' })
    )
    await userEvent.click(screen.getByRole('button', { name: 'Generate quiz' }))

    await waitFor(() => expect(router.state.location.pathname).toBe('/trivia/9'))
    expect(body).toEqual({ source: 'resume', count: 15, difficulty: 'hard' })
  })

  it('quizzes on a typed topic and validates its length', async () => {
    serveProfile({ resume: false })
    let body: unknown
    server.use(
      http.post(api('/api/trivia'), async ({ request }) => {
        body = await request.json()
        return HttpResponse.json(makeQuiz({ id: 9 }))
      })
    )
    await renderWithProviders(<TriviaGenerator />)

    expect(await screen.findByText('Upload and activate a resume to use it.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'My resume' })).toBeDisabled()

    const topic = screen.getByLabelText('Topic')
    await userEvent.type(topic, 'Go')
    await userEvent.click(screen.getByRole('button', { name: 'Generate quiz' }))
    expect(await screen.findByText('Enter a topic of 3-300 characters')).toBeInTheDocument()
    expect(topic).toHaveAttribute('aria-invalid', 'true')
    expect(body).toBeUndefined()

    await userEvent.type(topic, ' channels  ')
    await userEvent.click(screen.getByRole('button', { name: 'Generate quiz' }))

    await waitFor(() =>
      expect(body).toEqual({ source: 'prompt', prompt: 'Go channels', count: 10, difficulty: 'medium' })
    )
  })

  it('explains a resume still being parsed', async () => {
    serveProfile({ status: 'pending' })
    await renderWithProviders(<TriviaGenerator />)

    expect(await screen.findByText('Your resume is still being parsed.')).toBeInTheDocument()
  })

  it('shows generation progress', async () => {
    serveProfile({ resume: false })
    const hold = gate()
    server.use(
      http.post(api('/api/trivia'), async () => {
        await hold.wait
        return HttpResponse.json(makeQuiz({ id: 9 }))
      })
    )
    await renderWithProviders(<TriviaGenerator />)

    await userEvent.type(await screen.findByLabelText('Topic'), 'Postgres')
    await userEvent.click(screen.getByRole('button', { name: 'Generate quiz' }))
    expect(await screen.findByText('This can take up to a minute.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Generating…' })).toBeInTheDocument()
    hold.release()
  })

  it('shows a generation error', async () => {
    serveProfile({ resume: false })
    server.use(
      http.post(api('/api/trivia'), () => HttpResponse.json({ error: 'failed to generate quiz' }, { status: 502 }))
    )
    await renderWithProviders(<TriviaGenerator />)

    await userEvent.type(await screen.findByLabelText('Topic'), 'Postgres')
    await userEvent.click(screen.getByRole('button', { name: 'Generate quiz' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('failed to generate quiz')
  })
})

describe('TriviaHistory', () => {
  it('lists quizzes with progress and deletes one', async () => {
    let deleted = 0
    server.use(
      http.get(api('/api/trivia'), () =>
        HttpResponse.json([
          makeQuizSummary({ id: 1, topic: 'Go', answered_count: 1 }),
          makeQuizSummary({
            id: 2,
            topic: 'Your resume',
            source: 'resume',
            score: 2,
            completed_at: '2026-01-02T00:00:00Z',
            difficulty: 'hard'
          })
        ])
      ),
      http.delete(api('/api/trivia/1'), () => {
        deleted++
        return HttpResponse.json({ status: 'deleted' })
      })
    )
    await renderWithProviders(<TriviaHistory />)

    expect(await screen.findByRole('link', { name: /Go/ })).toHaveAttribute('href', '/trivia/1')
    expect(screen.getByText('1/2 answered')).toBeInTheDocument()
    expect(screen.getByText('2/2')).toBeInTheDocument()
    expect(screen.getByText('hard')).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Delete quiz: Go' }))
    await waitFor(() => expect(deleted).toBe(1))
  })

  it('marks only the quiz being deleted', async () => {
    const hold = gate()
    server.use(
      http.get(api('/api/trivia'), () =>
        HttpResponse.json([makeQuizSummary({ id: 1, topic: 'Go' }), makeQuizSummary({ id: 2, topic: 'SQL' })])
      ),
      http.delete(api('/api/trivia/1'), async () => {
        await hold.wait
        return HttpResponse.json({ status: 'deleted' })
      })
    )
    await renderWithProviders(<TriviaHistory />)

    await userEvent.click(await screen.findByRole('button', { name: 'Delete quiz: Go' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Delete quiz: Go' })).toBeDisabled())
    expect(screen.getByRole('button', { name: 'Delete quiz: SQL' })).toBeEnabled()
    hold.release()
  })

  it('shows the empty state', async () => {
    server.use(http.get(api('/api/trivia'), () => HttpResponse.json([])))
    await renderWithProviders(<TriviaHistory />)

    expect(await screen.findByText('No quizzes yet. Generate one above.')).toBeInTheDocument()
  })

  it('shows load and delete errors', async () => {
    server.use(
      http.get(api('/api/trivia'), () => HttpResponse.json({ error: 'failed to load quizzes' }, { status: 500 }))
    )
    const { unmount } = await renderWithProviders(<TriviaHistory />)
    expect(await screen.findByRole('alert')).toHaveTextContent('failed to load quizzes')
    unmount()

    server.use(
      http.get(api('/api/trivia'), () => HttpResponse.json([makeQuizSummary({ id: 1 })])),
      http.delete(api('/api/trivia/1'), () => HttpResponse.json({ error: 'quiz not found' }, { status: 404 }))
    )
    await renderWithProviders(<TriviaHistory />)
    await userEvent.click(await screen.findByRole('button', { name: /Delete quiz/ }))
    expect(await screen.findByRole('alert')).toHaveTextContent('quiz not found')
  })
})
