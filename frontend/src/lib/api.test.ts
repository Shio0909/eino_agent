import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { api, apiRaw, ApiError, errorMessage } from '@/lib/api'
import { useAuth } from '@/store/auth'

type FetchArgs = [string, RequestInit]

function mockFetch(res: Response | (() => Promise<Response>)) {
  const fn = vi.fn(async (..._args: FetchArgs) => (typeof res === 'function' ? res() : res.clone()))
  vi.stubGlobal('fetch', fn)
  return fn
}

const json = (status: number, body: unknown, headers: Record<string, string> = {}) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json', ...headers } })

beforeEach(() => {
  useAuth.setState({ token: null, user: null })
})
afterEach(() => {
  vi.unstubAllGlobals()
})

describe('request building', () => {
  it('prefixes the API base and appends non-empty query values', async () => {
    const f = mockFetch(json(200, {}))
    await api('/knowledge-bases', { query: { page: 2, q: '', a: undefined, b: null, flag: false, ids: ['x', 'y'] } })
    expect(f.mock.calls[0][0]).toBe('/api/v1/knowledge-bases?page=2&flag=false&ids=x&ids=y')
  })

  it('does not prefix paths that already start with /api/ or /health', async () => {
    const f = mockFetch(json(200, {}))
    await api('/health/ready')
    await api('/api/v1/system/info')
    expect(f.mock.calls.map((c) => c[0])).toEqual(['/health/ready', '/api/v1/system/info'])
  })

  it('sends the bearer token when logged in and none otherwise', async () => {
    const f = mockFetch(json(200, {}))
    await api('/x')
    expect((f.mock.calls[0][1].headers as Record<string, string>).Authorization).toBeUndefined()
    useAuth.setState({ token: 'tok' })
    await api('/x')
    expect((f.mock.calls[1][1].headers as Record<string, string>).Authorization).toBe('Bearer tok')
  })

  it('sends a JSON body with its content type', async () => {
    const f = mockFetch(json(200, {}))
    await api('/x', { method: 'POST', body: { a: 1 } })
    const init = f.mock.calls[0][1]
    expect(init.method).toBe('POST')
    expect(init.body).toBe('{"a":1}')
    expect((init.headers as Record<string, string>)['Content-Type']).toBe('application/json')
  })

  it('sends no content type when there is no body', async () => {
    const f = mockFetch(json(200, {}))
    await api('/x', { method: 'DELETE' })
    expect((f.mock.calls[0][1].headers as Record<string, string>)['Content-Type']).toBeUndefined()
  })

  it('leaves the content type to the browser for multipart bodies', async () => {
    const f = mockFetch(json(200, {}))
    const form = new FormData()
    form.append('file', new Blob(['x']), 'a.txt')
    await api('/x', { method: 'POST', form })
    const init = f.mock.calls[0][1]
    expect(init.body).toBe(form)
    expect((init.headers as Record<string, string>)['Content-Type']).toBeUndefined()
  })
})

describe('responses', () => {
  it('returns the parsed body', async () => {
    mockFetch(json(200, { a: 1 }))
    expect(await api<{ a: number }>('/x')).toEqual({ a: 1 })
  })

  it('returns undefined for 204 and for an empty body', async () => {
    mockFetch(new Response(null, { status: 204 }))
    expect(await api('/x')).toBeUndefined()
    mockFetch(new Response('', { status: 200 }))
    expect(await api('/x')).toBeUndefined()
  })

  it('apiRaw also reports the HTTP status', async () => {
    mockFetch(json(202, { status: 'pending' }))
    expect(await apiRaw('/x')).toEqual({ status: 202, data: { status: 'pending' } })
  })
})

describe('errors', () => {
  it('reads the standard error envelope', async () => {
    mockFetch(json(403, { error: { code: 'forbidden', message: '没有权限', trace_id: 't1', details: { kb: 'a' } } }))
    const err = await api('/x').catch((e) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect(err).toMatchObject({ status: 403, code: 'forbidden', message: '没有权限', traceId: 't1', details: { kb: 'a' } })
  })

  it('falls back to the X-Trace-ID header', async () => {
    mockFetch(json(500, { error: { code: 'internal', message: 'boom' } }, { 'X-Trace-ID': 'hdr' }))
    expect((await api('/x').catch((e) => e)).traceId).toBe('hdr')
  })

  it('reads a bare error string, as the settings save failure returns', async () => {
    mockFetch(json(500, { error: 'persist failed', changed: ['rag.top_k'] }))
    expect(await api('/x').catch((e) => e)).toMatchObject({ status: 500, message: 'persist failed' })
  })

  it('reads a top-level message', async () => {
    mockFetch(json(400, { message: '参数错误' }))
    expect((await api('/x').catch((e) => e)).message).toBe('参数错误')
  })

  it('uses plain text bodies, shortened', async () => {
    mockFetch(new Response('x'.repeat(500), { status: 502, statusText: 'Bad Gateway' }))
    const err = await api('/x').catch((e) => e)
    expect(err.status).toBe(502)
    expect(err.message).toHaveLength(200)
  })

  it('uses the status text when the body is empty', async () => {
    mockFetch(new Response('', { status: 503, statusText: 'Service Unavailable' }))
    expect((await api('/x').catch((e) => e)).message).toBe('Service Unavailable')
  })

  it('reports an unreachable server as status 0', async () => {
    mockFetch(() => Promise.reject(new TypeError('Failed to fetch')))
    expect(await api('/x').catch((e) => e)).toMatchObject({ status: 0, code: 'network_error' })
  })

  it('rethrows an abort untouched so callers can tell it from a failure', async () => {
    const abort = new DOMException('aborted', 'AbortError')
    mockFetch(() => Promise.reject(abort))
    expect(await api('/x').catch((e) => e)).toBe(abort)
  })
})

describe('401 handling', () => {
  it('drops the session when the token is rejected', async () => {
    useAuth.setState({ token: 'tok', user: { id: 'u', role: 'user', tenant_id: 1 } })
    mockFetch(json(401, { error: { code: 'unauthenticated', message: '登录已过期' } }))
    await api('/sessions').catch(() => {})
    expect(useAuth.getState().token).toBeNull()
    expect(useAuth.getState().user).toBeNull()
  })

  it('keeps the session when a failed login is what returned 401', async () => {
    useAuth.setState({ token: 'tok' })
    mockFetch(json(401, { error: { code: 'unauthenticated', message: '用户名或密码错误' } }))
    await api('/auth/login', { method: 'POST', body: {} }).catch(() => {})
    expect(useAuth.getState().token).toBe('tok')
  })

  it('does not log out on other errors', async () => {
    useAuth.setState({ token: 'tok' })
    mockFetch(json(403, { error: { code: 'forbidden', message: 'no' } }))
    await api('/x').catch(() => {})
    expect(useAuth.getState().token).toBe('tok')
  })
})

describe('errorMessage', () => {
  it('reads ApiError, Error and unknown values', () => {
    expect(errorMessage(new ApiError(400, 'x', '坏了'))).toBe('坏了')
    expect(errorMessage(new Error('plain'))).toBe('plain')
    expect(errorMessage('weird')).toBe('未知错误')
  })
})
