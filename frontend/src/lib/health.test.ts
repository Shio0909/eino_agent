import { afterEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '@/lib/api'
import { componentLabel, featureLabel, fetchHealth, healthStatusMeta, sortComponents } from '@/lib/health'
import type { HealthComponent } from '@/types/system'

afterEach(() => vi.unstubAllGlobals())

const report = { status: 'down', timestamp: '2024-01-01T00:00:00Z', components: { database: { status: 'down', required: true, error: 'refused' } } }

describe('fetchHealth', () => {
  it('returns the report even when the server answers 503 for a failing dependency', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify(report), { status: 503, headers: { 'Content-Type': 'application/json' } })))
    await expect(fetchHealth()).resolves.toEqual(report)
  })

  it('reads /health/ready without credentials', async () => {
    const f = vi.fn(async (..._args: [string, RequestInit]) => new Response(JSON.stringify(report), { status: 200 }))
    vi.stubGlobal('fetch', f)
    await fetchHealth()
    expect(f.mock.calls[0][0]).toBe('/health/ready')
    expect(f.mock.calls[0][1].headers).toBeUndefined()
  })

  it('fails with an ApiError when the body is not a health report', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response('<html>bad gateway</html>', { status: 502 })))
    const err = await fetchHealth().catch((e: unknown) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect((err as ApiError).status).toBe(502)
  })

  it.each([['an empty object', '{}'], ['no components', '{"status":"ok"}'], ['no status', '{"components":{}}'], ['null', 'null'], ['a list', '[]']])(
    'rejects a 200 whose body is %s',
    async (_label, body) => {
      vi.stubGlobal('fetch', vi.fn(async () => new Response(body, { status: 200 })))
      await expect(fetchHealth()).rejects.toBeInstanceOf(ApiError)
    },
  )

  it('uses the body text for the error, or the status text when there is none', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response('upstream timed out', { status: 504 })))
    await expect(fetchHealth()).rejects.toMatchObject({ message: 'upstream timed out' })
    vi.stubGlobal('fetch', vi.fn(async () => new Response('', { status: 502, statusText: 'Bad Gateway' })))
    await expect(fetchHealth()).rejects.toMatchObject({ message: 'Bad Gateway' })
  })

  it('fails with a connection error when the server cannot be reached', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => { throw new TypeError('Failed to fetch') }))
    const err = await fetchHealth().catch((e: unknown) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect((err as ApiError).status).toBe(0)
  })

  it('passes the abort signal through and lets an abort escape unchanged', async () => {
    const ctl = new AbortController()
    const f = vi.fn(async (..._args: [string, RequestInit]) => { throw new DOMException('aborted', 'AbortError') })
    vi.stubGlobal('fetch', f)
    await expect(fetchHealth(ctl.signal)).rejects.toMatchObject({ name: 'AbortError' })
    expect(f.mock.calls[0][1].signal).toBe(ctl.signal)
  })
})

describe('healthStatusMeta', () => {
  it('gives each status a label and badge colour', () => {
    expect(healthStatusMeta('ok')).toEqual({ label: '正常', variant: 'success' })
    expect(healthStatusMeta('degraded')).toEqual({ label: '降级', variant: 'warning' })
    expect(healthStatusMeta('down')).toEqual({ label: '不可用', variant: 'danger' })
    expect(healthStatusMeta('disabled')).toEqual({ label: '未启用', variant: 'neutral' })
  })

  it('treats a status it does not know as neutral and shows it as it is', () => {
    expect(healthStatusMeta('weird')).toEqual({ label: 'weird', variant: 'neutral' })
  })
})

describe('labels', () => {
  it('names the known components and falls back to the key', () => {
    expect(componentLabel('vector_store')).toBe('向量库')
    expect(componentLabel('import_queue')).toBe('导入队列')
    expect(componentLabel('something_new')).toBe('something_new')
  })

  it('names the known features and falls back to the key', () => {
    expect(featureLabel('graph_rag')).toBe('GraphRAG')
    expect(featureLabel('react_agent')).toBe('ReAct 智能体')
    expect(featureLabel('something_new')).toBe('something_new')
  })
})

describe('sortComponents', () => {
  const c = (status: HealthComponent['status'], required: boolean): HealthComponent => ({ status, required })

  it('lists failures first, then required components, then the rest by name', () => {
    const sorted = sortComponents({
      redis: c('disabled', false),
      llm: c('ok', true),
      docreader: c('down', false),
      database: c('ok', true),
      graphrag: c('degraded', false),
      import_queue: c('ok', false),
    })
    expect(sorted.map(([key]) => key)).toEqual(['docreader', 'graphrag', 'database', 'llm', 'import_queue', 'redis'])
  })

  it('puts components that are switched off after the ones that are running', () => {
    const sorted = sortComponents({ a_off: c('disabled', false), b_on: c('ok', false) })
    expect(sorted.map(([key]) => key)).toEqual(['b_on', 'a_off'])
  })

  it('puts a required component that is down ahead of an optional one that is down', () => {
    const sorted = sortComponents({ a: c('down', false), b: c('down', true) })
    expect(sorted.map(([key]) => key)).toEqual(['b', 'a'])
  })
})
