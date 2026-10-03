import type { ReactNode } from 'react'
import { AlertTriangle } from 'lucide-react'
import { Badge } from '@/components/ui/Badge'
import { formatDuration, truncate } from '@/lib/format'
import { hasRetrieval, retrievalView, sourceName, type TraceChunk } from '@/lib/trace'
import type { TraceStep } from '@/types/chat'

const GATE_TEXT: Record<string, string> = {
  irrelevant_evidence: '检索到的内容与问题不相关，系统没有基于它们作答。',
  insufficient_project_context: '知识库里没有足够的相关上下文，系统没有基于检索结果作答。',
}

function Stat({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="rounded-lg bg-surface-2 px-3 py-2">
      <p className="text-xs text-muted">{label}</p>
      <p className="font-semibold tabular-nums">{value}</p>
    </div>
  )
}

function ChunkRow({ chunk }: { chunk: TraceChunk }) {
  const name = sourceName({ ...chunk.metadata, source: chunk.source ?? chunk.metadata?.source }, chunk.doc_id)
  const content = chunk.content ?? ''
  return (
    <li className="rounded-lg border border-border p-2.5">
      <div className="flex flex-wrap items-center gap-1.5 text-xs">
        <span className="font-semibold tabular-nums">#{chunk.rank}</span>
        <span className="truncate font-medium">{name}</span>
        {chunk.match_type ? <Badge variant="primary">{chunk.match_type}</Badge> : null}
        {typeof chunk.score === 'number' && chunk.score !== 0 ? <span className="ml-auto text-muted tabular-nums">相关度 {chunk.score.toFixed(3)}</span> : null}
      </div>
      {content ? (
        <details className="mt-1.5">
          <summary className="cursor-pointer text-[13px] text-muted">{truncate(content.replace(/\s+/g, ' '), 120)}</summary>
          <p className="mt-1.5 text-[13px] whitespace-pre-wrap">{truncate(content, 2000)}</p>
        </details>
      ) : null}
    </li>
  )
}

function Group({ title, chunks, open }: { title: string; chunks: TraceChunk[]; open?: boolean }) {
  if (chunks.length === 0) return null
  return (
    <details open={open} className="rounded-lg">
      <summary className="cursor-pointer py-1 text-[13px] font-medium">
        {title} <span className="text-muted">({chunks.length})</span>
      </summary>
      <ul className="mt-1.5 space-y-1.5">
        {chunks.map((c, i) => (
          <ChunkRow key={`${c.doc_id}-${c.rank}-${i}`} chunk={c} />
        ))}
      </ul>
    </details>
  )
}

export function RetrievalPanel({ trace, latencyMs }: { trace: TraceStep[]; latencyMs?: number }) {
  const v = retrievalView(trace)
  if (!hasRetrieval(v)) return <p className="py-4 text-center text-[13px] text-muted">这次回答没有检索记录</p>
  return (
    <div className="space-y-3">
      {v.gate ? (
        <div className="flex gap-2 rounded-lg bg-warning-soft p-3 text-[13px] text-warning">
          <AlertTriangle className="mt-0.5 size-4 shrink-0" />
          <div>
            <p className="font-medium">证据不足</p>
            <p>{GATE_TEXT[v.gate.status] ?? v.gate.summary ?? '检索结果没有通过证据检查。'}</p>
          </div>
        </div>
      ) : null}
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
        <Stat label="召回候选" value={v.retrieved.length} />
        <Stat label="重排后" value={v.rerankAfter.length || '-'} />
        <Stat label="进入上下文" value={v.context.length} />
        <Stat label={latencyMs ? '总耗时' : '上下文长度'} value={latencyMs ? formatDuration(latencyMs) : (v.contextChars ?? '-')} />
      </div>
      <Group title="进入上下文的片段" chunks={v.context} open />
      <Group title="重排后" chunks={v.rerankAfter} />
      <Group title="重排前" chunks={v.rerankBefore} />
      <Group title="召回候选" chunks={v.retrieved} />
      {v.contextPreview ? (
        <details>
          <summary className="cursor-pointer py-1 text-[13px] font-medium">上下文预览</summary>
          <pre className="mt-1.5 max-h-60 overflow-auto rounded-lg bg-code p-2.5 font-mono text-xs whitespace-pre-wrap">{v.contextPreview}</pre>
        </details>
      ) : null}
    </div>
  )
}
