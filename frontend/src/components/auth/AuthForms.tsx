import { useCreateProfileMutation, useLoginMutation, useRequestPasswordResetMutation } from '@/src/hooks/use-profile'
import { useAppForm } from '@/src/hooks/use-app-form'
import { Button } from '@/src/components/ui/button'
import { errorMessage } from '@/src/lib/utils'
import { useAuth } from '@/src/stores/auth-store'
import { revalidateLogic } from '@tanstack/react-form'
import { useState } from 'react'
import { z } from 'zod'

const email = z.email('Enter a valid email')

const loginSchema = z.object({ email, password: z.string().min(1, 'Required') })
const signupSchema = z.object({ email, password: z.string().min(8, 'At least 8 characters') })

export function AuthForms({ prompt }: { prompt?: string } = {}) {
  const { login, sessionMessage } = useAuth()
  const [mode, setMode] = useState<'login' | 'signup' | 'forgot'>('login')

  const loginMutation = useLoginMutation()
  const createMutation = useCreateProfileMutation()
  const mutation = mode === 'login' ? loginMutation : createMutation

  const form = useAppForm({
    defaultValues: { email: '', password: '' },
    validationLogic: revalidateLogic(),
    validators: { onDynamic: mode === 'signup' ? signupSchema : loginSchema },
    onSubmit: async ({ value }) => {
      const { token } = await mutation.mutateAsync(value)
      login(token)
    }
  })

  if (mode === 'forgot') {
    return <ForgotPasswordForm initialEmail={form.state.values.email} onBack={() => setMode('login')} />
  }

  const error = errorMessage(mutation.error)

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
      <form.AppForm>
        <form.Form className="space-y-3">
          <form.AppField name="email">
            {(f) => <f.TextField label="Email" srOnlyLabel type="email" autoComplete="email" />}
          </form.AppField>
          <form.AppField name="password">
            {(f) => (
              <f.TextField
                label="Password"
                srOnlyLabel
                type="password"
                autoComplete={mode === 'login' ? 'current-password' : 'new-password'}
              />
            )}
          </form.AppField>
          {error && (
            <p role="alert" className="text-sm text-destructive">
              {error}
            </p>
          )}
          <div className="flex items-center justify-between">
            <form.SubmitButton>{mode === 'login' ? 'Log in' : 'Sign up'}</form.SubmitButton>
            {mode === 'login' && (
              <Button type="button" variant="ghost" size="sm" onClick={() => setMode('forgot')}>
                Forgot password?
              </Button>
            )}
          </div>
        </form.Form>
      </form.AppForm>
    </div>
  )
}

function ForgotPasswordForm({ initialEmail, onBack }: { initialEmail: string; onBack: () => void }) {
  const mutation = useRequestPasswordResetMutation()

  const form = useAppForm({
    defaultValues: { email: initialEmail },
    validationLogic: revalidateLogic(),
    validators: { onDynamic: z.object({ email }) },
    onSubmit: async ({ value }) => {
      await mutation.mutateAsync(value.email)
    }
  })

  const error = errorMessage(mutation.error)

  return (
    <div className="mx-auto mt-10 max-w-sm text-left">
      <h2 className="mb-2 text-base font-semibold text-heading">Reset your password</h2>
      {mutation.isSuccess ? (
        <p role="status" className="text-sm text-muted-foreground">
          If an account exists for {mutation.variables}, a reset link is on its way. It expires in 1 hour.
        </p>
      ) : (
        <form.AppForm>
          <form.Form className="space-y-3">
            <p className="text-sm text-muted-foreground pb-2">
              Enter your account email and we'll send you a link to set a new password.
            </p>
            <form.AppField name="email">
              {(f) => <f.TextField label="Email" srOnlyLabel type="email" autoComplete="email" />}
            </form.AppField>
            {error && (
              <p role="alert" className="text-sm text-destructive">
                {error}
              </p>
            )}
            <form.SubmitButton>Send reset link</form.SubmitButton>
          </form.Form>
        </form.AppForm>
      )}
      <Button type="button" variant="ghost" size="sm" className="mt-4" onClick={onBack}>
        Back to log in
      </Button>
    </div>
  )
}
