import type { ZodType } from 'zod'

export const API_BASE_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8090'

export async function apiFetch<T>(path: string, schema: ZodType<T>, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE_URL}${path}`, init)

  if (!res.ok) {
    let message = res.statusText

    try {
      const body = (await res.json()) as { error?: string }
      if (body.error) message = body.error
    } catch {
      // response had no JSON body
    }

    throw new Error(message)
  }

  return schema.parse(await res.json())
}
