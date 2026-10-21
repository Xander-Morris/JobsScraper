import { http, HttpResponse } from 'msw'
import { beforeEach, describe, expect, it } from 'vitest'
import { makeExtraction, makeProfile } from '@/src/test/fixtures'
import { api, server } from '@/src/test/server'
import { applyResumeExtractionToProfile } from '@/src/api/profile-merge'

interface Call {
  method: string
  path: string
  body: unknown
}

let calls: Call[]

beforeEach(() => {
  calls = []
  let nextId = 100

  server.use(
    http.all(api('/api/profile*'), async ({ request }) => {
      const body = request.method === 'DELETE' ? null : await request.json().catch(() => null)
      calls.push({ method: request.method, path: new URL(request.url).pathname, body })
      return HttpResponse.json({ id: nextId++, status: 'ok' })
    })
  )
})

const summary = (c: Call) => `${c.method} ${c.path}`

describe('applyResumeExtractionToProfile', () => {
  it('fills only empty basic info fields', async () => {
    const profile = makeProfile({
      name: 'Kept Name',
      linked_in: '',
      github: 'https://github.com/mine',
      email_notifications: true
    })
    const extraction = makeExtraction({ skills: [] })

    const result = await applyResumeExtractionToProfile(profile, extraction)

    expect(result.updatedBasicInfo).toBe(true)
    expect(calls).toHaveLength(1)
    expect(calls[0]).toMatchObject({
      method: 'PUT',
      path: '/api/profile',
      body: {
        name: 'Kept Name',
        linked_in: 'https://linkedin.com/in/ada',
        github: 'https://github.com/mine',
        portfolio: 'https://ada.dev',
        email_notifications: true
      }
    })
  })

  it('makes no calls when nothing would change', async () => {
    const profile = makeProfile({
      name: 'Ada',
      linked_in: 'x',
      github: 'y',
      portfolio: 'z',
      skills: [{ id: 1, skill: 'go' }]
    })

    const result = await applyResumeExtractionToProfile(profile, makeExtraction({ skills: [' Go '] }))

    expect(calls).toEqual([])
    expect(result).toEqual({
      updatedBasicInfo: false,
      addedEducation: 0,
      updatedEducation: 0,
      addedSkills: 0,
      addedWorkExperience: 0,
      updatedWorkExperience: 0
    })
  })

  it('adds new skills once, case-insensitively', async () => {
    const profile = makeProfile({
      name: 'A',
      linked_in: 'a',
      github: 'a',
      portfolio: 'a',
      skills: [{ id: 1, skill: 'React' }]
    })

    const result = await applyResumeExtractionToProfile(
      profile,
      makeExtraction({ skills: ['react', 'Go', 'go', '  ', 'SQL'] })
    )

    expect(result.addedSkills).toBe(2)
    expect(calls.map((c) => c.body)).toEqual([{ skill: 'Go' }, { skill: 'SQL' }])
  })

  it('adds new education and updates dates on matching entries', async () => {
    const profile = makeProfile({
      name: 'A',
      linked_in: 'a',
      github: 'a',
      portfolio: 'a',
      education: [
        { id: 7, school_name: 'MIT', major: 'CS', degree: 'BS', gpa: 3.9, start_date: '2018-09-01', end_date: null },
        { id: 8, school_name: 'Same', major: 'Math', degree: 'MS', gpa: null, start_date: '2020-01-01', end_date: null }
      ]
    })

    const result = await applyResumeExtractionToProfile(
      profile,
      makeExtraction({
        skills: [],
        education: [
          { school_name: ' mit ', major: 'cs', degree: 'bs', start_date: '2018-09-01', end_date: '2022-05-01' },
          { school_name: 'Same', major: 'Math', degree: 'MS', start_date: 'Fall 2020', end_date: '' },
          { school_name: 'Stanford', major: 'EE', degree: 'PhD', start_date: '2023-01-01', end_date: 'present' },
          { school_name: '', major: 'x', degree: 'y', start_date: '', end_date: '' }
        ]
      })
    )

    expect(result).toMatchObject({ addedEducation: 1, updatedEducation: 1 })
    expect(calls.map(summary)).toEqual(['PUT /api/profile/education/7', 'POST /api/profile/education'])
    expect(calls[0].body).toEqual({
      school_name: 'mit',
      major: 'cs',
      degree: 'bs',
      gpa: 3.9,
      start_date: '2018-09-01',
      end_date: '2022-05-01'
    })
    expect(calls[1].body).toEqual({ school_name: 'Stanford', major: 'EE', degree: 'PhD', start_date: '2023-01-01' })
  })

  it('adds new work experience with its unique bullets', async () => {
    const profile = makeProfile({ name: 'A', linked_in: 'a', github: 'a', portfolio: 'a' })

    const result = await applyResumeExtractionToProfile(
      profile,
      makeExtraction({
        skills: [],
        work_experience: [
          {
            company: 'Acme',
            job_title: 'Dev',
            location: 'NYC',
            start_date: '2021-01-01',
            end_date: 'Present',
            bullets: ['Shipped', ' Shipped ', '', 'Led']
          },
          { company: '', job_title: 'Ghost', location: '', start_date: '', end_date: '', bullets: null }
        ]
      })
    )

    expect(result.addedWorkExperience).toBe(1)
    expect(calls.map(summary)).toEqual([
      'POST /api/profile/work-experience',
      'POST /api/profile/work-experience/100/bullets',
      'POST /api/profile/work-experience/100/bullets'
    ])
    expect(calls[0].body).toEqual({
      company: 'Acme',
      job_title: 'Dev',
      job_type: 'unknown',
      location: 'NYC',
      start_date: '2021-01-01'
    })
    expect(calls.slice(1).map((c) => c.body)).toEqual([{ bullet: 'Shipped' }, { bullet: 'Led' }])
  })

  it('syncs bullets and fields on matching work experience', async () => {
    const profile = makeProfile({
      name: 'A',
      linked_in: 'a',
      github: 'a',
      portfolio: 'a',
      work_experience: [
        {
          id: 9,
          company: 'Acme',
          job_title: 'Dev',
          job_type: 'full_time',
          location: null,
          start_date: '2021-01-01',
          end_date: null,
          bullets: [
            { id: 1, bullet: 'Keep', position: 0 },
            { id: 2, bullet: 'Drop', position: 1 }
          ]
        }
      ]
    })

    const result = await applyResumeExtractionToProfile(
      profile,
      makeExtraction({
        skills: [],
        work_experience: [
          {
            company: 'ACME',
            job_title: 'dev',
            location: 'Remote',
            start_date: '2021-01-01',
            end_date: '',
            bullets: ['Keep', 'New']
          }
        ]
      })
    )

    expect(result.updatedWorkExperience).toBe(1)
    expect(calls.map(summary)).toEqual([
      'PUT /api/profile/work-experience/9',
      'POST /api/profile/work-experience/9/bullets',
      'DELETE /api/profile/work-experience/9/bullets/2'
    ])
    expect(calls[0].body).toMatchObject({ job_type: 'full_time', location: 'Remote' })
  })

  it('skips unchanged work experience', async () => {
    const profile = makeProfile({
      name: 'A',
      linked_in: 'a',
      github: 'a',
      portfolio: 'a',
      work_experience: [
        {
          id: 9,
          company: 'Acme',
          job_title: 'Dev',
          job_type: 'full_time',
          location: 'Remote',
          start_date: '2021-01-01',
          end_date: null,
          bullets: [{ id: 1, bullet: 'Keep', position: 0 }]
        }
      ]
    })

    const result = await applyResumeExtractionToProfile(
      profile,
      makeExtraction({
        skills: [],
        work_experience: [
          { company: 'Acme', job_title: 'Dev', location: '', start_date: 'bad', end_date: '', bullets: ['Keep'] }
        ]
      })
    )

    expect(result.updatedWorkExperience).toBe(0)
    expect(calls).toEqual([])
  })

  it('handles null lists on both sides', async () => {
    const profile = makeProfile({
      name: 'A',
      linked_in: 'a',
      github: 'a',
      portfolio: 'a',
      skills: null,
      education: null,
      work_experience: null
    })

    const result = await applyResumeExtractionToProfile(
      profile,
      makeExtraction({ skills: null, education: null, work_experience: null })
    )

    expect(calls).toEqual([])
    expect(result.addedSkills).toBe(0)
  })

  it('treats missing dates on both sides as unchanged', async () => {
    const profile = makeProfile({
      name: 'A',
      linked_in: 'a',
      github: 'a',
      portfolio: 'a',
      education: [
        { id: 7, school_name: 'MIT', major: 'CS', degree: 'BS', gpa: null, start_date: null, end_date: null }
      ],
      work_experience: [
        {
          id: 9,
          company: 'Acme',
          job_title: 'Dev',
          job_type: 'full_time',
          location: null,
          start_date: null,
          end_date: null,
          bullets: null
        },
        {
          id: 10,
          company: 'Beta',
          job_title: 'Dev',
          job_type: 'full_time',
          location: 'Remote',
          start_date: '2020-01-01',
          end_date: null,
          bullets: []
        }
      ]
    })

    const result = await applyResumeExtractionToProfile(
      profile,
      makeExtraction({
        skills: [],
        education: [
          { school_name: 'MIT', major: 'CS', degree: 'BS', start_date: 'soon', end_date: '' },
          { school_name: 'Yale', major: 'Art', degree: 'BA', start_date: 'soon', end_date: '' }
        ],
        work_experience: [
          { company: 'Acme', job_title: 'Dev', location: '', start_date: 'bad', end_date: '', bullets: null },
          { company: 'Beta', job_title: 'Dev', location: '', start_date: '', end_date: '', bullets: ['Added'] }
        ]
      })
    )

    expect(result).toMatchObject({ addedEducation: 1, updatedEducation: 0, updatedWorkExperience: 1 })
    expect(calls.map(summary)).toEqual(['POST /api/profile/education', 'POST /api/profile/work-experience/10/bullets'])
    expect(calls[0].body).toEqual({ school_name: 'Yale', major: 'Art', degree: 'BA' })
  })
})
