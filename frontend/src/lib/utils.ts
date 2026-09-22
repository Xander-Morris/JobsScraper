import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function errorMessage(error: unknown): string | null {
  if (!error) return null
  return error instanceof Error ? error.message : 'Something went wrong'
}
