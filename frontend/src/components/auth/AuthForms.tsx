import { useCreateProfileMutation, useLoginMutation, useRequestPasswordResetMutation } from '@/src/api/profile'
import { Button } from '@/src/components/ui/button'
import { Input } from '@/src/components/ui/input'
import { Label } from '@/src/components/ui/label'
import { useAuth } from '@/src/stores/profile-store'
import { useId, useState, type SubmitEvent } from 'react'

export function AuthForms({ prompt }: { prompt?: string } = {}) {
  const { login, sessionMessage } = useAuth()
  const [mode, setMode] = useState<'login' | 'signup' | 'forgot'>('login')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const emailId = useId()
  const passwordId = useId()

  const loginMutation = useLoginMutation()
  const createMutation = useCreateProfileMutation()
  const mutation = mode === 'login' ? loginMutation : createMutation

  if (mode === 'forgot') {
    return <ForgotPasswordForm initialEmail={email} onBack={() => setMode('login')} />
  }

  function handleSubmit(e: SubmitEvent) {
    e.preventDefault()
    mutation.mutate({ email, password }, { onSuccess: ({ token }) => login(token) })
  }

  const error =
    mutation.error instanceof Error ? mutation.error.message : mutation.error ? 'Something went wrong' : null

  return (
    <div className="mx-auto mt-10 max-w-sm text-left">
      {sessionMessage ? (
        <p role="alert" className="mb-4 text-sm text-muted-foreground">
          {sessionMessage}
        </p>
      ) : prompt ? (
        <p className="mb-4 text-sm text-muted-foreground">{prompt}</p>
      ) : null}
      <div role="group" aria-label="Authentication mode" className="mb-4 flex gap-2 text-sm">
        <Button
          type="button"
          variant={mode === 'login' ? 'secondary' : 'ghost'}
          size="sm"
          aria-pressed={mode === 'login'}
          onClick={() => setMode('login')}
        >
          Log in
        </Button>
        <Button
          type="button"
          variant={mode === 'signup' ? 'secondary' : 'ghost'}
          size="sm"
          aria-pressed={mode === 'signup'}
          onClick={() => setMode('signup')}
        >
          Sign up
        </Button>
      </div>
      <form onSubmit={handleSubmit} className="space-y-3">
        <div className="space-y-1.5">
          <Label htmlFor={emailId} className="sr-only">
            Email
          </Label>
          <Input
            id={emailId}
            type="email"
            required
            autoComplete="email"
            placeholder="Email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor={passwordId} className="sr-only">
            Password
          </Label>
          <Input
            id={passwordId}
            type="password"
            required
            minLength={mode === 'signup' ? 8 : undefined}
            autoComplete={mode === 'login' ? 'current-password' : 'new-password'}
            placeholder="Password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
        </div>
        {error && (
          <p role="alert" className="text-sm text-destructive">
            {error}
          </p>
        )}
        <div className="flex items-center justify-between">
          <Button type="submit" disabled={mutation.isPending}>
            {mode === 'login' ? 'Log in' : 'Sign up'}
          </Button>
          {mode === 'login' && (
            <Button type="button" variant="ghost" size="sm" onClick={() => setMode('forgot')}>
              Forgot password?
            </Button>
          )}
        </div>
      </form>
    </div>
  )
}

function ForgotPasswordForm({ initialEmail, onBack }: { initialEmail: string; onBack: () => void }) {
  const [email, setEmail] = useState(initialEmail)
  const emailId = useId()
  const mutation = useRequestPasswordResetMutation()

  function handleSubmit(e: SubmitEvent) {
    e.preventDefault()
    mutation.mutate(email)
  }

  const error =
    mutation.error instanceof Error ? mutation.error.message : mutation.error ? 'Something went wrong' : null

  return (
    <div className="mx-auto mt-10 max-w-sm text-left">
      <h2 className="mb-2 text-base font-semibold text-heading">Reset your password</h2>
      {mutation.isSuccess ? (
        <p role="status" className="text-sm text-muted-foreground">
          If an account exists for {email}, a reset link is on its way. It expires in 1 hour.
        </p>
      ) : (
        <form onSubmit={handleSubmit} className="space-y-3">
          <p className="text-sm text-muted-foreground pb-2">
            Enter your account email and we'll send you a link to set a new password.
          </p>
          <div className="space-y-1.5">
            <Label htmlFor={emailId} className="sr-only">
              Email
            </Label>
            <Input
              id={emailId}
              type="email"
              required
              autoComplete="email"
              placeholder="Email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
            />
          </div>
          {error && (
            <p role="alert" className="text-sm text-destructive">
              {error}
            </p>
          )}
          <Button type="submit" disabled={mutation.isPending}>
            Send reset link
          </Button>
        </form>
      )}
      <Button type="button" variant="ghost" size="sm" className="mt-4" onClick={onBack}>
        Back to log in
      </Button>
    </div>
  )
}
