import type { ReactNode } from 'react'

export default function SectionHeading({ step, title, action }: { step: number; title: string; action?: ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-2">
      <p className="text-xs font-medium text-heading">
        <span className="text-muted-foreground">{step}.</span> {title}
      </p>
      {action}
    </div>
  )
}
