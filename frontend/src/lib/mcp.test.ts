import { describe, expect, it } from 'vitest'
import { buildMcpImport, parseToolNames, type McpForm } from '@/lib/mcp'

const form = (over: Partial<McpForm> = {}): McpForm => ({
  provider: 'custom',
  name: 'docs',
  endpoint: 'https://mcp.example.com/sse',
  transport: '',
  toolNames: '',
  apiKey: '',
  useKeyInUrl: false,
  ...over,
})

describe('parseToolNames', () => {
  it('splits on commas, spaces and new lines, dropping blanks and repeats', () => {
    expect(parseToolNames('search, fetch\nsearch  lookup,,\n')).toEqual(['search', 'fetch', 'lookup'])
  })

  it('returns nothing for blank text', () => {
    expect(parseToolNames('  \n ')).toEqual([])
  })
})

describe('buildMcpImport', () => {
  it('sends a custom server with only the fields that were filled in', () => {
    expect(buildMcpImport(form())).toEqual({ ok: true, request: { provider: 'custom', name: 'docs', endpoint: 'https://mcp.example.com/sse' } })
  })

  it('includes transport, tools and key when given', () => {
    const result = buildMcpImport(form({ transport: 'streamable_http', toolNames: 'a, b', apiKey: ' secret ' }))
    expect(result).toEqual({
      ok: true,
      request: { provider: 'custom', name: 'docs', endpoint: 'https://mcp.example.com/sse', transport: 'streamable_http', tool_names: ['a', 'b'], api_key: 'secret' },
    })
  })

  it('trims the name and endpoint', () => {
    const result = buildMcpImport(form({ name: '  docs ', endpoint: ' https://x.example.com/mcp ' }))
    expect(result).toMatchObject({ ok: true, request: { name: 'docs', endpoint: 'https://x.example.com/mcp' } })
  })

  it('needs a name and an http(s) endpoint for a custom server', () => {
    expect(buildMcpImport(form({ name: ' ' }))).toMatchObject({ ok: false, errors: { name: expect.any(String) } })
    expect(buildMcpImport(form({ endpoint: '' }))).toMatchObject({ ok: false, errors: { endpoint: expect.any(String) } })
    expect(buildMcpImport(form({ endpoint: 'ftp://x' }))).toMatchObject({ ok: false, errors: { endpoint: expect.any(String) } })
    expect(buildMcpImport(form({ endpoint: 'https://' }))).toMatchObject({ ok: false, errors: { endpoint: expect.any(String) } })
    expect(buildMcpImport(form({ endpoint: 'https://a b' }))).toMatchObject({ ok: false, errors: { endpoint: expect.any(String) } })
    expect(buildMcpImport(form({ endpoint: 'mcp.example.com' }))).toMatchObject({ ok: false, errors: { endpoint: expect.any(String) } })
  })

  it('reports both problems at once', () => {
    const result = buildMcpImport(form({ name: '', endpoint: '' }))
    expect(result.ok).toBe(false)
    if (!result.ok) expect(Object.keys(result.errors).sort()).toEqual(['endpoint', 'name'])
  })

  it('lets a tavily import be empty, since the server has defaults and a key of its own', () => {
    expect(buildMcpImport(form({ provider: 'tavily', name: '', endpoint: '' }))).toEqual({ ok: true, request: { provider: 'tavily' } })
  })

  it('still checks a tavily endpoint when one is typed', () => {
    expect(buildMcpImport(form({ provider: 'tavily', name: '', endpoint: 'nope' }))).toMatchObject({ ok: false, errors: { endpoint: expect.any(String) } })
  })

  it('passes use_key_in_url for tavily only, and only when switched on', () => {
    expect(buildMcpImport(form({ provider: 'tavily', name: '', endpoint: '', apiKey: 'k', useKeyInUrl: true }))).toEqual({
      ok: true,
      request: { provider: 'tavily', api_key: 'k', use_key_in_url: true },
    })
    expect(buildMcpImport(form({ provider: 'tavily', name: '', endpoint: '', useKeyInUrl: false }))).toEqual({ ok: true, request: { provider: 'tavily' } })
    expect(buildMcpImport(form({ useKeyInUrl: true }))).toEqual({ ok: true, request: { provider: 'custom', name: 'docs', endpoint: 'https://mcp.example.com/sse' } })
  })
})
