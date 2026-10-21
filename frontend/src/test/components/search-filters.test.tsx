import { act, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { useState } from 'react'
import { describe, expect, it, vi } from 'vitest'
import type { JobSearchState } from '@/src/lib/job-search'
import { renderWithProviders } from '@/src/test/render'
import { api, server } from '@/src/test/server'
import { SearchFilters } from '@/src/components/search-filters'

async function renderFilters(search: JobSearchState = {}) {
  const onChange = vi.fn()
  const view = await renderWithProviders(<SearchFilters search={search} onChange={onChange} />)
  return { onChange, ...view }
}

describe('SearchFilters', () => {
  it('debounces the search box', async () => {
    const { onChange } = await renderFilters({ tags: ['go'] })

    await userEvent.type(screen.getByRole('searchbox', { name: 'Search jobs' }), 'rust')
    expect(onChange).not.toHaveBeenCalled()

    await waitFor(() => expect(onChange).toHaveBeenCalledWith({ tags: ['go'], q: 'rust' }), { timeout: 1000 })
    expect(onChange).toHaveBeenCalledOnce()
  })

  it('clears q when the box is emptied', async () => {
    const { onChange } = await renderFilters({ q: 'go' })

    await userEvent.clear(screen.getByRole('searchbox'))

    await waitFor(() => expect(onChange).toHaveBeenCalledWith({ q: undefined }), { timeout: 1000 })
  })

  it('syncs the box when the search changes from outside', async () => {
    function Harness() {
      const [search, setSearch] = useState<JobSearchState>({ q: 'go' })
      return (
        <>
          <button type="button" onClick={() => setSearch({ q: 'rust' })}>
            back
          </button>
          <SearchFilters search={search} onChange={setSearch} />
        </>
      )
    }
    await renderWithProviders(<Harness />)
    expect(screen.getByRole('searchbox')).toHaveValue('go')

    await userEvent.click(screen.getByRole('button', { name: 'back' }))
    expect(screen.getByRole('searchbox')).toHaveValue('rust')
  })

  it('picks a dropdown option', async () => {
    const { onChange } = await renderFilters({ q: 'go' })

    await userEvent.click(screen.getByRole('button', { name: /Any workplace/ }))
    await userEvent.click(await screen.findByRole('menuitem', { name: 'Remote' }))

    expect(onChange).toHaveBeenCalledWith({ q: 'go', workplaceType: 'remote' })
  })

  it('picks job type, date and sort options', async () => {
    const { onChange } = await renderFilters({ workplaceType: 'remote' })

    await userEvent.click(screen.getByRole('button', { name: /Any job type/ }))
    await userEvent.click(await screen.findByRole('menuitem', { name: 'Intern' }))
    expect(onChange).toHaveBeenLastCalledWith({ workplaceType: 'remote', jobType: 'intern' })

    await userEvent.click(screen.getByRole('button', { name: /Any time/ }))
    await userEvent.click(await screen.findByRole('menuitem', { name: 'Past week' }))
    expect(onChange).toHaveBeenLastCalledWith({ workplaceType: 'remote', datePosted: 'week' })

    await userEvent.click(screen.getByRole('button', { name: /Sort: relevance/ }))
    await userEvent.click(await screen.findByRole('menuitem', { name: 'Sort: newest' }))
    expect(onChange).toHaveBeenLastCalledWith({ workplaceType: 'remote', sort: 'date' })
  })

  it('labels the current selections', async () => {
    await renderFilters({ workplaceType: 'remote', jobType: 'full_time', datePosted: '24h', sort: 'date' })

    expect(screen.getByRole('button', { name: /^Remote/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Full-Time/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Past 24 hours/ })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Sort: newest/ })).toBeInTheDocument()
  })

  it('shows the salary range', async () => {
    await renderFilters({ minSalary: 50000, maxSalary: 100000 })
    expect(screen.getByText('$50k – $100k')).toBeInTheDocument()
  })

  it('shows an open-ended salary range by default', async () => {
    await renderFilters()
    expect(screen.getByText(/\$20k – \$500k\+/)).toBeInTheDocument()
  })

  it('filters tags as you type and toggles them', async () => {
    const { onChange } = await renderFilters({ tags: ['go'] })
    const combobox = await screen.findByRole('combobox', { name: 'Filter tags' })

    expect(screen.getByRole('button', { name: 'Remove go filter' })).toBeInTheDocument()

    await userEvent.click(combobox)
    expect(screen.getByRole('listbox', { name: 'Matching tags' })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'go' })).toHaveAttribute('aria-selected', 'true')

    await userEvent.type(combobox, 'rea')
    expect(screen.getAllByRole('option').map((o) => o.textContent)).toEqual(['react'])

    await userEvent.click(screen.getByRole('option', { name: 'react' }))
    expect(onChange).toHaveBeenCalledWith({ tags: ['go', 'react'] })

    await userEvent.clear(combobox)
    await userEvent.type(combobox, 'zzz')
    expect(screen.getByText('No matching tags.')).toBeInTheDocument()

    await userEvent.keyboard('{Escape}')
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
  })

  it('removes the last tag filter', async () => {
    const { onChange } = await renderFilters({ tags: ['go'] })

    await userEvent.click(await screen.findByRole('button', { name: 'Remove go filter' }))

    expect(onChange).toHaveBeenCalledWith({ tags: undefined })
  })

  it('caps the visible tag matches', async () => {
    server.use(http.get(api('/api/tags'), () => HttpResponse.json(Array.from({ length: 45 }, (_, i) => `tag${i}`))))
    await renderFilters()

    await userEvent.click(await screen.findByRole('combobox'))

    expect(screen.getAllByRole('option')).toHaveLength(40)
    expect(screen.getByText('+5 more, keep typing to narrow down')).toBeInTheDocument()
  })

  it('closes the tag menu on blur', async () => {
    await renderFilters()
    await userEvent.click(await screen.findByRole('combobox'))
    expect(screen.getByRole('listbox')).toBeInTheDocument()

    await userEvent.click(document.body)
    await act(() => new Promise((r) => setTimeout(r, 200)))
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
  })

  it('resets dropdowns to their any option', async () => {
    const { onChange } = await renderFilters({ workplaceType: 'remote', jobType: 'intern', datePosted: 'week' })

    await userEvent.click(screen.getByRole('button', { name: /^Remote/ }))
    await userEvent.click(await screen.findByRole('menuitem', { name: 'Any workplace' }))
    expect(onChange).toHaveBeenLastCalledWith({ workplaceType: undefined, jobType: 'intern', datePosted: 'week' })

    await userEvent.click(screen.getByRole('button', { name: /Intern/ }))
    await userEvent.click(await screen.findByRole('menuitem', { name: 'Any job type' }))
    expect(onChange).toHaveBeenLastCalledWith({ workplaceType: 'remote', jobType: undefined, datePosted: 'week' })

    await userEvent.click(screen.getByRole('button', { name: /Past week/ }))
    await userEvent.click(await screen.findByRole('menuitem', { name: 'Any time' }))
    expect(onChange).toHaveBeenLastCalledWith({ workplaceType: 'remote', jobType: 'intern', datePosted: undefined })
  })

  it('commits salary changes from the slider', async () => {
    const { onChange } = await renderFilters({ minSalary: 50000 })
    const [minThumb, maxThumb] = screen.getAllByRole('slider')

    act(() => minThumb.focus())
    await userEvent.keyboard('{Home}')
    expect(onChange).toHaveBeenLastCalledWith({ minSalary: undefined, maxSalary: undefined })

    act(() => minThumb.focus())
    await userEvent.keyboard('{ArrowRight}')
    expect(onChange).toHaveBeenLastCalledWith({ minSalary: 25000, maxSalary: undefined })

    act(() => maxThumb.focus())
    await userEvent.keyboard('{ArrowLeft}')
    expect(onChange).toHaveBeenLastCalledWith({ minSalary: 25000, maxSalary: 495000 })
  })

  it('empties the box when the search q is cleared from outside', async () => {
    function Harness() {
      const [search, setSearch] = useState<JobSearchState>({ q: 'go' })
      return (
        <>
          <button type="button" onClick={() => setSearch({})}>
            back
          </button>
          <SearchFilters search={search} onChange={setSearch} />
        </>
      )
    }
    await renderWithProviders(<Harness />)

    await userEvent.click(screen.getByRole('button', { name: 'back' }))
    expect(screen.getByRole('searchbox')).toHaveValue('')
  })
})
