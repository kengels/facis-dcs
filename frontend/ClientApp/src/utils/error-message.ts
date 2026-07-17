import axios from 'axios'

interface ErrorPayload {
  message?: unknown
  name?: unknown
}

export const extractErrorMessage = (error: unknown, fallback: string): string => {
  if (axios.isAxiosError(error)) {
    const payload = error.response?.data as ErrorPayload | string | undefined
    if (typeof payload === 'string' && payload.trim()) return payload
    if (payload && typeof payload === 'object') {
      if (typeof payload.message === 'string' && payload.message.trim()) return payload.message
      if (typeof payload.name === 'string' && payload.name.trim()) return payload.name
    }
    if (error.message) return error.message
  }
  if (error instanceof Error && error.message) return error.message
  return fallback
}
