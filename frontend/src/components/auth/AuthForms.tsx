import { useCreateProfileMutation, useLoginMutation, useRequestPasswordResetMutation } from '@/src/hooks/use-profile'
import { useAppForm } from '@/src/hooks/use-app-form'
import { Button } from '@/src/components/ui/button'
import { cn, errorMessage } from '@/src/lib/utils'
import { useAuth } from '@/src/stores/auth-store'
import { revalidateLogic } from '@tanstack/react-form'
import { useState, type ReactNode } from 'react'
import { z } from 'zod'

const email = z.email('Enter a valid email')

const loginSchema = z.object({ email, password: z.string().min(1, 'Required') })
const signupSchema = z.object({ email, password: z.string().min(8, 'At least 8 characters') })

export function AuthPanel({ className, children }: { className?: string; children: ReactNode }) {
  return (
    <div
      className={cn(
        'mx-auto w-full max-w-sm rounded-2xl border border-border bg-card/80 p-6 text-left shadow-[0_24px_60px_-24px_rgb(0_0_0/0.8)] backdrop-blur-sm',
        className
      )}
    >
      {children}
    </div>
  )
}

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

  const error = errorMessage(mutation.error)
  const note = sessionMessage ?? prompt

  return (
    <section className="px-4 pt-16 pb-20 text-center sm:pt-24">
      <p className="inline-flex items-center gap-2 rounded-full border border-accent-border bg-accent-bg px-3 py-1 text-xs text-primary">
        <span aria-hidden="true" className="size-1.5 rounded-full bg-primary shadow-[0_0_8px_var(--primary)]" />
        Daily digest of new matches
      </p>
      <h1 className="mx-auto mt-6 max-w-3xl text-4xl font-[450] tracking-[-0.04em] sm:text-6xl sm:leading-[1.05]">
        Find the jobs that fit your resume
      </h1>
      <p className="mx-auto mt-5 max-w-lg text-base text-muted-foreground sm:text-lg">
        Listings from public job boards and company career pages, ranked by how well they match your skills.
      </p>

      <AuthPanel className="mt-10">
        {mode === 'forgot' ? (
          <ForgotPasswordForm initialEmail={form.state.values.email} onBack={() => setMode('login')} />
        ) : (
          <>
            {note && (
              <p role={sessionMessage ? 'alert' : undefined} className="mb-4 text-sm text-muted-foreground">
                {note}
              </p>
            )}
            <div
              role="group"
              aria-label="Authentication mode"
              className="mb-5 grid grid-cols-2 gap-1 rounded-lg border border-border bg-black/40 p-1 text-sm"
            >
              {(['login', 'signup'] as const).map((m) => (
                <button
                  key={m}
                  type="button"
                  aria-pressed={mode === m}
                  onClick={() => setMode(m)}
                  className="h-8 rounded-md font-medium text-muted-foreground/80 transition-colors outline-none hover:text-heading focus-visible:ring-2 focus-visible:ring-ring aria-pressed:bg-accent-bg aria-pressed:text-primary aria-pressed:ring-1 aria-pressed:ring-accent-border"
                >
                  {m === 'login' ? 'Log in' : 'Sign up'}
                </button>
              ))}
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
                <form.SubmitButton size="lg" className="w-full">
                  {mode === 'login' ? 'Log in' : 'Create account'}
                </form.SubmitButton>
                {mode === 'login' && (
                  <button
                    type="button"
                    onClick={() => setMode('forgot')}
                    className="block w-full pt-1 text-center text-xs text-muted-foreground hover:text-heading"
                  >
                    Forgot password?
                  </button>
                )}
              </form.Form>
            </form.AppForm>
          </>
        )}
      </AuthPanel>
    </section>
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
    <div>
      <h2 className="mb-2 text-base font-semibold">Reset your password</h2>
      {mutation.isSuccess ? (
        <p role="status" className="text-sm text-muted-foreground">
          If an account exists for {mutation.variables}, a reset link is on its way. It expires in 1 hour.
        </p>
      ) : (
        <form.AppForm>
          <form.Form className="space-y-3">
            <p className="pb-2 text-sm text-muted-foreground">
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
            <form.SubmitButton size="lg" className="w-full">
              Send reset link
            </form.SubmitButton>
          </form.Form>
        </form.AppForm>
      )}
      <Button type="button" variant="ghost" size="sm" className="mt-4 -ml-2.5" onClick={onBack}>
        Back to log in
      </Button>
    </div>
  )
}
