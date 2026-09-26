import { AuthPanel } from '@/src/components/auth/AuthForms'
import { useConfirmPasswordResetMutation } from '@/src/hooks/use-profile'
import { useAppForm } from '@/src/hooks/use-app-form'
import { errorMessage } from '@/src/lib/utils'
import { useAuth } from '@/src/stores/auth-store'
import { revalidateLogic } from '@tanstack/react-form'
import { createFileRoute, Link } from '@tanstack/react-router'
import { z } from 'zod'

export const Route = createFileRoute('/reset-password')({
  validateSearch: z.object({ token: z.string().catch('') }),
  component: ResetPasswordPage
})

const resetPasswordSchema = z
  .object({ password: z.string().min(8, 'At least 8 characters'), confirm: z.string() })
  .refine((v) => v.password === v.confirm, { message: "Passwords don't match", path: ['confirm'] })

function ResetPasswordPage() {
  const { token } = Route.useSearch()
  const { login } = useAuth()
  const navigate = Route.useNavigate()
  const mutation = useConfirmPasswordResetMutation()

  const form = useAppForm({
    defaultValues: { password: '', confirm: '' },
    validationLogic: revalidateLogic(),
    validators: { onDynamic: resetPasswordSchema },
    onSubmit: async ({ value }) => {
      const { token: accessToken } = await mutation.mutateAsync({ token, password: value.password })
      login(accessToken)
      void navigate({ to: '/profile' })
    }
  })

  if (!token) {
    return (
      <AuthPanel className="mt-20">
        <p role="alert" className="text-sm text-muted-foreground">
          This reset link is incomplete. Request a new one from the log in page.
        </p>
        <Link to="/profile" className="mt-4 inline-block text-sm text-primary hover:underline">
          Go to log in
        </Link>
      </AuthPanel>
    )
  }

  const error = errorMessage(mutation.error)

  return (
    <AuthPanel className="mt-20">
      <h1 className="mb-4 text-lg font-semibold tracking-tight">Set a new password</h1>
      <form.AppForm>
        <form.Form className="space-y-3">
          <form.AppField name="password">
            {(f) => <f.TextField label="New password" srOnlyLabel type="password" autoComplete="new-password" />}
          </form.AppField>
          <form.AppField name="confirm">
            {(f) => (
              <f.TextField label="Confirm new password" srOnlyLabel type="password" autoComplete="new-password" />
            )}
          </form.AppField>
          {error && (
            <p role="alert" className="text-sm text-destructive">
              {error}{' '}
              <Link to="/profile" className="underline">
                Request a new link
              </Link>
            </p>
          )}
          <form.SubmitButton size="lg" className="w-full">
            Reset password
          </form.SubmitButton>
        </form.Form>
      </form.AppForm>
    </AuthPanel>
  )
}
