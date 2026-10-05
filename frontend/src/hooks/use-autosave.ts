import { ApiError } from '@/src/api/client'
import { useCallback, useEffect, useRef, useState } from 'react'

const MAX_SENDS_PER_WINDOW = 5
const WINDOW_MS = 1000
const MAX_RETRY_DELAY_MS = 30_000

export type AutosaveStatus = 'idle' | 'saving' | 'saved' | 'retrying' | 'error'

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms))

// Other 4xx responses fail the same way on every retry.
function isRetryable(error: unknown) {
  return !(error instanceof ApiError) || error.status >= 500 || error.status === 408 || error.status === 429
}

// useAutosave saves the latest queued value, one request at a time and at most 5 a second.
// Changes made while waiting are coalesced, never dropped; failed saves retry with backoff.
export function useAutosave<T>(save: (value: T) => Promise<unknown>, onSaved?: (value: T) => void) {
  const [status, setStatus] = useState<AutosaveStatus>('idle')
  const callbacks = useRef({ save, onSaved })
  const pending = useRef<{ value: T } | null>(null)
  const running = useRef(false)
  const sentAt = useRef<number[]>([])

  useEffect(() => {
    callbacks.current = { save, onSaved }
  })

  const waitForSlot = useCallback(async () => {
    for (;;) {
      const now = Date.now()
      sentAt.current = sentAt.current.filter((t) => now - t < WINDOW_MS)

      if (sentAt.current.length < MAX_SENDS_PER_WINDOW) {
        sentAt.current.push(now)
        return
      }

      await sleep(sentAt.current[0] + WINDOW_MS - now)
    }
  }, [])

  const run = useCallback(async () => {
    running.current = true
    let failures = 0

    while (pending.current) {
      await waitForSlot()

      // Taken after the wait, so it's the newest value.
      const { value } = pending.current
      pending.current = null
      setStatus(failures ? 'retrying' : 'saving')

      try {
        await callbacks.current.save(value)
        failures = 0
        if (!pending.current) callbacks.current.onSaved?.(value)
      } catch (error) {
        if (!isRetryable(error)) {
          failures = 0
          if (!pending.current) {
            running.current = false
            setStatus('error')
            return
          }
          continue
        }

        pending.current ??= { value }
        failures++
        setStatus('retrying')
        await sleep(Math.min(1000 * 2 ** (failures - 1), MAX_RETRY_DELAY_MS))
      }
    }

    running.current = false
    setStatus('saved')
  }, [waitForSlot])

  const queue = useCallback(
    (value: T) => {
      pending.current = { value }
      if (!running.current) void run()
    },
    [run]
  )

  // Warn before closing the tab with unsaved changes.
  useEffect(() => {
    const onBeforeUnload = (e: BeforeUnloadEvent) => {
      if (running.current) e.preventDefault()
    }

    window.addEventListener('beforeunload', onBeforeUnload)
    return () => window.removeEventListener('beforeunload', onBeforeUnload)
  }, [])

  return { queue, status }
}
