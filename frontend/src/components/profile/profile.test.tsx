import { renderHook, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { ResumeExtraction } from '@/src/api/schemas'
import { signIn } from '@/src/test/auth-mock'
import { makeExtraction, makeProfile, makeResume } from '@/src/test/fixtures'
import { useProfileQuery } from '@/src/hooks/use-profile'
import { renderWithProviders } from '@/src/test/render'
import { api, server } from '@/src/test/server'
import { BasicInfoSection } from './basic-info-section'
import { EducationSection } from './education-section'
import { ProfileOutletProvider, useProfileOutletContext } from './profile-context'
import { ResumesSection } from './resumes-section'
import { SkillsSection } from './skills-section'
import { WorkExperienceSection } from './work-experience-section'

vi.mock('@/src/stores/auth-store', () => import('@/src/test/auth-mock'))
vi.mock('@mapbox/search-js-react', () => ({
  AddressAutofill: ({ children }: { children: React.ReactNode }) => children
}))

interface Call {
  method: string
  path: string
  body: unknown
}

let calls: Call[]
const mutations = () => calls.filter((c) => !(c.method === 'GET'))
const summary = (c: Call) => `${c.method} ${c.path}`

// Records every profile request; writes answer like the backend does.
function recordProfileApi(extraction: ResumeExtraction = makeExtraction()) {
  calls = []
  server.use(
    http.get(api('/api/profile/resumes/:id/extraction'), () => HttpResponse.json(extraction)),
    http.get(api('/api/profile'), () => HttpResponse.json(makeProfile())),
    http.all(api('/api/profile/*'), async ({ request }) => {
      const type = request.headers.get('Content-Type') ?? ''
      const body = type.includes('json') ? await request.json() : null
      const path = new URL(request.url).pathname
      calls.push({ method: request.method, path, body })
      return HttpResponse.json(
        request.method === 'POST' && !path.endsWith('/extraction') ? { id: 99 } : { status: 'ok' }
      )
    }),
    http.put(api('/api/profile'), async ({ request }) => {
      calls.push({ method: 'PUT', path: '/api/profile', body: await request.json() })
      return HttpResponse.json({ status: 'updated' })
    })
  )
}

beforeEach(() => {
  signIn()
  recordProfileApi()
})

describe('profile context', () => {
  it('throws outside the provider and provides the profile inside it', () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    expect(() => renderHook(() => useProfileOutletContext())).toThrow('/profile route')

    const profile = makeProfile()
    const { result } = renderHook(() => useProfileOutletContext(), {
      wrapper: ({ children }) => <ProfileOutletProvider profile={profile}>{children}</ProfileOutletProvider>
    })
    expect(result.current.profile).toBe(profile)
  })
})

// Feeds the section from the profile query like the route does, so autosave's cache patch flows back in.
function BasicInfoFromQuery() {
  const { data } = useProfileQuery()
  return data ? <BasicInfoSection profile={data} /> : null
}

async function renderBasicInfo(profile = makeProfile()) {
  server.use(http.get(api('/api/profile'), () => HttpResponse.json(profile)))
  await renderWithProviders(<BasicInfoFromQuery />)
  await screen.findByRole('textbox', { name: 'Name' })
}

// Same-tick keystrokes race autosave's form reset in a way no person can type; space them out.
const typist = () => userEvent.setup({ delay: 20 })

describe('BasicInfoSection', () => {
  it('autosaves edits', async () => {
    await renderBasicInfo(makeProfile({ name: 'Ada' }))

    await typist().type(screen.getByRole('textbox', { name: 'Name' }), ' L')

    await waitFor(() => expect(screen.getByText('All changes saved')).toBeInTheDocument())
    expect(calls.at(-1)).toMatchObject({
      method: 'PUT',
      body: { name: 'Ada L', address: '', linked_in: '', email_notifications: false }
    })
  })

  it('blocks saving invalid URLs', async () => {
    await renderBasicInfo()

    await userEvent.type(screen.getByRole('textbox', { name: 'GitHub URL' }), 'not a url')

    expect(await screen.findByText('Fix the highlighted fields to save')).toBeInTheDocument()
    expect(screen.getByText('Enter a valid URL')).toBeInTheDocument()
    expect(calls).toEqual([])
  })

  it('toggles the email digest', async () => {
    await renderBasicInfo()

    await userEvent.click(screen.getByRole('switch', { name: 'Email me matching jobs' }))

    await waitFor(() => expect(calls.at(-1)?.body).toMatchObject({ email_notifications: true }))
  })

  it('edits the address', async () => {
    await renderBasicInfo()

    await typist().type(screen.getByRole('textbox', { name: 'Address' }), 'London')

    await waitFor(() => expect(calls.at(-1)?.body).toMatchObject({ address: 'London' }))
  })

  it('reports a save that keeps failing', async () => {
    server.use(http.put(api('/api/profile'), () => HttpResponse.json({ error: 'bad' }, { status: 400 })))
    await renderBasicInfo()

    await userEvent.type(screen.getByRole('textbox', { name: 'Name' }), 'A')

    expect(await screen.findByText("Couldn't save changes")).toBeInTheDocument()
  })
})

describe('SkillsSection', () => {
  it('adds trimmed skills and removes them', async () => {
    await renderWithProviders(<SkillsSection skills={[{ id: 3, skill: 'Go' }]} />)

    const input = screen.getByRole('textbox', { name: 'Add a skill' })
    await userEvent.type(input, '  Rust  ')
    await userEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() =>
      expect(calls).toContainEqual({ method: 'POST', path: '/api/profile/skills', body: { skill: 'Rust' } })
    )
    await waitFor(() => expect(input).toHaveValue(''))

    await userEvent.click(screen.getByRole('button', { name: 'Remove Go' }))
    await waitFor(() => expect(calls.map(summary)).toContain('DELETE /api/profile/skills/3'))
  })

  it('ignores blank skills', async () => {
    await renderWithProviders(<SkillsSection skills={[]} />)

    await userEvent.type(screen.getByRole('textbox', { name: 'Add a skill' }), '   ')
    await userEvent.click(screen.getByRole('button', { name: 'Add' }))

    expect(calls).toEqual([])
  })
})

describe('EducationSection', () => {
  const entry = {
    id: 7,
    school_name: 'MIT',
    major: 'CS',
    degree: 'BS',
    gpa: 3.9,
    start_date: '2018-09-01',
    end_date: '2022-05-01'
  }

  it('adds an entry', async () => {
    await renderWithProviders(<EducationSection education={[]} />)

    await userEvent.type(screen.getByRole('textbox', { name: 'School' }), ' Stanford ')
    await userEvent.type(screen.getByRole('textbox', { name: 'Major' }), 'EE')
    await userEvent.type(screen.getByRole('textbox', { name: 'Degree' }), 'MS')
    await userEvent.type(screen.getByRole('spinbutton', { name: 'GPA' }), '3.5')
    await userEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() =>
      expect(calls).toContainEqual({
        method: 'POST',
        path: '/api/profile/education',
        body: { school_name: 'Stanford', major: 'EE', degree: 'MS', gpa: 3.5 }
      })
    )
  })

  it('validates required fields, GPA and date order', async () => {
    await renderWithProviders(<EducationSection education={[]} />)

    await userEvent.click(screen.getByRole('button', { name: 'Add' }))
    expect(await screen.findAllByText('Required')).toHaveLength(3)

    await userEvent.type(screen.getByRole('spinbutton', { name: 'GPA' }), '5')
    expect(await screen.findByText('GPA must be 0–4')).toBeInTheDocument()

    await userEvent.type(screen.getByLabelText('Start date'), '2022-01-01')
    await userEvent.type(screen.getByLabelText('End date'), '2021-01-01')
    expect(await screen.findByText('End is before start')).toBeInTheDocument()

    expect(calls).toEqual([])
  })

  it('shows, edits and removes entries', async () => {
    await renderWithProviders(<EducationSection education={[entry]} />)

    expect(screen.getByText('MIT')).toBeInTheDocument()
    expect(screen.getByText('CS - BS')).toBeInTheDocument()
    expect(screen.getByText('3.90')).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Edit' }))
    // Saving unchanged skips the request.
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Edit' })).toBeInTheDocument())
    expect(calls).toEqual([])

    await userEvent.click(screen.getByRole('button', { name: 'Edit' }))
    const major = screen.getAllByRole('textbox', { name: 'Major' })[0]
    await userEvent.clear(major)
    await userEvent.type(major, 'Math')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(calls).toContainEqual({
        method: 'PUT',
        path: '/api/profile/education/7',
        body: {
          school_name: 'MIT',
          major: 'Math',
          degree: 'BS',
          gpa: 3.9,
          start_date: '2018-09-01',
          end_date: '2022-05-01'
        }
      })
    )

    await userEvent.click(await screen.findByRole('button', { name: 'Remove' }))
    await waitFor(() => expect(calls.map(summary)).toContain('DELETE /api/profile/education/7'))
  })

  it('cancels an edit and shows update errors', async () => {
    server.use(
      http.put(api('/api/profile/education/7'), () =>
        HttpResponse.json({ error: 'invalid start_date' }, { status: 400 })
      )
    )
    await renderWithProviders(<EducationSection education={[entry]} />)

    await userEvent.click(screen.getByRole('button', { name: 'Edit' }))
    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(screen.getByRole('button', { name: 'Edit' })).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Edit' }))
    await userEvent.type(screen.getAllByRole('textbox', { name: 'Degree' })[0], 'S')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('invalid start_date')
  })
})

describe('WorkExperienceSection', () => {
  const entry = {
    id: 4,
    company: 'Acme',
    job_title: 'Dev',
    job_type: 'full_time' as const,
    location: 'NYC',
    start_date: '2021-01-01',
    end_date: null,
    bullets: [{ id: 8, bullet: 'Shipped it', position: 0 }]
  }

  it('adds work experience with a chosen job type', async () => {
    await renderWithProviders(<WorkExperienceSection workExperience={[]} />)

    await userEvent.type(screen.getByRole('textbox', { name: 'Company' }), ' Globex ')
    await userEvent.type(screen.getByRole('textbox', { name: 'Job title' }), 'SRE')
    await userEvent.click(screen.getByRole('button', { name: 'Job type' }))
    await userEvent.click(await screen.findByRole('menuitem', { name: 'Contract' }))
    expect(screen.getByRole('button', { name: 'Job type' })).toHaveTextContent('Contract')

    await userEvent.click(screen.getByRole('button', { name: 'Add' }))

    await waitFor(() =>
      expect(calls).toContainEqual({
        method: 'POST',
        path: '/api/profile/work-experience',
        body: { company: 'Globex', job_title: 'SRE', job_type: 'contract', location: '', start_date: '', end_date: '' }
      })
    )
  })

  it('validates the new entry', async () => {
    await renderWithProviders(<WorkExperienceSection workExperience={[]} />)

    await userEvent.click(screen.getByRole('button', { name: 'Add' }))

    expect(await screen.findAllByText('Required')).toHaveLength(2)
    expect(calls).toEqual([])
  })

  it('shows entries and manages bullets', async () => {
    await renderWithProviders(<WorkExperienceSection workExperience={[entry]} />)

    expect(screen.getByText('Dev · Acme')).toBeInTheDocument()
    expect(screen.getByText('Full-time · NYC · 2021-01-01 – present')).toBeInTheDocument()
    expect(screen.getByText('• Shipped it')).toBeInTheDocument()

    const card = screen.getByText('Dev · Acme').closest('.rounded-lg') as HTMLElement

    await userEvent.type(within(card).getByRole('textbox', { name: 'Add a bullet point' }), 'Led it')
    await userEvent.click(within(card).getByRole('button', { name: 'Add' }))
    await waitFor(() =>
      expect(calls).toContainEqual({
        method: 'POST',
        path: '/api/profile/work-experience/4/bullets',
        body: { bullet: 'Led it' }
      })
    )

    await userEvent.click(within(card).getByRole('button', { name: 'Edit' }))
    const editBox = within(card).getByRole('textbox', { name: 'Bullet point' })
    await userEvent.clear(editBox)
    await userEvent.click(within(card).getByRole('button', { name: 'Save' }))
    expect(await within(card).findByText('Required')).toBeInTheDocument()

    await userEvent.type(editBox, 'Shipped it twice')
    await userEvent.click(within(card).getByRole('button', { name: 'Save' }))
    await waitFor(() =>
      expect(calls).toContainEqual({
        method: 'PUT',
        path: '/api/profile/work-experience/4/bullets/8',
        body: { bullet: 'Shipped it twice' }
      })
    )

    const removes = within(card).getAllByRole('button', { name: 'Remove' })
    await userEvent.click(removes.at(-1)!)
    await waitFor(() => expect(calls.map(summary)).toContain('DELETE /api/profile/work-experience/4/bullets/8'))

    await userEvent.click(removes[0])
    await waitFor(() => expect(calls.map(summary)).toContain('DELETE /api/profile/work-experience/4'))
  })

  it('cancels a bullet edit without saving', async () => {
    await renderWithProviders(<WorkExperienceSection workExperience={[entry]} />)

    await userEvent.click(screen.getByRole('button', { name: 'Edit' }))
    await userEvent.type(screen.getByRole('textbox', { name: 'Bullet point' }), ' extra')
    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }))

    expect(screen.getByText('• Shipped it')).toBeInTheDocument()
    expect(mutations()).toEqual([])
  })
})

describe('ResumesSection', () => {
  it('uploads a resume then starts extraction', async () => {
    const profile = makeProfile()
    await renderWithProviders(<ResumesSection resumes={[]} profile={profile} />)

    expect(screen.getByText('No resumes uploaded yet.')).toBeInTheDocument()
    const upload = screen.getByRole('button', { name: 'Upload resume' })
    expect(upload).toBeDisabled()

    await userEvent.upload(
      screen.getByLabelText('Resume file'),
      new File(['pdf'], 'cv.pdf', { type: 'application/pdf' })
    )
    await userEvent.click(upload)

    await waitFor(() =>
      expect(calls.map(summary)).toEqual(['POST /api/profile/resumes', 'POST /api/profile/resumes/99/extraction'])
    )
  })

  it('shows an upload error', async () => {
    server.use(
      http.post(api('/api/profile/resumes'), () =>
        HttpResponse.json({ error: 'resume must be a PDF' }, { status: 400 })
      )
    )
    await renderWithProviders(<ResumesSection resumes={[]} profile={makeProfile()} />)

    await userEvent.upload(screen.getByLabelText('Resume file'), new File(['x'], 'cv.pdf'))
    await userEvent.click(screen.getByRole('button', { name: 'Upload resume' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('resume must be a PDF')
  })

  it('renames, activates, downloads, replaces and deletes a resume', async () => {
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
    server.use(http.get(api('/api/profile/resumes/10/download'), () => new HttpResponse('PDF')))
    await renderWithProviders(
      <ResumesSection
        resumes={[makeResume({ id: 10, file_name: 'my.cv.pdf', file_size: 3 * 1024 * 1024, is_active: false })]}
        profile={makeProfile()}
      />
    )

    expect(screen.getByText('3.0 MB')).toBeInTheDocument()
    const name = screen.getByRole('textbox', { name: 'Resume name' })
    expect(name).toHaveValue('my.cv')
    expect(screen.getByText('.pdf')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Rename' })).toBeDisabled()

    await userEvent.clear(name)
    await userEvent.type(name, 'final')
    await userEvent.click(screen.getByRole('button', { name: 'Rename' }))
    await waitFor(() => expect(calls.map(summary)).toContain('PUT /api/profile/resumes/10'))

    await userEvent.click(screen.getByRole('button', { name: 'Set active' }))
    await waitFor(() => expect(calls.map(summary)).toContain('POST /api/profile/resumes/10/activate'))

    await userEvent.click(screen.getByRole('button', { name: /Download/ }))
    await waitFor(() => expect(click).toHaveBeenCalled())

    await userEvent.upload(screen.getByLabelText('Replace file'), new File(['new'], 'new.pdf'))
    await waitFor(() => expect(calls.map(summary)).toContain('POST /api/profile/resumes/10/extraction'))

    await userEvent.click(screen.getByRole('button', { name: 'Delete' }))
    await waitFor(() => expect(calls.map(summary)).toContain('DELETE /api/profile/resumes/10'))
  })

  it('shows small files in KB and the active badge', async () => {
    await renderWithProviders(<ResumesSection resumes={[makeResume({ file_size: 100 })]} profile={makeProfile()} />)

    expect(screen.getByText('1 KB')).toBeInTheDocument()
    expect(screen.getByText('Active')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Set active' })).not.toBeInTheDocument()
  })

  it('shows extracted details and applies them to the profile', async () => {
    recordProfileApi(
      makeExtraction({
        skills: ['Go'],
        education: [{ school_name: 'MIT', degree: 'BS', major: 'CS', start_date: '2018-09-01', end_date: '' }],
        work_experience: [
          {
            company: 'Acme',
            job_title: 'Dev',
            location: 'NYC',
            start_date: '2021-01-01',
            end_date: '',
            bullets: ['Shipped']
          }
        ],
        projects: [
          { name: 'Crawler', url: 'https://github.com/ada/crawler', technologies: ['Go'], bullets: ['Indexed'] },
          { name: 'Game', url: '', technologies: null, bullets: null }
        ]
      })
    )
    await renderWithProviders(<ResumesSection resumes={[makeResume()]} profile={makeProfile()} />)

    expect(await screen.findByText('BS in CS · MIT')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'https://linkedin.com/in/ada' })).toHaveAttribute('target', '_blank')
    expect(screen.getByText('Dev · Acme')).toBeInTheDocument()
    expect(screen.getByText('NYC · 2021-01-01 – present')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Crawler' })).toHaveAttribute('href', 'https://github.com/ada/crawler')
    expect(screen.getByText('Game')).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Apply to profile' }))

    expect(
      await screen.findByText(
        '1 education entry added, 1 skill added, 1 work experience entry added, basic info updated.'
      )
    ).toBeInTheDocument()
  })

  it('says when applying adds nothing', async () => {
    recordProfileApi(
      makeExtraction({
        full_name: '',
        email: '',
        phone: '',
        linked_in: '',
        github: '',
        portfolio: '',
        summary: '',
        skills: []
      })
    )
    await renderWithProviders(
      <ResumesSection
        resumes={[makeResume()]}
        profile={makeProfile({ name: 'A', linked_in: 'a', github: 'a', portfolio: 'a' })}
      />
    )

    expect(await screen.findByText('No details were found in this resume.')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Apply to profile' }))
    expect(await screen.findByText('Nothing new to add, profile already has this information.')).toBeInTheDocument()
  })

  it.each([
    ['pending', 'Extracting details...'],
    ['unsupported', ".doc files aren't automatically parsed. Upload a PDF or DOCX to see extracted details."]
  ] as const)('shows the %s extraction state', async (status, text) => {
    recordProfileApi(makeExtraction({ status }))
    await renderWithProviders(<ResumesSection resumes={[makeResume()]} profile={makeProfile()} />)

    expect(await screen.findByText(text)).toBeInTheDocument()
  })

  it('retries a failed extraction', async () => {
    recordProfileApi(makeExtraction({ status: 'failed', error: 'model timed out' }))
    await renderWithProviders(<ResumesSection resumes={[makeResume({ id: 10 })]} profile={makeProfile()} />)

    expect(await screen.findByText("Couldn't extract details: model timed out")).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Retry' }))

    await waitFor(() => expect(calls.map(summary)).toContain('POST /api/profile/resumes/10/extraction'))
  })
})
