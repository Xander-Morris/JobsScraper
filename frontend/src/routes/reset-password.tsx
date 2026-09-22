import { useConfirmPasswordResetMutation } from '@/src/hooks/use-profile'
import { Button } from '@/src/components/ui/button'
import { Input } from '@/src/components/ui/input'
import { Label } from '@/src/components/ui/label'
import { useAuth } from '@/src/stores/auth-store'
import { createFileRoute, Link } from '@tanstack/react-router'
import { useId, useState, type SubmitEvent } from 'react'
import { z } from 'zod'

export const Route = createFileRoute('/reset-password')({
  validateSearch: z.object({ token: z.string().catch('') }),
  component: ResetPasswordPage
})

function ResetPasswordPage() {
  const { token } = Route.useSearch()
  const { login } = useAuth()
  const navigate = Route.useNavigate()
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const passwordId = useId()
  const confirmId = useId()
  const mutation = useConfirmPasswordResetMutation()

  if (!token) {
    return (
      <div className="mx-auto mt-10 max-w-sm text-left">
        <p role="alert" className="text-sm text-muted-foreground">
          This reset link is incomplete. Request a new one from the log in page.
        </p>
        <Link to="/profile" className="mt-4 inline-block text-sm">
          Go to log in
        </Link>
      </div>
    )
  }

  const mismatch = confirm !== '' && password !== confirm

  function handleSubmit(e: SubmitEvent) {
    e.preventDefault()
    if (password !== confirm) return

    mutation.mutate(
      { token, password },
      {
        onSuccess: ({ token: accessToken }) => {
          login(accessToken)
          void navigate({ to: '/profile' })
        }
      }
    )
  }

  const error =
    mutation.error instanceof Error ? mutation.error.message : mutation.error ? 'Something went wrong' : null

  return (
    <div className="mx-auto mt-10 max-w-sm text-left">
      <h2 className="mb-4 text-base font-semibold text-heading">Set a new password</h2>
      <form onSubmit={handleSubmit} className="space-y-3">
        <div className="space-y-1.5">
          <Label htmlFor={passwordId} className="sr-only">
            New password
          </Label>
          <Input
            id={passwordId}
            type="password"
            required
            minLength={8}
            autoComplete="new-password"
            placeholder="New password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor={confirmId} className="sr-only">
            Confirm new password
          </Label>
          <Input
            id={confirmId}
            type="password"
            required
            autoComplete="new-password"
            placeholder="Confirm new password"
            aria-invalid={mismatch}
            value={confirm}
            onChange={(e) => setConfirm(e.target.value)}
          />
        </div>
        {mismatch && (
          <p role="alert" className="text-sm text-destructive">
            Passwords don't match
          </p>
        )}
        {error && (
          <p role="alert" className="text-sm text-destructive">
            {error}{' '}
            <Link to="/profile" className="underline">
              Request a new link
            </Link>
          </p>
        )}
        <Button type="submit" disabled={mutation.isPending || mismatch}>
          Reset password
        </Button>
      </form>
    </div>
  )
}
