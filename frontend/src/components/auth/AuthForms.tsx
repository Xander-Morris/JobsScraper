import { useCreateProfileMutation, useLoginMutation } from '@/src/api/profile'
import { Button } from '@/src/components/ui/button'
import { Input } from '@/src/components/ui/input'
import { Label } from '@/src/components/ui/label'
import { useAuth } from '@/src/stores/profile-store'
import { useId, useState, type SubmitEvent } from 'react'

export function AuthForms({ prompt }: { prompt?: string } = {}) {
  const { login, sessionMessage } = useAuth()
  const [mode, setMode] = useState<'login' | 'signup'>('login')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const emailId = useId()
  const passwordId = useId()

  const loginMutation = useLoginMutation()
  const createMutation = useCreateProfileMutation()
  const mutation = mode === 'login' ? loginMutation : createMutation

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
        <Button type="submit" disabled={mutation.isPending}>
          {mode === 'login' ? 'Log in' : 'Sign up'}
        </Button>
      </form>
    </div>
  )
}
