import { CheckIcon, ClipboardIcon } from 'lucide-react'
import useCopied from '@/src/hooks/use-copied'
import { Button } from '../../ui/button'
import SectionHeading from './section-heading'

export type ContactField = { label: string; value: string }

// Click a row to copy just that value, instead of one blob to paste and edit apart.
function CopyField({ label, value }: ContactField) {
  const { copied, copy } = useCopied()

  return (
    <button
      type="button"
      onClick={() => void copy(value)}
      aria-label={`Copy ${label}`}
      className="flex w-full items-center justify-between gap-3 rounded-md px-2 py-1 text-left transition-colors hover:bg-muted"
    >
      <span className="min-w-0">
        <span className="block text-[0.7rem] uppercase tracking-wide text-muted-foreground">{label}</span>
        <span className="block truncate text-xs">{value}</span>
      </span>
      {copied ? (
        <CheckIcon aria-hidden="true" className="size-3.5 shrink-0 text-brand" />
      ) : (
        <ClipboardIcon aria-hidden="true" className="size-3.5 shrink-0 text-muted-foreground" />
      )}
    </button>
  )
}

export default function ContactSection({ fields }: { fields: ContactField[] }) {
  const { copied, copy } = useCopied()
  const allText = fields.map((field) => `${field.label}: ${field.value}`).join('\n')

  return (
    <section className="space-y-1">
      <SectionHeading
        step={1}
        title="Your details"
        action={
          <Button type="button" variant="ghost" size="xs" onClick={() => void copy(allText)}>
            {copied ? <CheckIcon aria-hidden="true" /> : <ClipboardIcon aria-hidden="true" />} Copy all
          </Button>
        }
      />
      <p className="px-2 text-[0.7rem] text-muted-foreground">Click a field to copy just that value.</p>
      <div className="space-y-0.5">
        {fields.map((field) => (
          <CopyField key={field.label} {...field} />
        ))}
      </div>
    </section>
  )
}
