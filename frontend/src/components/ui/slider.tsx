import { Slider as SliderPrimitive } from '@base-ui/react/slider'
import { cn } from '@/src/lib/utils'

function Slider({ className, value, ...props }: SliderPrimitive.Root.Props<number[]>) {
  return (
    <SliderPrimitive.Root
      data-slot="slider"
      value={value}
      className={cn('relative flex w-full touch-none items-center select-none', className)}
      {...props}
    >
      <SliderPrimitive.Control className="flex w-full items-center py-2">
        <SliderPrimitive.Track className="relative h-1.5 w-full grow rounded-full bg-input">
          <SliderPrimitive.Indicator className="absolute h-full rounded-full bg-primary" />
          {(value ?? []).map((_, i) => (
            <SliderPrimitive.Thumb
              key={i}
              index={i}
              className="block size-4 rounded-full border border-primary bg-background shadow-sm outline-none transition-colors focus-visible:ring-[3px] focus-visible:ring-ring/50"
            />
          ))}
        </SliderPrimitive.Track>
      </SliderPrimitive.Control>
    </SliderPrimitive.Root>
  )
}

export { Slider }
