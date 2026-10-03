import { describe, expect, it } from 'vitest'
import { changedPaths, diffSettings, isMasked, validateSettings } from '@/lib/settings'
import type { Settings } from '@/types/settings'

const base = (): Settings => ({
  rag: { top_k: 5, enable_hybrid: true, enable_rewrite: true, enable_rerank: true, chunk_size: 500, chunk_overlap: 50, chunk_strategy: 'recursive', enable_contextual_enrichment: false },
  llm: { provider: 'openai', model: 'gpt-4o', base_url: 'https://api.example.com/v1', api_key: 'sk-1****wxyz', temperature: 0.7, max_tokens: 4096 },
  embedding: { provider: 'openai', model: 'text-embedding-3-small', base_url: 'https://api.example.com/v1', api_key: '****' },
  reranker: { enabled: false, provider: '', model: '', base_url: '', api_key: '', top_k: 5, threshold: 0.5 },
  agent: { enabled: true, max_steps: 10, system_prompt: 'You are helpful', enable_knowledge_tool: true, enable_web_search: false },
  mcp: { enabled: true, server_count: 1 },
  docreader: { enabled: true, active: true },
  graphrag: { enabled: false, neo4j_uri: 'bolt://x', extract_temperature: 0, top_k: 5 },
})

describe('isMasked', () => {
  it('recognises the placeholder the server returns for stored keys', () => {
    expect(isMasked('sk-1****wxyz')).toBe(true)
    expect(isMasked('****')).toBe(true)
    expect(isMasked('a***b')).toBe(false)
    expect(isMasked('sk-real-key')).toBe(false)
    expect(isMasked('')).toBe(false)
  })
})

describe('changedPaths', () => {
  it('is empty for an untouched copy', () => {
    expect(changedPaths(base(), base())).toEqual([])
  })

  it('names each changed editable field with a dotted path', () => {
    const draft = base()
    draft.rag.top_k = 8
    draft.llm.model = 'gpt-4.1'
    draft.agent.enable_web_search = true
    expect(changedPaths(base(), draft).sort()).toEqual(['agent.enable_web_search', 'llm.model', 'rag.top_k'])
  })

  it('ignores fields the API does not accept for update', () => {
    const draft = base()
    draft.rag.chunk_strategy = 'other'
    draft.agent.system_prompt = 'changed'
    draft.graphrag.neo4j_uri = 'bolt://elsewhere'
    draft.mcp = { enabled: false, server_count: 9 }
    expect(changedPaths(base(), draft)).toEqual([])
  })

  it('treats a key as changed only when a new, unmasked value was typed', () => {
    const draft = base()
    expect(changedPaths(base(), draft)).toEqual([])
    draft.llm.api_key = ''
    expect(changedPaths(base(), draft)).toEqual([])
    draft.llm.api_key = 'sk-brand-new'
    expect(changedPaths(base(), draft)).toEqual(['llm.api_key'])
    draft.llm.api_key = 'sk-other****tail'
    expect(changedPaths(base(), draft)).toEqual([])
  })
})

describe('diffSettings', () => {
  it('builds a patch holding only the changed fields', () => {
    const draft = base()
    draft.rag.top_k = 8
    draft.llm.api_key = 'sk-brand-new'
    draft.reranker.enabled = true
    expect(diffSettings(base(), draft)).toEqual({ rag: { top_k: 8 }, llm: { api_key: 'sk-brand-new' }, reranker: { enabled: true } })
  })

  it('is empty when nothing changed, so nothing is sent', () => {
    expect(diffSettings(base(), base())).toEqual({})
  })

  it('sends numbers as numbers and keeps false values', () => {
    const draft = base()
    draft.rag.enable_hybrid = false
    draft.llm.temperature = 0
    expect(diffSettings(base(), draft)).toEqual({ rag: { enable_hybrid: false }, llm: { temperature: 0 } })
  })
})

describe('validateSettings', () => {
  const errorsFor = (mutate: (s: Settings) => void) => {
    const draft = base()
    mutate(draft)
    return validateSettings(draft, changedPaths(base(), draft))
  }

  it('accepts the current values and the limits themselves', () => {
    expect(validateSettings(base(), changedPaths(base(), base()))).toEqual({})
    expect(errorsFor((s) => { s.rag.top_k = 100; s.rag.chunk_size = 50; s.rag.chunk_overlap = 0 })).toEqual({})
    expect(errorsFor((s) => { s.rag.top_k = 1; s.rag.chunk_size = 10000; s.rag.chunk_overlap = 5000 })).toEqual({})
  })

  it.each([
    ['rag.top_k', 0, 101],
    ['rag.chunk_size', 49, 10001],
    ['agent.max_steps', 0, 51],
    ['llm.max_tokens', 0, 128001],
    ['reranker.top_k', 0, 51],
    ['graphrag.top_k', 0, 51],
  ])('rejects %s outside its range', (path, low, high) => {
    const [section, field] = path.split('.') as [keyof Settings, string]
    for (const value of [low, high]) {
      const errors = errorsFor((s) => { (s[section] as unknown as Record<string, number>)[field] = value })
      expect(errors[path as string], `${path}=${value}`).toBeTruthy()
    }
  })

  it.each([
    ['llm.temperature', -0.1, 2.1],
    ['reranker.threshold', -0.1, 1.1],
    ['graphrag.extract_temperature', -0.1, 2.1],
  ])('rejects %s outside its range', (path, low, high) => {
    const [section, field] = path.split('.') as [keyof Settings, string]
    for (const value of [low, high]) {
      expect(errorsFor((s) => { (s[section] as unknown as Record<string, number>)[field] = value })[path as string]).toBeTruthy()
    }
  })

  it.each([
    ['rag.top_k', 1, 100],
    ['rag.chunk_size', 50, 10000],
    ['agent.max_steps', 1, 50],
    ['llm.temperature', 0, 2],
    ['llm.max_tokens', 1, 128000],
    ['reranker.top_k', 1, 50],
    ['reranker.threshold', 0, 1],
    ['graphrag.extract_temperature', 0, 2],
    ['graphrag.top_k', 1, 50],
  ])('accepts %s at both ends of its range', (path, low, high) => {
    const [section, field] = path.split('.') as [keyof Settings, string]
    for (const value of [low, high]) {
      const errors = errorsFor((s) => { (s[section] as unknown as Record<string, number>)[field] = value })
      expect(errors[path as string], `${path}=${value}`).toBeUndefined()
    }
  })

  it('keeps the range message when the overlap is also not below the size', () => {
    expect(errorsFor((s) => { s.rag.chunk_overlap = 6000 })['rag.chunk_overlap']).toContain('5000')
  })

  it('accepts fractional values where the field allows them', () => {
    expect(errorsFor((s) => { s.llm.temperature = 1.3; s.reranker.threshold = 0.25 })).toEqual({})
  })

  it('requires whole numbers for counts', () => {
    expect(errorsFor((s) => { s.rag.top_k = 2.5 })['rag.top_k']).toBeTruthy()
  })

  it('rejects values that are not numbers', () => {
    expect(errorsFor((s) => { s.rag.top_k = Number.NaN })['rag.top_k']).toContain('数字')
  })

  it('requires the overlap to be smaller than the chunk size, whichever of the two changed', () => {
    expect(errorsFor((s) => { s.rag.chunk_overlap = 500 })['rag.chunk_overlap']).toBeTruthy()
    expect(errorsFor((s) => { s.rag.chunk_size = 50 })['rag.chunk_overlap']).toBeTruthy()
    expect(errorsFor((s) => { s.rag.chunk_size = 400; s.rag.chunk_overlap = 399 })).toEqual({})
  })

  it('does not complain about a stored value that was left alone', () => {
    const draft = base()
    draft.rag.top_k = 500 // out of range on the server already
    draft.llm.model = 'x'
    expect(validateSettings(draft, ['llm.model'])).toEqual({})
  })
})
