import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Profile, ResumeExtraction } from '@/src/api/schemas'
import { signIn } from '@/src/test/auth-mock'
import { makeExtraction, makeJob, makeProfile, makeResume, makeTailoredResume } from '@/src/test/fixtures'
import { renderWithProviders } from '@/src/test/render'
import { api, server } from '@/src/test/server'
import CoverLetterSection from './cover-letter-section'
import FitSection from './fit-section'
import JobApplyPanel from './job-apply-panel'
import OpenAndToggle from './open-and-toggle'

vi.mock('@/src/stores/auth-store', () => import('@/src/test/auth-mock'))

const job = makeJob({ id: 1, tags: ['go', 'kubernetes'], description: 'We use Go and Postgres.' })

function serve({
  profile = makeProfile({ name: 'Ada', github: 'https://github.com/ada', resumes: [makeResume({ id: 10 })] }),
  extraction = makeExtraction()
}: { profile?: Profile; extraction?: ResumeExtraction } = {}) {
  server.use(
    http.get(api('/api/profile'), () => HttpResponse.json(profile)),
    http.get(api('/api/profile/resumes/10/extraction'), () => HttpResponse.json(extraction)),
    http.get(api('/api/jobs/1/tailored-resume'), () => HttpResponse.json({ error: 'none' }, { status: 404 }))
  )
}

beforeEach(() => signIn())

describe('JobApplyPanel', () => {
  it('lists copyable contact details, resume and fit', async () => {
    serve()
    const user = userEvent.setup()
    await renderWithProviders(<JobApplyPanel job={job} />)

    expect(await screen.findByRole('heading', { name: 'Apply assist' })).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Copy Email' }))
    expect(await navigator.clipboard.readText()).toBe('ada@example.com')

    await waitFor(() => expect(screen.getByRole('button', { name: 'Copy Phone' })).toBeInTheDocument())
    expect(screen.queryByRole('button', { name: 'Copy LinkedIn' })).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /Copy all/ }))
    expect(await navigator.clipboard.readText()).toBe(
      'Name: Ada\nEmail: ada@example.com\nPhone: 555-0100\nGitHub: https://github.com/ada'
    )

    expect(screen.getByText('resume.pdf')).toBeInTheDocument()
    expect(screen.getByText('Fit check')).toBeInTheDocument()
    expect(screen.getByText('Go')).toBeInTheDocument()
    expect(screen.getByText('kubernetes')).toBeInTheDocument()
  })

  it('drafts a cover letter on demand', async () => {
    serve()
    server.use(
      http.post(api('/api/jobs/1/generate'), () =>
        HttpResponse.json({ cover_letter: 'Dear Acme,', tailored_bullets: ['Built Go APIs'] })
      )
    )
    await renderWithProviders(<JobApplyPanel job={job} />)

    await userEvent.click(await screen.findByRole('button', { name: 'Draft letter for this role' }))

    expect(await screen.findByRole('textbox', { name: 'Cover letter draft' })).toHaveValue('Dear Acme,')
  })

  it('shows a generation error', async () => {
    serve()
    server.use(
      http.post(api('/api/jobs/1/generate'), () => HttpResponse.json({ error: 'failed to generate' }, { status: 502 }))
    )
    await renderWithProviders(<JobApplyPanel job={job} />)

    await userEvent.click(await screen.findByRole('button', { name: 'Draft letter for this role' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('failed to generate')
  })

  it('waits on resume parsing', async () => {
    serve({ extraction: makeExtraction({ status: 'pending' }) })
    await renderWithProviders(<JobApplyPanel job={job} />)

    expect(await screen.findByText('Waiting on resume parsing before a letter can be drafted.')).toBeInTheDocument()
    expect(screen.getByText('Waiting on resume parsing before it can be tailored.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Draft letter/ })).not.toBeInTheDocument()
  })

  it('prompts for a resume when none is active', async () => {
    serve({ profile: makeProfile({ resumes: [makeResume({ is_active: false })] }) })
    await renderWithProviders(<JobApplyPanel job={job} />)

    expect(await screen.findByText(/No active resume/)).toBeInTheDocument()
    expect(screen.getByText('Upload and activate a resume to draft a letter for this role.')).toBeInTheDocument()
    expect(screen.queryByText('Fit check')).not.toBeInTheDocument()
  })

  it('downloads the active resume', async () => {
    serve()
    server.use(http.get(api('/api/profile/resumes/10/download'), () => new HttpResponse('PDF')))
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
    await renderWithProviders(<JobApplyPanel job={job} />)

    await userEvent.click(await screen.findByRole('button', { name: /Download/ }))

    await waitFor(() => expect(click).toHaveBeenCalled())
  })

  it('reports a failed download', async () => {
    serve()
    server.use(
      http.get(api('/api/profile/resumes/10/download'), () =>
        HttpResponse.json({ error: 'resume not found' }, { status: 404 })
      )
    )
    await renderWithProviders(<JobApplyPanel job={job} />)

    await userEvent.click(await screen.findByRole('button', { name: /Download/ }))

    expect(await screen.findByRole('alert')).toHaveTextContent('resume not found')
  })

  it('tailors a resume, confirming before a regenerate', async () => {
    serve()
    let posts = 0
    server.use(
      http.post(api('/api/jobs/1/tailored-resume'), () => {
        posts++
        return HttpResponse.json(makeTailoredResume({ stale: true }))
      })
    )
    await renderWithProviders(<JobApplyPanel job={job} />)

    await userEvent.click(await screen.findByRole('button', { name: 'Tailor resume for this role' }))

    expect(await screen.findByRole('link', { name: /Open editor/ })).toHaveAttribute('href', '/jobs/1/resume')
    expect(screen.getByText('Resume changed')).toBeInTheDocument()

    vi.mocked(window.confirm).mockReturnValueOnce(false)
    await userEvent.click(screen.getByRole('button', { name: 'Regenerate' }))
    expect(window.confirm).toHaveBeenCalled()
    expect(posts).toBe(1)

    await userEvent.click(screen.getByRole('button', { name: 'Regenerate' }))
    await waitFor(() => expect(posts).toBe(2))
  })

  it('asks for a profile when it has nothing to copy', async () => {
    serve({ profile: makeProfile({ email: '', resumes: [] }) })
    await renderWithProviders(<JobApplyPanel job={job} />)

    expect(await screen.findByText('Fill in your profile to use apply assist.')).toBeInTheDocument()
  })
})

describe('CoverLetterSection', () => {
  const data = { cover_letter: 'Dear Acme,  ', tailored_bullets: ['Built APIs', 'Ran on-call'] }

  it('edits, appends bullets, copies, downloads and regenerates', async () => {
    const user = userEvent.setup()
    const onRegenerate = vi.fn()
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (this: HTMLAnchorElement) {
      expect(this.download).toBe('acme-backend-engineer-cover-letter.txt')
    })
    await renderWithProviders(
      <CoverLetterSection job={job} data={data} onRegenerate={onRegenerate} isPending={false} />
    )

    const draft = screen.getByRole('textbox', { name: 'Cover letter draft' })
    await user.click(screen.getAllByRole('button', { name: 'Add bullet to letter' })[0])
    expect(draft).toHaveValue('Dear Acme,\n\n• Built APIs')

    await user.type(draft, ' Thanks!')
    await user.click(screen.getByRole('button', { name: /Copy letter/ }))
    expect(await navigator.clipboard.readText()).toBe('Dear Acme,\n\n• Built APIs Thanks!')
    expect(screen.getByRole('button', { name: /Copied!/ })).toBeInTheDocument()

    await user.click(screen.getAllByRole('button', { name: 'Copy bullet' })[1])
    expect(await navigator.clipboard.readText()).toBe('Ran on-call')

    await user.click(screen.getByRole('button', { name: /Download .txt/ }))
    expect(click).toHaveBeenCalledOnce()

    await user.click(screen.getByRole('button', { name: /Regenerate/ }))
    expect(onRegenerate).toHaveBeenCalledOnce()
  })

  it('disables regenerate while pending and hides empty bullets', async () => {
    await renderWithProviders(
      <CoverLetterSection
        job={job}
        data={{ cover_letter: 'x', tailored_bullets: [] }}
        onRegenerate={() => {}}
        isPending
      />
    )

    expect(screen.getByRole('button', { name: /Generating…/ })).toBeDisabled()
    expect(screen.queryByText(/Tailored bullets/)).not.toBeInTheDocument()
  })
})

describe('FitSection', () => {
  it('renders nothing without skills', async () => {
    const { container } = await renderWithProviders(<FitSection matched={[]} missing={[]} />)
    expect(container.querySelector('section')).toBeNull()
  })

  it('caps each list at twelve', async () => {
    const many = Array.from({ length: 15 }, (_, i) => `skill${i}`)
    await renderWithProviders(<FitSection matched={many} missing={[]} />)

    expect(screen.getByText('skill11')).toBeInTheDocument()
    expect(screen.queryByText('skill12')).not.toBeInTheDocument()
    expect(screen.queryByText('Asked for, not on your resume')).not.toBeInTheDocument()
  })
})

describe('OpenAndToggle', () => {
  it('links out and toggles applied', async () => {
    let applied = 0
    let removed = 0
    server.use(
      http.post(api('/api/jobs/1/apply'), () => {
        applied++
        return HttpResponse.json({ status: 'applied' })
      }),
      http.delete(api('/api/jobs/1/apply'), () => {
        removed++
        return HttpResponse.json({ status: 'unapplied' })
      })
    )

    const { unmount } = await renderWithProviders(<OpenAndToggle job={job} />)
    const link = screen.getByRole('link', { name: /View posting/ })
    expect(link).toHaveAttribute('href', job.url)
    expect(link).toHaveAttribute('target', '_blank')
    expect(link).toHaveAttribute('rel', 'noreferrer')

    await userEvent.click(screen.getByRole('button', { name: 'Mark as applied' }))
    await waitFor(() => expect(applied).toBe(1))
    unmount()

    await renderWithProviders(<OpenAndToggle job={{ ...job, applied: true }} />)
    await userEvent.click(within(document.body).getByRole('button', { name: /Remove from applied/ }))
    await waitFor(() => expect(removed).toBe(1))
  })
})
