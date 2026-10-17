import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { auth, signIn } from '@/src/test/auth-mock'
import { makeJobListItem } from '@/src/test/fixtures'
import { renderWithProviders } from '@/src/test/render'
import { Header } from './Header'
import { JobCard } from './jobs/job-card'
import { MatchRing } from './jobs/match-ring'
import Pagination from './pagination'
import { ProgressBar } from './progress-bar'

vi.mock('@/src/stores/auth-store', () => import('@/src/test/auth-mock'))

describe('Header', () => {
  it('links to each section and hides log out when signed out', async () => {
    await renderWithProviders(<Header />)

    const nav = screen.getByRole('navigation', { name: 'Main' })
    expect(within(nav).getByRole('link', { name: 'Jobs' })).toHaveAttribute('href', '/jobs')
    expect(within(nav).getByRole('link', { name: 'Profile' })).toHaveAttribute('href', '/profile')
    expect(within(nav).getByRole('link', { name: 'Trivia' })).toHaveAttribute('href', '/trivia')
    expect(screen.queryByRole('button', { name: 'Log out' })).not.toBeInTheDocument()
  })

  it('logs out when signed in', async () => {
    signIn()
    await renderWithProviders(<Header />)

    await userEvent.click(screen.getByRole('button', { name: 'Log out' }))

    expect(auth.logout).toHaveBeenCalledOnce()
  })
})

describe('Pagination', () => {
  it('renders nothing for a single page', async () => {
    await renderWithProviders(<Pagination page={1} total={20} pageSize={20} onPageChange={() => {}} />)

    expect(screen.queryByRole('navigation', { name: 'Pagination' })).not.toBeInTheDocument()
  })

  it('pages forward and back within bounds', async () => {
    const onPageChange = vi.fn()
    await renderWithProviders(<Pagination page={1} total={45} pageSize={20} onPageChange={onPageChange} />)

    expect(screen.getByText('Page 1 of 3')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Previous' })).toBeDisabled()

    await userEvent.click(screen.getByRole('button', { name: 'Next' }))
    expect(onPageChange).toHaveBeenCalledWith(2)
  })

  it('disables next on the last page', async () => {
    const onPageChange = vi.fn()
    await renderWithProviders(<Pagination page={3} total={45} pageSize={20} onPageChange={onPageChange} />)

    expect(screen.getByRole('button', { name: 'Next' })).toBeDisabled()
    await userEvent.click(screen.getByRole('button', { name: 'Previous' }))
    expect(onPageChange).toHaveBeenCalledWith(2)
  })
})

describe('ProgressBar', () => {
  it('shows only after a fetch has been pending for a moment', async () => {
    const { container, client } = await renderWithProviders(<ProgressBar />)
    const bar = () => container.querySelector('[aria-hidden="true"]')!

    expect(bar()).toHaveClass('opacity-0')

    let finish!: (v: string) => void
    act(() => {
      void client.fetchQuery({ queryKey: ['slow'], queryFn: () => new Promise<string>((r) => (finish = r)) })
    })

    expect(bar()).toHaveClass('opacity-0')
    await waitFor(() => expect(bar()).toHaveClass('opacity-100'))

    await act(async () => finish('done'))
    await waitFor(() => expect(bar()).toHaveClass('opacity-0'))
  })
})

describe('MatchRing', () => {
  it.each([
    [92, '92% match: Strong match', 'stroke-primary'],
    [70, '70% match: Good match', 'stroke-chart-2'],
    [45, '45% match: Fair match', 'stroke-chart-3'],
    [10, '10% match: Low match', 'stroke-muted-foreground/40']
  ])('%i%% is labelled and coloured', async (percent, label, arc) => {
    const { container } = await renderWithProviders(<MatchRing percent={percent} />)

    expect(screen.getByRole('img', { name: label })).toBeInTheDocument()
    expect(container.querySelectorAll('circle')[1]).toHaveClass(arc)
  })

  it('draws no arc at zero', async () => {
    const { container } = await renderWithProviders(<MatchRing percent={0} />)

    expect(container.querySelectorAll('circle')).toHaveLength(1)
  })
})

describe('JobCard', () => {
  it('shows the job summary and links to its page', async () => {
    await renderWithProviders(
      <ul>
        <JobCard job={makeJobListItem({ id: 42, tags: ['a', 'b', 'c', 'd', 'e', 'f', 'g'], applied: true })} />
      </ul>
    )

    expect(screen.getByRole('link')).toHaveAttribute('href', '/jobs/42')
    expect(screen.getByRole('heading', { name: 'Backend Engineer' })).toBeInTheDocument()
    expect(screen.getByText('Acme')).toBeInTheDocument()
    expect(screen.getByText('$120k – $150k')).toBeInTheDocument()
    expect(screen.getByText('2 hours ago')).toBeInTheDocument()
    expect(screen.getByText('Applied')).toBeInTheDocument()
    expect(screen.getByText('Remote', { selector: '[data-slot="badge"]' })).toBeInTheDocument()
    expect(screen.getByText('e')).toBeInTheDocument()
    expect(screen.queryByText('f')).not.toBeInTheDocument()
    expect(screen.getByText('+2')).toBeInTheDocument()
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
  })

  it('shows a match ring and accent for strong matches', async () => {
    await renderWithProviders(
      <ul>
        <JobCard
          job={makeJobListItem({
            match_percent: 90,
            salary_min: null,
            salary_max: null,
            posted_at: undefined,
            location: ''
          })}
        />
      </ul>
    )

    expect(screen.getByRole('img', { name: /90% match/ })).toBeInTheDocument()
    expect(screen.getByRole('link').className).toContain('shadow-[inset_2px_0_0_var(--primary)]')
    expect(screen.queryByText(/\$/)).not.toBeInTheDocument()
  })
})
