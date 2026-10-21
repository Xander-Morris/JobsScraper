import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it, vi } from 'vitest'
import { auth } from '@/src/test/auth-mock'
import { renderWithProviders } from '@/src/test/render'
import { api, server } from '@/src/test/server'
import { AuthForms } from '@/src/components/auth/AuthForms'

vi.mock('@/src/stores/auth-store', () => import('@/src/test/auth-mock'))

const emailBox = () => screen.getByRole('textbox', { name: 'Email' })
const passwordBox = () => screen.getByLabelText('Password')
// The mode toggle is also labelled "Log in"; the submit button comes last.
const submitLogin = () => screen.getAllByRole('button', { name: 'Log in' }).at(-1)!

describe('AuthForms', () => {
  it('logs in and remembers the choice', async () => {
    let body: unknown
    server.use(
      http.post(api('/api/profile/login'), async ({ request }) => {
        body = await request.json()
        return HttpResponse.json({ token: 'tok' }, { status: 202 })
      })
    )
    await renderWithProviders(<AuthForms />)

    await userEvent.type(emailBox(), 'ada@example.com')
    await userEvent.type(passwordBox(), 'pw')
    await userEvent.click(screen.getByRole('checkbox', { name: 'Remember me' }))
    await userEvent.click(submitLogin())

    await waitFor(() => expect(auth.login).toHaveBeenCalledWith('tok', true))
    expect(body).toEqual({ email: 'ada@example.com', password: 'pw', remember_me: true })
  })

  it('validates before submitting', async () => {
    const login = vi.fn()
    server.use(http.post(api('/api/profile/login'), login))
    await renderWithProviders(<AuthForms />)

    await userEvent.type(emailBox(), 'not-an-email')
    await userEvent.click(submitLogin())

    expect(await screen.findByText('Enter a valid email')).toBeInTheDocument()
    expect(screen.getByText('Required')).toBeInTheDocument()
    expect(emailBox()).toHaveAttribute('aria-invalid', 'true')
    expect(login).not.toHaveBeenCalled()
  })

  it('shows the server error', async () => {
    server.use(
      http.post(api('/api/profile/login'), () =>
        HttpResponse.json({ error: 'invalid email or password' }, { status: 401 })
      )
    )
    await renderWithProviders(<AuthForms />)

    await userEvent.type(emailBox(), 'ada@example.com')
    await userEvent.type(passwordBox(), 'wrong')
    await userEvent.click(submitLogin())

    expect(await screen.findByRole('alert')).toHaveTextContent('invalid email or password')
    expect(auth.login).not.toHaveBeenCalled()
  })

  it('signs up with the longer password rule and always remembers', async () => {
    server.use(http.post(api('/api/profile/create'), () => HttpResponse.json({ token: 'new' }, { status: 201 })))
    await renderWithProviders(<AuthForms />)

    await userEvent.click(screen.getByRole('button', { name: 'Sign up' }))
    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument()

    await userEvent.type(emailBox(), 'ada@example.com')
    await userEvent.type(passwordBox(), 'short')
    await userEvent.click(screen.getByRole('button', { name: 'Create account' }))
    expect(await screen.findByText('At least 8 characters')).toBeInTheDocument()

    await userEvent.type(passwordBox(), 'enough')
    await userEvent.click(screen.getByRole('button', { name: 'Create account' }))

    await waitFor(() => expect(auth.login).toHaveBeenCalledWith('new', true))
  })

  it('shows the prompt, or the session message as an alert', async () => {
    const { unmount } = await renderWithProviders(<AuthForms prompt="Log in to see matches" />)
    expect(screen.getByText('Log in to see matches')).not.toHaveAttribute('role')
    unmount()

    auth.sessionMessage = 'Your session expired'
    await renderWithProviders(<AuthForms prompt="Log in to see matches" />)
    expect(screen.getByRole('alert')).toHaveTextContent('Your session expired')
    expect(screen.queryByText('Log in to see matches')).not.toBeInTheDocument()
  })

  it('requests a password reset', async () => {
    let body: unknown
    server.use(
      http.post(api('/api/profile/password-reset'), async ({ request }) => {
        body = await request.json()
        return HttpResponse.json({ status: 'sent' }, { status: 202 })
      })
    )
    await renderWithProviders(<AuthForms />)

    await userEvent.type(emailBox(), 'ada@example.com')
    await userEvent.click(screen.getByRole('button', { name: 'Forgot password?' }))

    expect(screen.getByRole('heading', { name: 'Reset your password' })).toBeInTheDocument()
    expect(emailBox()).toHaveValue('ada@example.com')

    await userEvent.click(screen.getByRole('button', { name: 'Send reset link' }))

    expect(await screen.findByRole('status')).toHaveTextContent('If an account exists for ada@example.com')
    expect(body).toEqual({ email: 'ada@example.com' })

    await userEvent.click(screen.getByRole('button', { name: 'Back to log in' }))
    expect(screen.getByRole('button', { name: 'Forgot password?' })).toBeInTheDocument()
  })

  it('validates and reports errors on the reset form', async () => {
    server.use(
      http.post(api('/api/profile/password-reset'), () => HttpResponse.json({ error: 'slow down' }, { status: 429 }))
    )
    await renderWithProviders(<AuthForms />)

    await userEvent.click(screen.getByRole('button', { name: 'Forgot password?' }))
    await userEvent.click(screen.getByRole('button', { name: 'Send reset link' }))
    expect(await screen.findByText('Enter a valid email')).toBeInTheDocument()

    await userEvent.type(emailBox(), 'ada@example.com')
    await userEvent.click(screen.getByRole('button', { name: 'Send reset link' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('slow down')
  })
})
