import { act, renderHook } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '@/src/api/client'
import { useAutosave } from './use-autosave'

function deferred() {
  let resolve!: () => void
  let reject!: (e: unknown) => void
  const promise = new Promise<void>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

const flush = () => act(() => vi.advanceTimersByTimeAsync(0))

describe('useAutosave', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  it('saves a queued value and reports saved', async () => {
    const save = vi.fn().mockResolvedValue(undefined)
    const onSaved = vi.fn()
    const { result } = renderHook(() => useAutosave(save, onSaved))

    expect(result.current.status).toBe('idle')

    act(() => result.current.queue('a'))
    await flush()

    expect(save).toHaveBeenCalledWith('a')
    expect(onSaved).toHaveBeenCalledWith('a')
    expect(result.current.status).toBe('saved')
  })

  it('coalesces values queued during a save into the newest one', async () => {
    const first = deferred()
    const save = vi.fn().mockReturnValueOnce(first.promise).mockResolvedValue(undefined)
    const onSaved = vi.fn()
    const { result } = renderHook(() => useAutosave(save, onSaved))

    act(() => result.current.queue(1))
    await flush()
    expect(result.current.status).toBe('saving')

    act(() => {
      result.current.queue(2)
      result.current.queue(3)
    })

    await act(async () => first.resolve())
    await flush()

    expect(save.mock.calls.map(([v]) => v)).toEqual([1, 3])
    // Only the final value counts as saved.
    expect(onSaved.mock.calls.map(([v]) => v)).toEqual([3])
    expect(result.current.status).toBe('saved')
  })

  it('sends at most five saves a second', async () => {
    const save = vi.fn().mockResolvedValue(undefined)
    const { result } = renderHook(() => useAutosave(save))

    for (let i = 0; i < 7; i++) {
      act(() => result.current.queue(i))
      await flush()
    }

    expect(save).toHaveBeenCalledTimes(5)

    await act(() => vi.advanceTimersByTimeAsync(1000))

    expect(save).toHaveBeenCalledTimes(6)
    expect(save).toHaveBeenLastCalledWith(6)
  })

  it('retries retryable failures with backoff', async () => {
    const save = vi
      .fn()
      .mockRejectedValueOnce(new ApiError('down', 503))
      .mockRejectedValueOnce(new TypeError('network'))
      .mockResolvedValue(undefined)
    const onSaved = vi.fn()
    const { result } = renderHook(() => useAutosave(save, onSaved))

    act(() => result.current.queue('x'))
    await flush()
    expect(result.current.status).toBe('retrying')
    expect(save).toHaveBeenCalledTimes(1)

    await act(() => vi.advanceTimersByTimeAsync(1000))
    expect(save).toHaveBeenCalledTimes(2)

    // Second failure doubles the wait.
    await act(() => vi.advanceTimersByTimeAsync(1999))
    expect(save).toHaveBeenCalledTimes(2)
    await act(() => vi.advanceTimersByTimeAsync(1))

    expect(save).toHaveBeenCalledTimes(3)
    expect(onSaved).toHaveBeenCalledWith('x')
    expect(result.current.status).toBe('saved')
  })

  it.each([408, 429, 500])('treats %i as retryable', async (status) => {
    const save = vi.fn().mockRejectedValueOnce(new ApiError('x', status)).mockResolvedValue(undefined)
    const { result } = renderHook(() => useAutosave(save))

    act(() => result.current.queue('x'))
    await flush()
    await act(() => vi.advanceTimersByTimeAsync(1000))

    expect(save).toHaveBeenCalledTimes(2)
  })

  it('stops on a client error', async () => {
    const save = vi.fn().mockRejectedValue(new ApiError('invalid url', 400))
    const onSaved = vi.fn()
    const { result } = renderHook(() => useAutosave(save, onSaved))

    act(() => result.current.queue('bad'))
    await flush()
    await act(() => vi.advanceTimersByTimeAsync(60_000))

    expect(save).toHaveBeenCalledTimes(1)
    expect(onSaved).not.toHaveBeenCalled()
    expect(result.current.status).toBe('error')

    // A new edit tries again.
    save.mockResolvedValue(undefined)
    act(() => result.current.queue('good'))
    await flush()
    expect(result.current.status).toBe('saved')
  })

  it('moves on to a newer value after a client error', async () => {
    const first = deferred()
    const save = vi.fn().mockReturnValueOnce(first.promise).mockResolvedValue(undefined)
    const { result } = renderHook(() => useAutosave(save))

    act(() => result.current.queue('bad'))
    await flush()
    act(() => result.current.queue('good'))

    await act(async () => first.reject(new ApiError('nope', 422)))
    await flush()

    expect(save.mock.calls.map(([v]) => v)).toEqual(['bad', 'good'])
    expect(result.current.status).toBe('saved')
  })

  it('warns before unload only while a save is running', async () => {
    const pending = deferred()
    const save = vi.fn().mockReturnValue(pending.promise)
    const { result } = renderHook(() => useAutosave(save))

    const idle = new Event('beforeunload', { cancelable: true })
    window.dispatchEvent(idle)
    expect(idle.defaultPrevented).toBe(false)

    act(() => result.current.queue('x'))
    await flush()

    const busy = new Event('beforeunload', { cancelable: true })
    window.dispatchEvent(busy)
    expect(busy.defaultPrevented).toBe(true)

    await act(async () => pending.resolve())
  })

  it('uses the latest save callback', async () => {
    const saveA = vi.fn().mockResolvedValue(undefined)
    const saveB = vi.fn().mockResolvedValue(undefined)
    const { result, rerender } = renderHook(({ save }) => useAutosave(save), { initialProps: { save: saveA } })

    rerender({ save: saveB })
    act(() => result.current.queue('v'))
    await flush()

    expect(saveA).not.toHaveBeenCalled()
    expect(saveB).toHaveBeenCalledWith('v')
  })
})
