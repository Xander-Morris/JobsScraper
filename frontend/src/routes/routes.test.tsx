import { QueryClientProvider } from '@tanstack/react-query'
import { createMemoryHistory, createRouter, RouterProvider } from '@tanstack/react-router'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it, vi } from 'vitest'
import { routeTree } from '@/src/routeTree.gen'
import { auth, signIn } from '@/src/test/auth-mock'
import {
  makeExtraction,
  makeJob,
  makeJobListItem,
  makeProfile,
  makeQuiz,
  makeQuizSummary,
  makeResume,
  makeTailoredResume
} from '@/src/test/fixtures'
import { createTestQueryClient } from '@/src/test/render'
import { api, server } from '@/src/test/server'

vi.mock('@/src/stores/auth-store', () => import('@/src/test/auth-mock'))
vi.mock('@tanstack/react-router-devtools', () => ({ TanStackRouterDevtools: () => null }))
vi.mock('@mapbox/search-js-react', () => ({
  AddressAutofill: ({ children }: { children: React.ReactNode }) => children
}))

// renderApp mounts the real route tree at path, like main.tsx does.
async function renderApp(path: string) {
  const queryClient = createTestQueryClient()
  const router = createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: [path] }),
    context: { queryClient, auth }
  })

  await router.load()

  render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} context={{ auth }} />
    </QueryClientProvider>
  )

  return { router, queryClient }
}

const pathname = (router: Awaited<ReturnType<typeof renderApp>>['router']) => router.state.location.pathname

function serveProfile(profile = makeProfile({ resumes: [makeResume({ id: 10 })] })) {
  server.use(
    http.get(api('/api/profile'), () => HttpResponse.json(profile)),
    http.get(api('/api/profile/resumes/10/extraction'), () => HttpResponse.json(makeExtraction())),
    http.get(api('/api/trivia'), () => HttpResponse.json([makeQuizSummary()])),
    http.get(api('/api/jobs/:id/tailored-resume'), () => HttpResponse.json({ error: 'none' }, { status: 404 }))
  )
}

describe('redirects', () => {
  it('sends / to the profile basics', async () => {
    const { router } = await renderApp('/')

    await waitFor(() => expect(pathname(router)).toBe('/profile/basic'))
  })

  it('shows the log in forms on gated pages when signed out', async () => {
    await renderApp('/profile')
    expect(await screen.findByRole('group', { name: 'Authentication mode' })).toBeInTheDocument()
  })

  it.each(['/trivia', '/jobs'])('gates %s behind log in', async (path) => {
    await renderApp(path)
    expect(await screen.findByRole('group', { name: 'Authentication mode' })).toBeInTheDocument()
  })

  it('shows skeletons while auth initializes', async () => {
    auth.isInitializing = true
    await renderApp('/jobs')

    expect(screen.getByRole('heading', { name: 'Jobs' })).toBeInTheDocument()
    expect(screen.queryByRole('group', { name: 'Authentication mode' })).not.toBeInTheDocument()
  })
})

describe('profile pages', () => {
  it('renders the profile with section tabs', async () => {
    signIn()
    serveProfile(makeProfile({ email: 'ada@example.com', name: 'Ada' }))
    const { router } = await renderApp('/profile')

    await waitFor(() => expect(pathname(router)).toBe('/profile/basic'))
    expect(await screen.findByText('ada@example.com')).toBeInTheDocument()
    expect(screen.getByRole('textbox', { name: 'Name' })).toHaveValue('Ada')

    const tabs = screen.getByRole('navigation', { name: 'Profile sections' })
    await userEvent.click(within(tabs).getByRole('link', { name: 'Skills' }))
    expect(await screen.findByRole('textbox', { name: 'Add a skill' })).toBeInTheDocument()

    await userEvent.click(within(tabs).getByRole('link', { name: 'Education' }))
    expect(await screen.findByRole('textbox', { name: 'School' })).toBeInTheDocument()

    await userEvent.click(within(tabs).getByRole('link', { name: 'Work experience' }))
    expect(await screen.findByRole('textbox', { name: 'Company' })).toBeInTheDocument()

    await userEvent.click(within(tabs).getByRole('link', { name: 'Resumes' }))
    expect(await screen.findByText('No resumes uploaded yet.')).toBeInTheDocument()
  })

  it('signs out when the profile is gone', async () => {
    signIn()
    server.use(http.get(api('/api/profile'), () => HttpResponse.json({ error: 'profile not found' }, { status: 404 })))
    await renderApp('/profile/basic')

    expect(await screen.findByText('Signing you out…')).toBeInTheDocument()
    expect(auth.logout).toHaveBeenCalledWith('Your session expired or is no longer valid. Please log in again.')
  })

  it('reports other profile errors', async () => {
    signIn()
    server.use(http.get(api('/api/profile'), () => HttpResponse.json({ error: 'boom' }, { status: 500 })))
    await renderApp('/profile/basic')

    // Two retries with backoff come first.
    expect(
      await screen.findByText('Unable to load your profile. Please try again.', {}, { timeout: 6000 })
    ).toBeInTheDocument()
  }, 10_000)

  it('logs out from the profile header', async () => {
    signIn()
    serveProfile()
    await renderApp('/profile/basic')

    const buttons = await screen.findAllByRole('button', { name: 'Log out' })
    await userEvent.click(buttons.at(-1)!)
    expect(auth.logout).toHaveBeenCalled()
  })
})

describe('jobs pages', () => {
  function serveJobs(total = 45) {
    const seen: URLSearchParams[] = []
    server.use(
      http.get(api('/api/jobs'), ({ request }) => {
        const params = new URL(request.url).searchParams
        seen.push(params)
        const offset = Number(params.get('offset') ?? 0)
        const jobs = total === 0 ? [] : [makeJobListItem({ id: offset + 1, title: `Job ${offset + 1}` })]
        return HttpResponse.json({ jobs, total, limit: 20, offset })
      }),
      http.get(api('/api/jobs/:id'), ({ params }) =>
        HttpResponse.json(makeJob({ id: Number(params.id), title: `Job ${params.id}` }))
      )
    )
    return seen
  }

  it('lists jobs, paginates and filters through the URL', async () => {
    signIn()
    serveProfile()
    const seen = serveJobs()
    const { router } = await renderApp('/jobs')

    expect(await screen.findByRole('heading', { name: 'Job 1' })).toBeInTheDocument()
    expect(screen.getByText('45 results')).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Next' }))
    expect(await screen.findByRole('heading', { name: 'Job 21' })).toBeInTheDocument()
    expect(router.state.location.search).toMatchObject({ page: 2 })
    expect(seen.some((p) => p.get('offset') === '20' && p.get('limit') === '20')).toBe(true)

    await userEvent.click(screen.getByRole('button', { name: /Any workplace/ }))
    await userEvent.click(await screen.findByRole('menuitem', { name: 'Remote' }))

    await waitFor(() => expect(router.state.location.search).toMatchObject({ workplaceType: 'remote', page: 1 }))
    await waitFor(() => expect(seen.at(-1)?.get('workplace_type')).toBe('remote'))
    expect(JSON.parse(sessionStorage.getItem('jobs-last-search')!)).toMatchObject({ workplaceType: 'remote' })
  })

  it('prefetches the next page', async () => {
    signIn()
    const seen = serveJobs()
    await renderApp('/jobs')

    await screen.findByRole('heading', { name: 'Job 1' })
    await waitFor(() => expect(seen.some((p) => p.get('offset') === '20')).toBe(true))
  })

  it('shows the empty state and errors', async () => {
    signIn()
    serveJobs(0)
    await renderApp('/jobs?q=nothing')

    expect(await screen.findByText('No jobs match those filters')).toBeInTheDocument()
    expect(screen.getByText('0 results')).toBeInTheDocument()
  })

  it('shows a search error', async () => {
    signIn()
    server.use(http.get(api('/api/jobs'), () => HttpResponse.json({ error: 'failed to search jobs' }, { status: 500 })))
    await renderApp('/jobs')

    expect(await screen.findByRole('alert')).toHaveTextContent('failed to search jobs')
  })

  it('opens a job from the list and goes back to the same search', async () => {
    signIn()
    serveProfile()
    serveJobs()
    const { router } = await renderApp('/jobs?q=go')

    await userEvent.click(await screen.findByRole('link', { name: /Job 1/ }))
    await waitFor(() => expect(pathname(router)).toBe('/jobs/1'))
    expect(await screen.findByRole('heading', { level: 1, name: 'Job 1' })).toBeInTheDocument()
    expect(await screen.findByRole('heading', { name: 'Apply assist' })).toBeInTheDocument()

    await userEvent.click(screen.getByRole('link', { name: 'Back to jobs' }))
    await waitFor(() => expect(pathname(router)).toBe('/jobs'))
    expect(router.state.location.search).toMatchObject({ q: 'go' })
  })

  it('shows a job to anonymous visitors without apply assist', async () => {
    server.use(
      http.get(api('/api/jobs/7'), () => HttpResponse.json(makeJob({ id: 7, match_percent: 90, applied: true })))
    )
    await renderApp('/jobs/7')

    expect(await screen.findByRole('heading', { level: 1, name: 'Backend Engineer' })).toBeInTheDocument()
    expect(screen.getByText('90% · Strong match')).toBeInTheDocument()
    expect(screen.getByText('Applied')).toBeInTheDocument()
    expect(screen.getByText('Build Go services on Postgres.')).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: 'Apply assist' })).not.toBeInTheDocument()
    // Direct visits link back to the last search instead of history.
    expect(screen.getByRole('link', { name: 'Back to jobs' })).toHaveAttribute('href', '/jobs')
  })

  it('shows a missing job', async () => {
    server.use(http.get(api('/api/jobs/404'), () => HttpResponse.json({ error: 'job not found' }, { status: 404 })))
    await renderApp('/jobs/404')

    expect(await screen.findByRole('alert')).toHaveTextContent('job not found')
  })
})

describe('tailored resume page', () => {
  function serveTailored(tailored: unknown = makeTailoredResume()) {
    const saves: unknown[] = []
    let regenerations = 0
    serveProfile()
    server.use(
      http.get(api('/api/jobs/1'), () => HttpResponse.json(makeJob({ id: 1 }))),
      http.get(api('/api/jobs/1/tailored-resume'), () =>
        tailored ? HttpResponse.json(tailored) : HttpResponse.json({ error: 'none' }, { status: 404 })
      ),
      http.put(api('/api/jobs/1/tailored-resume'), async ({ request }) => {
        const content = await request.json()
        saves.push(content)
        return HttpResponse.json({ ...makeTailoredResume(), content })
      }),
      http.post(api('/api/jobs/1/tailored-resume'), () => {
        regenerations++
        return HttpResponse.json(makeTailoredResume())
      })
    )
    return { saves, regenerations: () => regenerations }
  }

  it('edits, saves, prints and regenerates', async () => {
    signIn()
    const { saves, regenerations } = serveTailored()
    await renderApp('/jobs/1/resume')

    expect(await screen.findByText('Backend Engineer at Acme')).toBeInTheDocument()
    const save = await screen.findByRole('button', { name: /Save/ })
    expect(save).toBeDisabled()

    await userEvent.type(screen.getByLabelText('Summary'), '!')
    expect(screen.getByText('Unsaved changes')).toBeInTheDocument()
    expect(
      within(screen.getByRole('article', { name: 'Resume preview' })).getByText('Go engineer!')
    ).toBeInTheDocument()

    await userEvent.click(save)
    await waitFor(() => expect(saves).toHaveLength(1))
    expect(saves[0]).toMatchObject({ summary: 'Go engineer!' })
    await waitFor(() => expect(screen.queryByText('Unsaved changes')).not.toBeInTheDocument())

    const title = document.title
    await userEvent.click(screen.getByRole('button', { name: /Download PDF/ }))
    expect(window.print).toHaveBeenCalledOnce()
    expect(document.title).toBe(title)

    vi.mocked(window.confirm).mockReturnValueOnce(false)
    await userEvent.click(screen.getByRole('button', { name: /Regenerate/ }))
    expect(regenerations()).toBe(0)

    await userEvent.click(screen.getByRole('button', { name: /Regenerate/ }))
    await waitFor(() => expect(regenerations()).toBe(1))
  })

  it('asks before leaving with unsaved changes', async () => {
    signIn()
    serveTailored()
    const { router } = await renderApp('/jobs/1/resume')

    await userEvent.type(await screen.findByLabelText('Summary'), '!')

    vi.mocked(window.confirm).mockReturnValueOnce(false)
    await userEvent.click(screen.getByRole('link', { name: 'Back to job' }))
    expect(window.confirm).toHaveBeenCalledWith('Discard unsaved changes?')
    expect(pathname(router)).toBe('/jobs/1/resume')

    await userEvent.click(screen.getByRole('link', { name: 'Back to job' }))
    await waitFor(() => expect(pathname(router)).toBe('/jobs/1'))
  })

  it('warns about a stale resume and shows the empty state', async () => {
    signIn()
    serveTailored(makeTailoredResume({ stale: true }))
    await renderApp('/jobs/1/resume')

    expect(await screen.findByRole('status')).toHaveTextContent('Your resume changed since this was tailored.')
  })

  it('explains when there is nothing tailored yet', async () => {
    signIn()
    serveTailored(null)
    await renderApp('/jobs/1/resume')

    expect(await screen.findByText(/No tailored resume for this job yet/)).toBeInTheDocument()
  })

  it('asks anonymous visitors to log in', async () => {
    await renderApp('/jobs/1/resume')

    expect(await screen.findByText('Log in to view your tailored resume.')).toBeInTheDocument()
  })
})

describe('reset password page', () => {
  it('explains an incomplete link', async () => {
    await renderApp('/reset-password')

    expect(await screen.findByRole('alert')).toHaveTextContent('This reset link is incomplete.')
  })

  it('validates and resets the password', async () => {
    let body: unknown
    server.use(
      http.post(api('/api/profile/password-reset/confirm'), async ({ request }) => {
        body = await request.json()
        return HttpResponse.json({ token: 'fresh' })
      })
    )
    const { router } = await renderApp('/reset-password?token=abc')

    await userEvent.type(await screen.findByLabelText('New password'), 'longenough')
    await userEvent.type(screen.getByLabelText('Confirm new password'), 'different')
    await userEvent.click(screen.getByRole('button', { name: 'Reset password' }))
    expect(await screen.findByText("Passwords don't match")).toBeInTheDocument()

    await userEvent.clear(screen.getByLabelText('Confirm new password'))
    await userEvent.type(screen.getByLabelText('Confirm new password'), 'longenough')
    await userEvent.click(screen.getByRole('button', { name: 'Reset password' }))

    await waitFor(() => expect(auth.login).toHaveBeenCalledWith('fresh', true))
    expect(body).toEqual({ token: 'abc', password: 'longenough' })
    await waitFor(() => expect(pathname(router)).toMatch(/^\/profile/))
  })

  it('shows an expired link error', async () => {
    server.use(
      http.post(api('/api/profile/password-reset/confirm'), () =>
        HttpResponse.json({ error: 'reset link is invalid or expired' }, { status: 400 })
      )
    )
    await renderApp('/reset-password?token=old')

    await userEvent.type(await screen.findByLabelText('New password'), 'longenough')
    await userEvent.type(screen.getByLabelText('Confirm new password'), 'longenough')
    await userEvent.click(screen.getByRole('button', { name: 'Reset password' }))

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('reset link is invalid or expired')
    expect(within(alert).getByRole('link', { name: 'Request a new link' })).toHaveAttribute('href', '/profile')
  })
})

describe('trivia pages', () => {
  it('shows the generator and history', async () => {
    signIn()
    serveProfile()
    await renderApp('/trivia')

    expect(await screen.findByRole('heading', { name: 'Trivia' })).toBeInTheDocument()
    expect(await screen.findByRole('heading', { name: 'New quiz' })).toBeInTheDocument()
    expect(await screen.findByRole('link', { name: /Go/ })).toHaveAttribute('href', '/trivia/5')
  })

  it('plays a quiz', async () => {
    signIn()
    server.use(
      http.get(api('/api/trivia/5'), () => HttpResponse.json(makeQuiz({ topic: 'Go basics', difficulty: 'hard' })))
    )
    await renderApp('/trivia/5')

    expect(await screen.findByRole('heading', { name: 'Go basics' })).toBeInTheDocument()
    expect(screen.getByText('hard')).toBeInTheDocument()
    expect(screen.getByText('Question 1 / 2')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Back to trivia' })).toHaveAttribute('href', '/trivia')
  })

  it('shows a missing quiz', async () => {
    signIn()
    server.use(http.get(api('/api/trivia/9'), () => HttpResponse.json({ error: 'quiz not found' }, { status: 404 })))
    await renderApp('/trivia/9')

    expect(await screen.findByRole('alert')).toHaveTextContent('quiz not found')
  })
})
