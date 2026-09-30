import { Button } from '@/src/components/ui/button'
import { Label } from '@/src/components/ui/label'
import { useId } from 'react'

export interface SegmentOption<T> {
  value: T
  label: string
  disabled?: boolean
}

export default function SegmentedControl<T extends string | number>({
  label,
  options,
  value,
  onChange
}: {
  label: string
  options: SegmentOption<T>[]
  value: T
  onChange: (value: T) => void
}) {
  const id = useId()

  return (
    <div className="space-y-1.5">
      <Label id={id}>{label}</Label>
      <div role="group" aria-labelledby={id} className="flex flex-wrap gap-1.5">
        {options.map((option) => (
          <Button
            key={option.value}
            type="button"
            size="sm"
            variant={option.value === value ? 'secondary' : 'outline'}
            aria-pressed={option.value === value}
            disabled={option.disabled}
            onClick={() => onChange(option.value)}
            className={option.value === value ? 'border-accent-border text-primary' : undefined}
          >
            {option.label}
          </Button>
        ))}
      </div>
    </div>
  )
}
