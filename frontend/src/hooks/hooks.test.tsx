import { act, waitFor } from '@testing-library/react'
import { http, HttpResponse } from 'msw'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '@/src/api/client'
import { queryKeys } from '@/src/api/query-keys'
import { signIn } from '@/src/test/auth-mock'
import {
  makeExtraction,
  makeJob,
  makeProfile,
  makeQuiz,
  makeQuizSummary,
  makeTailoredResume
} from '@/src/test/fixtures'
import { renderHookWithClient } from '@/src/test/render'
import { api, server } from '@/src/test/server'
import useCopied from './use-copied'
import * as jobsHooks from './use-jobs'
import * as profileHooks from './use-profile'
import { useTagsQuery } from './use-tags'
import * as triviaHooks from './use-trivia'

vi.mock('@/src/stores/auth-store', () => import('@/src/test/auth-mock'))

describe('useCopied', () => {
  beforeEach(() => {
    Object.defineProperty(navigator, 'clipboard', {
      value: { writeText: vi.fn().mockResolvedValue(undefined) },
      configurable: true
    })
  })

  it('copies text and resets after two seconds', async () => {
    vi.useFakeTimers()
    const { result } = renderHookWithClient(() => useCopied())

    await act(() => result.current.copy('hello'))

    expect(navigator.clipboard.writeText).toHaveBeenCalledWith('hello')
    expect(result.current.copied).toBe(true)

    await act(() => vi.advanceTimersByTimeAsync(1999))
    expect(result.current.copied).toBe(true)

    await act(() => vi.advanceTimersByTimeAsync(1))
    expect(result.current.copied).toBe(false)
  })

  it('restarts the timer on a second copy', async () => {
    vi.useFakeTimers()
    const { result } = renderHookWithClient(() => useCopied())

    await act(() => result.current.copy('a'))
    await act(() => vi.advanceTimersByTimeAsync(1500))
    await act(() => result.current.copy('b'))
    await act(() => vi.advanceTimersByTimeAsync(1500))

    expect(result.current.copied).toBe(true)
  })
})

describe('job hooks', () => {
  it('keys job queries on auth state', async () => {
    server.use(http.get(api('/api/jobs/1'), () => HttpResponse.json(makeJob())))

    const anon = renderHookWithClient(() => jobsHooks.useJobQuery(1))
    await waitFor(() => expect(anon.result.current.isSuccess).toBe(true))
    expect(anon.client.getQueryData(queryKeys.job(1, false))).toBeDefined()

    signIn()
    const authed = renderHookWithClient(() => jobsHooks.useJobQuery(1))
    await waitFor(() => expect(authed.result.current.isSuccess).toBe(true))
    expect(authed.client.getQueryData(queryKeys.job(1, true))).toBeDefined()
  })

  it('does not fetch a job with a bad id', () => {
    const { result } = renderHookWithClient(() => jobsHooks.useJobQuery(Number.NaN))

    expect(result.current.fetchStatus).toBe('idle')
  })

  it('searches jobs', async () => {
    server.use(http.get(api('/api/jobs'), () => HttpResponse.json({ jobs: [], total: 0, limit: 20, offset: 0 })))

    const { result } = renderHookWithClient(() => jobsHooks.useJobsQuery({ q: 'go' }))
    await waitFor(() => expect(result.current.data?.total).toBe(0))

    const disabled = renderHookWithClient(() => jobsHooks.useJobsQuery({}, { enabled: false }))
    expect(disabled.result.current.fetchStatus).toBe('idle')
  })

  it('invalidates job queries after marking applied', async () => {
    server.use(
      http.post(api('/api/jobs/1/apply'), () => HttpResponse.json({ status: 'applied' })),
      http.delete(api('/api/jobs/1/apply'), () => HttpResponse.json({ status: 'unapplied' }))
    )

    const { result, client } = renderHookWithClient(() => ({
      mark: jobsHooks.useMarkJobAppliedMutation(),
      unmark: jobsHooks.useUnmarkJobAppliedMutation()
    }))
    const invalidate = vi.spyOn(client, 'invalidateQueries')

    await act(() => result.current.mark.mutateAsync(1))
    await act(() => result.current.unmark.mutateAsync(1))

    expect(invalidate).toHaveBeenCalledTimes(2)
    expect(invalidate).toHaveBeenCalledWith({ queryKey: queryKeys.jobs })
  })

  it('caches generated and saved tailored resumes', async () => {
    signIn()
    const tailored = makeTailoredResume()
    server.use(
      http.post(api('/api/jobs/1/tailored-resume'), () => HttpResponse.json(tailored)),
      http.put(api('/api/jobs/1/tailored-resume'), () => HttpResponse.json({ ...tailored, stale: true }))
    )

    const { result, client } = renderHookWithClient(() => ({
      generate: jobsHooks.useGenerateTailoredResumeMutation(),
      save: jobsHooks.useSaveTailoredResumeMutation(1)
    }))

    await act(() => result.current.generate.mutateAsync(1))
    expect(client.getQueryData(queryKeys.tailoredResume(1))).toMatchObject({ stale: false })

    await act(() => result.current.save.mutateAsync(tailored.content))
    expect(client.getQueryData(queryKeys.tailoredResume(1))).toMatchObject({ stale: true })
  })

  it('only loads a tailored resume when signed in', async () => {
    server.use(
      http.get(api('/api/jobs/1/tailored-resume'), () => HttpResponse.json({ error: 'none' }, { status: 404 }))
    )

    const anon = renderHookWithClient(() => jobsHooks.useTailoredResumeQuery(1))
    expect(anon.result.current.fetchStatus).toBe('idle')

    signIn()
    const authed = renderHookWithClient(() => jobsHooks.useTailoredResumeQuery(1))
    await waitFor(() => expect(authed.result.current.isSuccess).toBe(true))
    expect(authed.result.current.data).toBeNull()
  })

  it('generates application content', async () => {
    server.use(
      http.post(api('/api/jobs/1/generate'), () => HttpResponse.json({ cover_letter: 'Hi', tailored_bullets: ['x'] }))
    )

    const { result } = renderHookWithClient(() => jobsHooks.useGenerateApplicationContentMutation())
    await act(() => result.current.mutateAsync(1))

    await waitFor(() => expect(result.current.data?.cover_letter).toBe('Hi'))
  })
})

describe('profile hooks', () => {
  it('retries the profile query except on 401 and 404', () => {
    const retry = profileHooks.profileQueryOptions.retry as (count: number, error: Error) => boolean

    expect(retry(0, new ApiError('x', 401))).toBe(false)
    expect(retry(0, new ApiError('x', 404))).toBe(false)
    expect(retry(0, new ApiError('x', 500))).toBe(true)
    expect(retry(1, new Error('network'))).toBe(true)
    expect(retry(2, new Error('network'))).toBe(false)
  })

  it('only loads the profile when signed in', async () => {
    server.use(http.get(api('/api/profile'), () => HttpResponse.json(makeProfile())))

    const anon = renderHookWithClient(() => profileHooks.useProfileQuery())
    expect(anon.result.current.fetchStatus).toBe('idle')

    signIn()
    const authed = renderHookWithClient(() => profileHooks.useProfileQuery())
    await waitFor(() => expect(authed.result.current.data?.email).toBe('ada@example.com'))
  })

  it('patches the cached profile after an update', async () => {
    server.use(http.put(api('/api/profile'), () => HttpResponse.json({ status: 'updated' })))

    const { result, client } = renderHookWithClient(() => profileHooks.useUpdateProfileMutation())
    client.setQueryData(queryKeys.profile, makeProfile({ name: 'Old' }))
    const invalidate = vi.spyOn(client, 'invalidateQueries')

    await act(() =>
      result.current.mutateAsync({
        name: 'New',
        address: '',
        linked_in: '',
        github: '',
        portfolio: '',
        email_notifications: true
      })
    )

    expect(client.getQueryData(queryKeys.profile)).toMatchObject({
      name: 'New',
      email_notifications: true,
      email: 'ada@example.com'
    })
    expect(invalidate).not.toHaveBeenCalled()
  })

  it('refetches the profile after other mutations', async () => {
    server.use(
      http.post(api('/api/profile/skills'), () => HttpResponse.json({ id: 1 })),
      http.delete(api('/api/profile/skills/1'), () => HttpResponse.json({ status: 'deleted' }))
    )

    const { result, client } = renderHookWithClient(() => ({
      add: profileHooks.useAddSkillMutation(),
      del: profileHooks.useDeleteSkillMutation()
    }))
    const invalidate = vi.spyOn(client, 'invalidateQueries')

    await act(() => result.current.add.mutateAsync({ skill: 'Go' }))
    await act(() => result.current.del.mutateAsync(1))

    expect(invalidate).toHaveBeenCalledTimes(2)
    expect(invalidate).toHaveBeenCalledWith({ queryKey: queryKeys.profile })
  })

  it('logs in with remember me defaulting to false', async () => {
    let body: unknown
    server.use(
      http.post(api('/api/profile/login'), async ({ request }) => {
        body = await request.json()
        return HttpResponse.json({ token: 't' })
      })
    )

    const { result } = renderHookWithClient(() => profileHooks.useLoginMutation())
    await act(() => result.current.mutateAsync({ email: 'a@b.co', password: 'pw' }))

    expect(body).toEqual({ email: 'a@b.co', password: 'pw', remember_me: false })
  })

  it('polls a pending extraction until it completes', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    signIn()

    let calls = 0
    server.use(
      http.get(api('/api/profile/resumes/10/extraction'), () => {
        calls++
        return HttpResponse.json(makeExtraction({ status: calls < 2 ? 'pending' : 'completed' }))
      })
    )

    const { result } = renderHookWithClient(() => profileHooks.useResumeExtractionQuery(10, { enabled: true }))

    await waitFor(() => expect(result.current.data?.status).toBe('pending'))
    await act(() => vi.advanceTimersByTimeAsync(2000))
    await waitFor(() => expect(result.current.data?.status).toBe('completed'))

    await act(() => vi.advanceTimersByTimeAsync(10_000))
    expect(calls).toBe(2)
  })

  it('invalidates the extraction after triggering it', async () => {
    server.use(http.post(api('/api/profile/resumes/10/extraction'), () => HttpResponse.json({ status: 'completed' })))

    const { result, client } = renderHookWithClient(() => profileHooks.useTriggerResumeExtractionMutation())
    const invalidate = vi.spyOn(client, 'invalidateQueries')

    await act(() => result.current.mutateAsync(10))

    expect(invalidate).toHaveBeenCalledWith({ queryKey: queryKeys.resumeExtraction(10) })
  })
})

describe('trivia hooks', () => {
  it('stores a generated quiz and refreshes the list', async () => {
    signIn()
    const quiz = makeQuiz({ id: 9 })
    server.use(
      http.post(api('/api/trivia'), () => HttpResponse.json(quiz)),
      http.get(api('/api/trivia'), () => HttpResponse.json([makeQuizSummary({ id: 9 })]))
    )

    const { result, client } = renderHookWithClient(() => ({
      list: triviaHooks.useTriviaQuizzesQuery(),
      generate: triviaHooks.useGenerateTriviaMutation()
    }))
    await waitFor(() => expect(result.current.list.isSuccess).toBe(true))
    const invalidate = vi.spyOn(client, 'invalidateQueries')

    await act(() =>
      result.current.generate.mutateAsync({ source: 'prompt', prompt: 'Go', count: 5, difficulty: 'easy' })
    )

    expect(client.getQueryData(queryKeys.triviaQuiz(9))).toMatchObject({ id: 9 })
    expect(invalidate).toHaveBeenCalledWith({ queryKey: queryKeys.trivia, exact: true })
  })

  it('removes a deleted quiz from the cache', async () => {
    server.use(http.delete(api('/api/trivia/9'), () => HttpResponse.json({ status: 'deleted' })))

    const { result, client } = renderHookWithClient(() => triviaHooks.useDeleteTriviaMutation())
    client.setQueryData(queryKeys.triviaQuiz(9), makeQuiz({ id: 9 }))

    await act(() => result.current.mutateAsync(9))

    expect(client.getQueryData(queryKeys.triviaQuiz(9))).toBeUndefined()
  })

  it('stores saved answers and retakes', async () => {
    server.use(
      http.put(api('/api/trivia/5/answers'), () => HttpResponse.json(makeQuiz({ answers: [1] }))),
      http.post(api('/api/trivia/5/retake'), () => HttpResponse.json(makeQuiz({ id: 6 })))
    )

    const { result, client } = renderHookWithClient(() => ({
      save: triviaHooks.useSaveTriviaAnswersMutation(),
      retake: triviaHooks.useRetakeTriviaMutation()
    }))

    await act(() => result.current.save.mutateAsync({ id: 5, answers: [1] }))
    await act(() => result.current.retake.mutateAsync(5))

    expect(client.getQueryData(queryKeys.triviaQuiz(5))).toMatchObject({ answers: [1] })
    expect(client.getQueryData(queryKeys.triviaQuiz(6))).toMatchObject({ id: 6 })
  })

  it('waits for sign-in before loading a quiz', () => {
    const { result } = renderHookWithClient(() => triviaHooks.useTriviaQuizQuery(5))

    expect(result.current.fetchStatus).toBe('idle')
  })
})

describe('useTagsQuery', () => {
  it('loads tags', async () => {
    const { result } = renderHookWithClient(() => useTagsQuery())

    await waitFor(() => expect(result.current.data).toEqual(['go', 'react', 'sql']))
  })
})
