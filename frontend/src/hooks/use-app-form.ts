import { createFormHook } from '@tanstack/react-form'
import { Form, SubmitButton, TextField } from '@/src/components/form'
import { fieldContext, formContext } from '@/src/lib/form-context'

export const { useAppForm } = createFormHook({
  fieldContext,
  formContext,
  fieldComponents: { TextField },
  formComponents: { Form, SubmitButton }
})
