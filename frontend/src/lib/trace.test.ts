import { describe, expect, it } from 'vitest'
import { addSource, hasRetrieval, retrievalView, sourceName, sourcesFromTrace } from '@/lib/trace'
import type { TraceStep } from '@/types/chat'

const chunk = (rank: number, doc: string, extra: Record<string, unknown> = {}) => ({ rank, doc_id: doc, content: `c${rank}`, ...extra })

describe('retrievalView', () => {
  it('reads each retrieval stage from the trace', () => {
    const trace: TraceStep[] = [
      { type: 'retrieval', stage: 'retrieved_candidates', metadata: { chunks: [chunk(1, 'a'), chunk(2, 'b')], count: 2 } },
      { type: 'rerank', stage: 'rerank', metadata: { before: [chunk(1, 'a'), chunk(2, 'b')], after: [chunk(1, 'b')] } },
      { type: 'context', stage: 'context_build', metadata: { chunks: [chunk(1, 'b')], context_chars: 120, context_preview: 'hello' } },
    ]
    const v = retrievalView(trace)
    expect(v.retrieved.map((c) => c.doc_id)).toEqual(['a', 'b'])
    expect(v.rerankBefore).toHaveLength(2)
    expect(v.rerankAfter.map((c) => c.doc_id)).toEqual(['b'])
    expect(v.context).toHaveLength(1)
    expect(v.contextChars).toBe(120)
    expect(v.contextPreview).toBe('hello')
    expect(v.gate).toBeUndefined()
  })

  it('merges the separate before/after rerank frames of a live stream', () => {
    const v = retrievalView([
      { type: 'rerank', stage: 'rerank', metadata: { stage: 'rerank', before: [chunk(1, 'a')] } },
      { type: 'rerank', stage: 'rerank', metadata: { stage: 'rerank', after: [chunk(1, 'z')] } },
    ])
    expect(v.rerankBefore.map((c) => c.doc_id)).toEqual(['a'])
    expect(v.rerankAfter.map((c) => c.doc_id)).toEqual(['z'])
  })

  it('reports the evidence gate when it rejected the candidates', () => {
    const v = retrievalView([
      { type: 'guardrail', stage: 'retrieval_gate', summary: 'no support', metadata: { status: 'irrelevant_evidence' } },
    ])
    expect(v.gate).toEqual({ status: 'irrelevant_evidence', summary: 'no support' })
  })

  it('ignores malformed chunk lists', () => {
    const v = retrievalView([{ type: 'retrieval', stage: 'retrieved_candidates', metadata: { chunks: 'oops' } }, { type: 'status' }])
    expect(v.retrieved).toEqual([])
  })

  it('is empty for an empty trace', () => {
    const v = retrievalView([])
    expect(v).toMatchObject({ retrieved: [], rerankBefore: [], rerankAfter: [], context: [] })
  })
})

describe('sourceName', () => {
  it('prefers a file name, then the source, then the title, then the id', () => {
    expect(sourceName({ filename: 'a.pdf', source: 's' }, 'id')).toBe('a.pdf')
    expect(sourceName({ source_filename: 'b.md', source: 's' }, 'id')).toBe('b.md')
    expect(sourceName({ filename: 'a.pdf', source_filename: 'b.md' }, 'id')).toBe('a.pdf')
    expect(sourceName({ source: 'notes.md' }, 'id')).toBe('notes.md')
    expect(sourceName({ title: 'T' }, 'id')).toBe('T')
    expect(sourceName({}, 'id')).toBe('id')
    expect(sourceName(undefined, '')).toBe('未命名来源')
  })

  it('skips non-string and blank values', () => {
    expect(sourceName({ filename: 3, source: '  ', title: 'ok' }, 'id')).toBe('ok')
  })
})

describe('addSource', () => {
  it('keeps one entry per document and passage', () => {
    let list = addSource([], { docId: 'a', content: 'same text', metadata: {} })
    list = addSource(list, { docId: 'a', content: 'same text', metadata: {} })
    list = addSource(list, { docId: 'a', content: 'other text', metadata: {} })
    list = addSource(list, { docId: 'b', content: 'same text', metadata: {} })
    expect(list).toHaveLength(3)
  })

  it('does not mutate the input list', () => {
    const base = addSource([], { docId: 'a', content: 'x', metadata: {} })
    addSource(base, { docId: 'b', content: 'y', metadata: {} })
    expect(base).toHaveLength(1)
  })

  it('ignores a source with neither text nor id', () => {
    expect(addSource([], { docId: '', content: '  ', metadata: {} })).toEqual([])
  })
})

describe('sourcesFromTrace', () => {
  it('uses source steps when they exist', () => {
    const list = sourcesFromTrace([
      { type: 'source', doc_id: 'a', content: 'text a', metadata: { filename: 'a.md' } },
      { type: 'context', stage: 'context_build', metadata: { chunks: [chunk(1, 'z')] } },
    ])
    expect(list.map((s) => [s.docId, s.name])).toEqual([['a', 'a.md']])
  })

  it('falls back to the context chunks that built the answer', () => {
    const list = sourcesFromTrace([
      { type: 'context', stage: 'context_build', metadata: { chunks: [chunk(1, 'z', { source: 'z.md', score: 0.8 })] } },
    ])
    expect(list).toHaveLength(1)
    expect(list[0]).toMatchObject({ docId: 'z', name: 'z.md', score: 0.8 })
  })

  it('is empty when the trace has neither', () => {
    expect(sourcesFromTrace([{ type: 'status', stage: 'request' }])).toEqual([])
  })
})

describe('hasRetrieval', () => {
  it('is false for a trace with no retrieval work', () => {
    expect(hasRetrieval(retrievalView([{ type: 'status', stage: 'request' }]))).toBe(false)
  })
  it.each([
    ['retrieved candidates', { type: 'retrieval', stage: 'retrieved_candidates', metadata: { chunks: [chunk(1, 'a')] } }],
    ['rerank', { type: 'rerank', stage: 'rerank', metadata: { after: [chunk(1, 'a')] } }],
    ['context', { type: 'context', stage: 'context_build', metadata: { chunks: [chunk(1, 'a')] } }],
    ['a gate refusal', { type: 'guardrail', stage: 'retrieval_gate', metadata: { status: 'irrelevant_evidence' } }],
  ])('is true when the trace has %s', (_, step) => {
    expect(hasRetrieval(retrievalView([step as TraceStep]))).toBe(true)
  })
})
