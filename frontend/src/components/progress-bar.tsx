import { useIsFetching } from '@tanstack/react-query'
import { useRouterState } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { cn } from '../lib/utils'

// Delay hides the bar for loads fast enough not to notice.
const SHOW_DELAY_MS = 150

// Global top bar for navigations and first-time fetches; background refetches don't count.
export function ProgressBar() {
  const isFetching = useIsFetching({ predicate: (query) => query.state.status === 'pending' }) > 0
  const isNavigating = useRouterState({ select: (state) => state.status === 'pending' })
  const active = isFetching || isNavigating
  const [visible, setVisible] = useState(false)

  useEffect(() => {
    if (!active) {
      setVisible(false)
      return
    }
    const handle = setTimeout(() => setVisible(true), SHOW_DELAY_MS)
    return () => clearTimeout(handle)
  }, [active])

  return (
    <div
      aria-hidden="true"
      className={cn(
        'pointer-events-none fixed inset-x-0 top-0 z-50 h-0.5 overflow-hidden transition-opacity duration-300 print:hidden',
        visible ? 'opacity-100' : 'opacity-0'
      )}
    >
      <div className="h-full w-1/3 animate-progress bg-brand" />
    </div>
  )
}
