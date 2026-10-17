import { http, HttpResponse, type JsonBodyType } from 'msw'
import { describe, expect, it, vi } from 'vitest'
import {
  makeJob,
  makeJobListItem,
  makeProfile,
  makeQuiz,
  makeQuizSummary,
  makeTailoredResume
} from '@/src/test/fixtures'
import { api, server } from '@/src/test/server'
import {
  fetchJob,
  fetchJobs,
  fetchTailoredResume,
  generateApplicationContent,
  generateTailoredResume,
  markJobApplied,
  saveTailoredResume,
  unmarkJobApplied
} from './jobs'
import * as profile from './profile'
import { fetchTags } from './tags'
import * as trivia from './trivia'

interface Captured {
  method: string
  url: URL
  body: unknown
}

// capture answers any request with reply and records what was sent.
function capture(reply: JsonBodyType, status = 200) {
  const calls: Captured[] = []

  server.use(
    http.all(api('/api/*'), async ({ request }) => {
      const type = request.headers.get('Content-Type') ?? ''
      let body: unknown = null
      if (type.includes('application/json')) body = await request.json()
      else if (type.includes('multipart/form-data')) body = await request.formData()
      calls.push({ method: request.method, url: new URL(request.url), body })
      return HttpResponse.json(reply, { status })
    })
  )

  return calls
}

describe('jobs api', () => {
  it('maps camelCase params onto the snake_case query', async () => {
    const calls = capture({ jobs: [makeJobListItem()], total: 1, limit: 20, offset: 40 })

    await fetchJobs({
      q: 'go dev',
      workplaceType: 'remote',
      jobType: 'intern',
      minSalary: 0,
      maxSalary: 200000,
      datePosted: 'week',
      tags: ['go', 'sql'],
      sort: 'date',
      limit: 20,
      offset: 40
    })

    expect(calls[0].url.pathname).toBe('/api/jobs')
    expect(Object.fromEntries(calls[0].url.searchParams)).toEqual({
      q: 'go dev',
      workplace_type: 'remote',
      job_type: 'intern',
      min_salary: '0',
      max_salary: '200000',
      date_posted: 'week',
      tags: 'go,sql',
      sort: 'date',
      limit: '20',
      offset: '40'
    })
  })

  it('sends no query string for empty params', async () => {
    const calls = capture({ jobs: [], total: 0, limit: 20, offset: 0 })

    await fetchJobs()
    await fetchJobs({ tags: [] })

    expect(calls.map((c) => c.url.search)).toEqual(['', ''])
  })

  it('decodes html entities in job text', async () => {
    capture(makeJob({ title: '  R&amp;D Engineer ', company: 'Smith &amp; Co' }))

    await expect(fetchJob(1)).resolves.toMatchObject({ title: 'R&D Engineer', company: 'Smith & Co', applied: false })
  })

  it('hits the apply and generate endpoints', async () => {
    const calls = capture({ status: 'applied' })

    await markJobApplied(3)
    await unmarkJobApplied(3)

    expect(calls.map((c) => `${c.method} ${c.url.pathname}`)).toEqual([
      'POST /api/jobs/3/apply',
      'DELETE /api/jobs/3/apply'
    ])

    capture({ cover_letter: 'Hi', tailored_bullets: [] })
    await expect(generateApplicationContent(3)).resolves.toEqual({ cover_letter: 'Hi', tailored_bullets: [] })
  })

  it('treats a missing tailored resume as null but rethrows other errors', async () => {
    capture({ error: 'no tailored resume for this job' }, 404)
    await expect(fetchTailoredResume(1)).resolves.toBeNull()

    capture({ error: 'boom' }, 500)
    await expect(fetchTailoredResume(1)).rejects.toMatchObject({ status: 500 })
  })

  it('generates and saves tailored resumes', async () => {
    const tailored = makeTailoredResume()
    const calls = capture(tailored)

    await generateTailoredResume(1)
    await saveTailoredResume(1, tailored.content)

    expect(calls[0].method).toBe('POST')
    expect(calls[1]).toMatchObject({ method: 'PUT', body: tailored.content })
  })

  it('fills nullish tailored lists with empty arrays', async () => {
    const tailored = makeTailoredResume()
    capture({ ...tailored, content: { ...tailored.content, skills: null, projects: undefined } })

    const got = await generateTailoredResume(1)
    expect(got.content.skills).toEqual([])
    expect(got.content.projects).toEqual([])
  })
})

describe('profile api', () => {
  it('sends credentials for auth calls', async () => {
    const calls = capture({ token: 't' })

    await profile.createProfile('a@b.co', 'pw')
    await profile.loginProfile('a@b.co', 'pw', true)
    await profile.confirmPasswordReset('tok', 'newpw')

    expect(calls.map((c) => c.body)).toEqual([
      { email: 'a@b.co', password: 'pw' },
      { email: 'a@b.co', password: 'pw', remember_me: true },
      { token: 'tok', password: 'newpw' }
    ])
  })

  it('requests a password reset', async () => {
    const calls = capture({ status: 'sent' })

    await profile.requestPasswordReset('a@b.co')
    expect(calls[0]).toMatchObject({ method: 'POST', body: { email: 'a@b.co' } })
    expect(calls[0].url.pathname).toBe('/api/profile/password-reset')
  })

  it('parses the profile', async () => {
    capture(makeProfile({ name: '  Ada  ' }))

    await expect(profile.fetchProfile()).resolves.toMatchObject({ name: 'Ada' })
  })

  it('routes every profile mutation to its endpoint', async () => {
    const calls = capture({ id: 1, status: 'ok' })

    const edu = { school_name: 'MIT', major: 'CS', degree: 'BS' }
    const we = { company: 'Acme', job_title: 'Dev', job_type: 'full_time' }

    await profile.updateProfile({
      name: 'A',
      address: '',
      linked_in: '',
      github: '',
      portfolio: '',
      email_notifications: true
    })
    await profile.addEducation(edu)
    await profile.updateEducation(2, edu)
    await profile.deleteEducation(2)
    await profile.addSkill({ skill: 'Go' })
    await profile.deleteSkill(3)
    await profile.addWorkExperience(we)
    await profile.updateWorkExperience(4, we)
    await profile.deleteWorkExperience(4)
    await profile.addWorkExperienceBullet(4, { bullet: 'Did it' })
    await profile.updateWorkExperienceBullet(4, 5, 'Did more')
    await profile.deleteWorkExperienceBullet(4, 5)
    await profile.deleteResume(6)
    await profile.activateResume(6)
    await profile.triggerResumeExtraction(6)

    expect(calls.map((c) => `${c.method} ${c.url.pathname}`)).toEqual([
      'PUT /api/profile',
      'POST /api/profile/education',
      'PUT /api/profile/education/2',
      'DELETE /api/profile/education/2',
      'POST /api/profile/skills',
      'DELETE /api/profile/skills/3',
      'POST /api/profile/work-experience',
      'PUT /api/profile/work-experience/4',
      'DELETE /api/profile/work-experience/4',
      'POST /api/profile/work-experience/4/bullets',
      'PUT /api/profile/work-experience/4/bullets/5',
      'DELETE /api/profile/work-experience/4/bullets/5',
      'DELETE /api/profile/resumes/6',
      'POST /api/profile/resumes/6/activate',
      'POST /api/profile/resumes/6/extraction'
    ])
    expect(calls[10].body).toEqual({ bullet: 'Did more' })
  })

  it('uploads resumes as multipart form data', async () => {
    capture({ id: 7, status: 'updated' })
    // Inspect the FormData handed to fetch; jsdom's File loses its name crossing into undici.
    const fetchSpy = vi.spyOn(globalThis, 'fetch')
    const file = new File(['pdf'], 'cv.pdf', { type: 'application/pdf' })

    await profile.uploadResume(file)
    await profile.updateResume(7, file)
    await profile.updateResume(7, 'renamed.pdf')

    const [upload, replace, rename] = fetchSpy.mock.calls.map(([, init]) => init?.body as FormData)
    expect((upload.get('resume') as File).name).toBe('cv.pdf')
    expect((replace.get('resume') as File).name).toBe('cv.pdf')
    expect(rename.get('file_name')).toBe('renamed.pdf')
    expect(rename.get('resume')).toBeNull()
  })

  it('downloads a resume as a blob', async () => {
    server.use(http.get(api('/api/profile/resumes/7/download'), () => new HttpResponse('bytes')))

    const blob = await profile.downloadResume(7)
    expect(await blob.text()).toBe('bytes')
  })
})

describe('trivia and tags api', () => {
  it('generates, saves, retakes and deletes quizzes', async () => {
    const quiz = makeQuiz()
    const calls = capture(quiz)

    await trivia.generateTrivia({ source: 'prompt', prompt: 'Go', count: 5, difficulty: 'easy' })
    await trivia.fetchTriviaQuiz(5)
    await trivia.saveTriviaAnswers(5, [1, 0])
    await trivia.retakeTrivia(5)

    expect(calls.map((c) => `${c.method} ${c.url.pathname}`)).toEqual([
      'POST /api/trivia',
      'GET /api/trivia/5',
      'PUT /api/trivia/5/answers',
      'POST /api/trivia/5/retake'
    ])
    expect(calls[2].body).toEqual({ answers: [1, 0] })

    capture([makeQuizSummary()])
    await expect(trivia.fetchTriviaQuizzes()).resolves.toHaveLength(1)

    capture({ status: 'deleted' })
    await expect(trivia.deleteTrivia(5)).resolves.toEqual({ status: 'deleted' })
  })

  it('defaults missing quiz answers to an empty list', async () => {
    capture({ ...makeQuiz(), answers: null })

    await expect(trivia.fetchTriviaQuiz(5)).resolves.toMatchObject({ answers: [] })
  })

  it('fetches and trims tags', async () => {
    capture([' go ', 'sql'])

    await expect(fetchTags()).resolves.toEqual(['go', 'sql'])
  })
})
