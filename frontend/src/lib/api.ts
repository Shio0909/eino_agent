import { useAuth } from '@/store/auth'
import type { ApiErrorBody } from '@/types/api'

export const API_BASE = '/api/v1'

export const NETWORK_ERROR_MESSAGE = '无法连接到服务器，请检查网络或后端是否已启动'

/** An error response from the server, or a failure to reach it (status 0). */
export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly traceId?: string
  readonly details?: unknown

  constructor(status: number, code: string, message: string, traceId?: string, details?: unknown) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.traceId = traceId
    this.details = details
  }
}

export type QueryValue = string | number | boolean | null | undefined | readonly string[]
export type Query = Record<string, QueryValue>

export interface RequestOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
  query?: Query
  /** JSON body. */
  body?: unknown
  /** Multipart body; sets no Content-Type so the browser adds the boundary. */
  form?: FormData
  signal?: AbortSignal
  headers?: Record<string, string>
}

function buildUrl(path: string, query?: Query): string {
  const url = path.startsWith('/api/') || path.startsWith('/health') ? path : API_BASE + path
  if (!query) return url
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(query)) {
    if (value === undefined || value === null || value === '') continue
    if (Array.isArray(value)) value.forEach((v) => params.append(key, v))
    else params.set(key, String(value))
  }
  const qs = params.toString()
  return qs ? `${url}?${qs}` : url
}

export function authHeaders(): Record<string, string> {
  const token = useAuth.getState().token
  return token ? { Authorization: `Bearer ${token}` } : {}
}

/** Turns a failed response into an ApiError, tolerating non-envelope bodies. */
export async function toApiError(res: Response): Promise<ApiError> {
  const traceId = res.headers.get('X-Trace-ID') ?? undefined
  let text = ''
  try {
    text = await res.text()
  } catch {
    // body unreadable; fall through to the status text
  }
  try {
    const parsed = JSON.parse(text) as Partial<ApiErrorBody> & { error?: unknown; message?: string }
    const err = parsed.error
    if (err && typeof err === 'object') {
      const e = err as ApiErrorBody['error']
      return new ApiError(res.status, e.code || 'error', e.message || res.statusText, e.trace_id ?? traceId, e.details)
    }
    if (typeof err === 'string') return new ApiError(res.status, 'error', err, traceId)
    if (typeof parsed.message === 'string') return new ApiError(res.status, 'error', parsed.message, traceId)
  } catch {
    // not JSON
  }
  return new ApiError(res.status, 'error', text.trim().slice(0, 200) || res.statusText || `HTTP ${res.status}`, traceId)
}

/** Drops the session when the server rejects the token. */
function onUnauthorized(path: string) {
  if (path !== '/auth/login') useAuth.getState().logout()
}

/** Like `api`, but also returns the HTTP status for endpoints whose meaning depends on it (200 vs 202). */
export async function apiRaw<T = void>(path: string, opts: RequestOptions = {}): Promise<{ status: number; data: T }> {
  const headers: Record<string, string> = { Accept: 'application/json', ...authHeaders(), ...opts.headers }
  let body: BodyInit | undefined
  if (opts.form) {
    body = opts.form
  } else if (opts.body !== undefined) {
    headers['Content-Type'] = 'application/json'
    body = JSON.stringify(opts.body)
  }

  let res: Response
  try {
    res = await fetch(buildUrl(path, opts.query), { method: opts.method ?? 'GET', headers, body, signal: opts.signal })
  } catch (err) {
    if (err instanceof DOMException && err.name === 'AbortError') throw err
    throw new ApiError(0, 'network_error', NETWORK_ERROR_MESSAGE)
  }

  if (!res.ok) {
    const error = await toApiError(res)
    if (res.status === 401) onUnauthorized(path)
    throw error
  }
  const text = await res.text()
  return { status: res.status, data: (text ? JSON.parse(text) : undefined) as T }
}

export async function api<T = void>(path: string, opts: RequestOptions = {}): Promise<T> {
  return (await apiRaw<T>(path, opts)).data
}

export const apiGet = <T>(path: string, query?: Query, signal?: AbortSignal) => api<T>(path, { query, signal })
export const apiPost = <T = void>(path: string, body?: unknown) => api<T>(path, { method: 'POST', body })
export const apiPut = <T = void>(path: string, body?: unknown) => api<T>(path, { method: 'PUT', body })
export const apiPatch = <T = void>(path: string, body?: unknown) => api<T>(path, { method: 'PATCH', body })
export const apiDelete = <T = void>(path: string) => api<T>(path, { method: 'DELETE' })

/** Human-readable text for anything thrown by a request. */
export function errorMessage(err: unknown): string {
  if (err instanceof ApiError) return err.message
  if (err instanceof Error) return err.message
  return '未知错误'
}
