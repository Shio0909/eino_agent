import type { TraceStep } from '@/types/chat'

export interface TraceChunk {
  rank: number
  doc_id: string
  content?: string
  score?: number
  match_type?: string
  source?: string
  metadata?: Record<string, unknown>
}

export interface RetrievalView {
  retrieved: TraceChunk[]
  rerankBefore: TraceChunk[]
  rerankAfter: TraceChunk[]
  context: TraceChunk[]
  contextChars?: number
  contextPreview?: string
  /** Set only when the evidence gate refused to answer from what was retrieved. */
  gate?: { status: string; summary?: string }
}

export interface SourceRef {
  /** Stable identity used to drop duplicates. */
  key: string
  docId: string
  name: string
  content: string
  score?: number
  metadata: Record<string, unknown>
}

function chunksOf(value: unknown): TraceChunk[] {
  return Array.isArray(value) ? value.filter((c): c is TraceChunk => !!c && typeof c === 'object') : []
}

function meta(step: TraceStep): Record<string, unknown> {
  return step.metadata ?? {}
}

/**
 * Collects the retrieval stages out of a trace. The same function serves a
 * live stream, a stored snapshot and a historical message, because all three
 * carry the stages as trace steps with a `stage` and chunk lists in `metadata`.
 */
export function retrievalView(trace: TraceStep[]): RetrievalView {
  const view: RetrievalView = { retrieved: [], rerankBefore: [], rerankAfter: [], context: [] }
  for (const step of trace) {
    const m = meta(step)
    switch (step.stage) {
      case 'retrieved_candidates':
        view.retrieved = chunksOf(m.chunks)
        break
      case 'rerank': {
        // A live stream sends "before" and "after" as separate frames.
        const before = chunksOf(m.before)
        const after = chunksOf(m.after)
        if (before.length) view.rerankBefore = before
        if (after.length) view.rerankAfter = after
        break
      }
      case 'context_build':
        view.context = chunksOf(m.chunks)
        if (typeof m.context_chars === 'number') view.contextChars = m.context_chars
        if (typeof m.context_preview === 'string') view.contextPreview = m.context_preview
        break
      case 'retrieval_gate':
        view.gate = { status: String(m.status ?? ''), summary: step.summary }
        break
    }
  }
  return view
}

/** True when the trace recorded any retrieval work or a refusal by the evidence gate. */
export function hasRetrieval(v: RetrievalView): boolean {
  return v.retrieved.length > 0 || v.rerankBefore.length > 0 || v.rerankAfter.length > 0 || v.context.length > 0 || !!v.gate
}

function text(v: unknown): string {
  return typeof v === 'string' ? v.trim() : ''
}

/** Display name of a retrieved passage: file name, then source, then title, then id. */
export function sourceName(metadata: Record<string, unknown> | undefined, docId: string): string {
  const m = metadata ?? {}
  return text(m.filename) || text(m.source) || text(m.title) || docId || '未命名来源'
}

export function sourceKey(docId: string, content: string): string {
  return `${docId}\u0000${content.trim().slice(0, 80)}`
}

/** Appends a source unless the same passage of the same document is already listed. */
export function addSource(
  list: SourceRef[],
  src: { docId: string; content: string; score?: number; metadata?: Record<string, unknown> },
): SourceRef[] {
  if (!src.docId && !src.content.trim()) return list
  const key = sourceKey(src.docId, src.content)
  if (list.some((s) => s.key === key)) return list
  const metadata = src.metadata ?? {}
  return [...list, { key, docId: src.docId, name: sourceName(metadata, src.docId), content: src.content, score: src.score, metadata }]
}

/**
 * Citations of a stored answer: the explicit source steps when the trace has
 * them, otherwise the chunks that went into the generation context.
 */
export function sourcesFromTrace(trace: TraceStep[]): SourceRef[] {
  let out: SourceRef[] = []
  for (const step of trace) {
    if (step.type === 'source') {
      out = addSource(out, { docId: step.doc_id ?? '', content: step.content ?? '', metadata: step.metadata })
    }
  }
  if (out.length) return out
  for (const c of retrievalView(trace).context) {
    out = addSource(out, {
      docId: c.doc_id ?? '',
      content: c.content ?? '',
      score: c.score,
      metadata: { ...c.metadata, source: c.source ?? c.metadata?.source },
    })
  }
  return out
}
