import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { describe, expect, it, vi } from 'vitest'
import type { ResumeExtraction, TailoredResumeContent } from '@/src/api/schemas'
import { makeExtraction, makeTailoredContent } from '@/src/test/fixtures'
import ResumeEditor from './resume-editor'
import ResumePreview from './resume-preview'

const withProject = (): TailoredResumeContent =>
  makeTailoredContent({
    skills: ['Go', 'SQL'],
    projects: [
      {
        name: 'Crawler',
        url: 'https://github.com/ada/crawler',
        technologies: ['Go'],
        bullets: [{ text: 'Indexed jobs', source: 'P0.B0' }]
      }
    ]
  })

// Holds editor state like the page does, and exposes the latest content.
function renderEditor(initial: TailoredResumeContent, extraction?: ResumeExtraction) {
  const latest = { current: initial }

  function Harness() {
    const [content, setContent] = useState(initial)
    latest.current = content
    return <ResumeEditor content={content} onChange={setContent} extraction={extraction} />
  }

  render(<Harness />)
  return latest
}

const bulletTexts = (latest: { current: TailoredResumeContent }) =>
  latest.current.work_experience[0].bullets.map((b) => b.text)

describe('ResumeEditor', () => {
  it('edits the summary', async () => {
    const latest = renderEditor(withProject())

    const summary = screen.getByLabelText('Summary')
    await userEvent.clear(summary)
    await userEvent.type(summary, 'Backend engineer')

    expect(latest.current.summary).toBe('Backend engineer')
  })

  it('removes skills', async () => {
    const latest = renderEditor(withProject())

    await userEvent.click(screen.getByRole('button', { name: 'Remove Go' }))

    expect(latest.current.skills).toEqual(['SQL'])
  })

  it('reorders, edits, adds and removes bullets', async () => {
    const latest = renderEditor(withProject())
    const role = screen.getByText('Engineer · Acme').parentElement!

    const up = within(role).getAllByRole('button', { name: 'Move bullet up' })
    const down = within(role).getAllByRole('button', { name: 'Move bullet down' })
    expect(up[0]).toBeDisabled()
    expect(down[1]).toBeDisabled()

    await userEvent.click(down[0])
    expect(bulletTexts(latest)).toEqual(['Ran on-call', 'Built APIs'])

    await userEvent.click(within(role).getAllByRole('button', { name: 'Move bullet up' })[1])
    expect(bulletTexts(latest)).toEqual(['Built APIs', 'Ran on-call'])

    await userEvent.type(within(role).getByLabelText('Bullet 1'), ' fast')
    expect(bulletTexts(latest)[0]).toBe('Built APIs fast')

    await userEvent.click(within(role).getByRole('button', { name: /Add bullet/ }))
    expect(latest.current.work_experience[0].bullets.at(-1)).toEqual({ text: '', source: '' })

    await userEvent.click(within(role).getAllByRole('button', { name: 'Remove bullet' })[0])
    expect(bulletTexts(latest)).toEqual(['Ran on-call', ''])

    // Projects were untouched.
    expect(latest.current.projects[0].bullets).toEqual([{ text: 'Indexed jobs', source: 'P0.B0' }])
  })

  it('edits project bullets and removes projects', async () => {
    const latest = renderEditor(withProject())
    const project = screen.getByText('Crawler').closest('div')!.parentElement!

    await userEvent.type(within(project).getByLabelText('Bullet 1'), '!')
    expect(latest.current.projects[0].bullets[0].text).toBe('Indexed jobs!')

    await userEvent.click(within(project).getByRole('button', { name: /^Remove$/ }))
    expect(latest.current.projects).toEqual([])
    expect(screen.queryByText('Projects')).not.toBeInTheDocument()
  })

  it('shows the original bullet once the text differs', async () => {
    const extraction = makeExtraction({
      work_experience: [
        {
          company: 'Acme',
          job_title: 'Engineer',
          location: '',
          start_date: '',
          end_date: '',
          bullets: ['Wrote the APIs', 'Ran on-call']
        }
      ],
      projects: [{ name: 'Crawler', url: '', technologies: null, bullets: ['Indexed jobs'] }]
    })
    renderEditor(withProject(), extraction)

    expect(screen.getByText('Original: Wrote the APIs')).toBeInTheDocument()
    // Unchanged and unsourced bullets show nothing.
    expect(screen.queryByText('Original: Ran on-call')).not.toBeInTheDocument()
    expect(screen.queryByText('Original: Indexed jobs')).not.toBeInTheDocument()
  })

  it('ignores malformed or out-of-range sources', () => {
    const content = makeTailoredContent({
      work_experience: [
        {
          company: 'Acme',
          job_title: 'Engineer',
          location: '',
          start_date: '',
          end_date: '',
          bullets: [
            { text: 'a', source: 'bogus' },
            { text: 'b', source: 'W9.B9' }
          ]
        }
      ]
    })
    renderEditor(content, makeExtraction())

    expect(screen.queryByText(/^Original:/)).not.toBeInTheDocument()
  })

  it('hides empty sections', () => {
    renderEditor(makeTailoredContent({ skills: [], work_experience: [], projects: [] }))

    expect(screen.queryByText('Skills')).not.toBeInTheDocument()
    expect(screen.queryByText('Experience')).not.toBeInTheDocument()
  })

  it('reports changes through onChange without mutating props', async () => {
    const content = withProject()
    const onChange = vi.fn()
    render(<ResumeEditor content={content} onChange={onChange} />)

    await userEvent.click(screen.getByRole('button', { name: 'Remove SQL' }))

    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ skills: ['Go'] }))
    expect(content.skills).toEqual(['Go', 'SQL'])
  })
})

describe('ResumePreview', () => {
  it('lays out every section', () => {
    render(<ResumePreview content={withProject()} />)
    const preview = screen.getByRole('article', { name: 'Resume preview' })

    expect(within(preview).getByText('Ada Lovelace')).toBeInTheDocument()
    expect(within(preview).getByText('ada@example.com | 555-0100')).toBeInTheDocument()
    expect(within(preview).getByText('Go engineer')).toBeInTheDocument()
    expect(within(preview).getByText('Go, SQL')).toBeInTheDocument()
    expect(within(preview).getByText('Engineer, Acme')).toBeInTheDocument()
    expect(within(preview).getByText('Jan 2022 – Present')).toBeInTheDocument()
    expect(within(preview).getByText('Remote')).toBeInTheDocument()
    expect(within(preview).getByText('Crawler (Go)')).toBeInTheDocument()
    expect(within(preview).getByText('https://github.com/ada/crawler')).toBeInTheDocument()
    expect(
      within(preview)
        .getAllByRole('listitem')
        .map((li) => li.textContent)
    ).toEqual(['Built APIs', 'Ran on-call', 'Indexed jobs'])
  })

  it('formats education and date ranges', () => {
    render(
      <ResumePreview
        content={makeTailoredContent({
          work_experience: [],
          education: [
            { school_name: 'MIT', degree: 'BS', major: 'CS', start_date: '2018-09-01', end_date: '2022-05-01' },
            { school_name: 'Stanford', degree: '', major: 'Math', start_date: '', end_date: '' },
            { school_name: 'Online', degree: 'Cert', major: '', start_date: 'Fall 2020', end_date: '' }
          ]
        })}
      />
    )

    expect(screen.getByText('BS in CS, MIT')).toBeInTheDocument()
    expect(screen.getByText('Sep 2018 – May 2022')).toBeInTheDocument()
    expect(screen.getByText('Math, Stanford')).toBeInTheDocument()
    expect(screen.getByText('Fall 2020 – Present')).toBeInTheDocument()
  })

  it('drops blank bullets, summary and contact line', () => {
    render(
      <ResumePreview
        content={makeTailoredContent({
          email: '',
          phone: '',
          summary: '   ',
          skills: [],
          work_experience: [
            {
              company: 'Acme',
              job_title: '',
              location: '',
              start_date: '',
              end_date: '',
              bullets: [{ text: '  ', source: '' }]
            }
          ]
        })}
      />
    )

    expect(screen.queryByText('Summary')).not.toBeInTheDocument()
    expect(screen.queryByText('Skills')).not.toBeInTheDocument()
    expect(screen.queryByRole('listitem')).not.toBeInTheDocument()
    expect(screen.getByText('Acme')).toBeInTheDocument()
  })
})
