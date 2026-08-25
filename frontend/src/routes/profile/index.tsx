import { createFileRoute } from '@tanstack/react-router'
import { useId, useState, type FormEvent } from 'react'
import { createProfile, loginProfile } from '@/src/api/profile'
import ProfileView from '@/src/components/profile/profile-view'
import { Button } from '@/src/components/ui/button'
import { Input } from '@/src/components/ui/input'
import { Label } from '@/src/components/ui/label'
import { useAuth } from '@/src/stores/profile-store'

export const Route = createFileRoute('/profile/')({ component: RouteComponent })

function RouteComponent() {
  const { token, isAuthenticated } = useAuth()
  return isAuthenticated ? <ProfileView token={token!} /> : <AuthForms />
}

function AuthForms() {
  const { login, sessionMessage } = useAuth()
  const [mode, setMode] = useState<'login' | 'signup'>('login')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const emailId = useId()
  const passwordId = useId()

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    setSubmitting(true)
    try {
      const { token } = mode === 'login' ? await loginProfile(email, password) : await createProfile(email, password)
      login(token)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Something went wrong')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="mx-auto mt-10 max-w-sm text-left">
      {sessionMessage && <p role="alert" className="mb-4 text-sm text-muted-foreground">{sessionMessage}</p>}
      <div role="group" aria-label="Authentication mode" className="mb-4 flex gap-2 text-sm">
        <Button type="button" variant={mode === 'login' ? 'secondary' : 'ghost'} size="sm" aria-pressed={mode === 'login'} onClick={() => setMode('login')}>Log in</Button>
        <Button type="button" variant={mode === 'signup' ? 'secondary' : 'ghost'} size="sm" aria-pressed={mode === 'signup'} onClick={() => setMode('signup')}>Sign up</Button>
      </div>
      <form onSubmit={handleSubmit} className="space-y-3">
        <div className="space-y-1.5">
          <Label htmlFor={emailId} className="sr-only">Email</Label>
          <Input id={emailId} type="email" required autoComplete="email" placeholder="Email" value={email} onChange={(e) => setEmail(e.target.value)} />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor={passwordId} className="sr-only">Password</Label>
          <Input id={passwordId} type="password" required autoComplete={mode === 'login' ? 'current-password' : 'new-password'} placeholder="Password" value={password} onChange={(e) => setPassword(e.target.value)} />
        </div>
        {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
        <Button type="submit" disabled={submitting}>{mode === 'login' ? 'Log in' : 'Sign up'}</Button>
      </form>
    </div>
  )
}
