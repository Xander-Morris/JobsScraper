import { Button } from '@/src/components/ui/button'
import { Input } from '@/src/components/ui/input'
import { Label } from '@/src/components/ui/label'
import { useFieldContext, useFormContext } from '@/src/lib/form-context'
import { cn } from '@/src/lib/utils'
import { useId, type ComponentProps } from 'react'

export function Form({ children, ...props }: Omit<ComponentProps<'form'>, 'onSubmit'>) {
  const form = useFormContext()

  return (
    <form
      noValidate
      onSubmit={(e) => {
        e.preventDefault()
        // request errors surface via mutation.error
        form.handleSubmit().catch(() => {})
      }}
      {...props}
    >
      {children}
    </form>
  )
}

type TextFieldProps = {
  label: string
  srOnlyLabel?: boolean
} & Omit<ComponentProps<'input'>, 'id' | 'name' | 'value' | 'onChange' | 'onBlur'>

export function TextField({ label, srOnlyLabel, className, ...props }: TextFieldProps) {
  const field = useFieldContext<string>()
  const id = useId()
  const error = (field.state.meta.errors[0] as { message?: string } | undefined)?.message

  return (
    <div className={cn('space-y-1.5', className)}>
      <Label htmlFor={id} className={srOnlyLabel ? 'sr-only' : undefined}>
        {label}
      </Label>
      <Input
        id={id}
        name={field.name}
        placeholder={srOnlyLabel ? label : undefined}
        value={field.state.value}
        onChange={(e) => field.handleChange(e.target.value)}
        onBlur={field.handleBlur}
        aria-invalid={!!error}
        aria-describedby={error ? `${id}-error` : undefined}
        {...props}
      />
      {error && (
        <p id={`${id}-error`} className="text-xs text-destructive">
          {error}
        </p>
      )}
    </div>
  )
}

export function SubmitButton({ disabled, ...props }: ComponentProps<typeof Button>) {
  const form = useFormContext()

  return (
    <form.Subscribe selector={(state) => state.isSubmitting}>
      {(isSubmitting) => <Button type="submit" disabled={disabled || isSubmitting} {...props} />}
    </form.Subscribe>
  )
}
