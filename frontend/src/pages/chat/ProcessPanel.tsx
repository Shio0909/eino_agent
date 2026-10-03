import { useState } from 'react'
import { cn } from '@/lib/cn'
import { hasRetrieval, retrievalView } from '@/lib/trace'
import type { TraceStep } from '@/types/chat'
import { RetrievalPanel } from '@/pages/chat/RetrievalPanel'
import { TraceTimeline } from '@/pages/chat/TraceTimeline'

type Tab = 'retrieval' | 'timeline'

interface ProcessPanelProps {
  trace: TraceStep[]
  traceId?: string
  latencyMs?: number
}

export function ProcessPanel({ trace, traceId, latencyMs }: ProcessPanelProps) {
  const withRetrieval = hasRetrieval(retrievalView(trace))
  const [tab, setTab] = useState<Tab>(withRetrieval ? 'retrieval' : 'timeline')
  const tabs: { id: Tab; label: string }[] = [
    { id: 'retrieval', label: '检索' },
    { id: 'timeline', label: `时间线 (${trace.length})` },
  ]
  return (
    <div className="rounded-xl border border-border bg-surface p-3">
      <div className="mb-3 flex items-center gap-1 border-b border-border" role="tablist">
        {tabs.map((t) => (
          <button
            key={t.id}
            role="tab"
            aria-selected={tab === t.id}
            onClick={() => setTab(t.id)}
            className={cn(
              '-mb-px border-b-2 px-3 py-1.5 text-[13px] font-medium transition-colors',
              tab === t.id ? 'border-primary text-primary' : 'border-transparent text-muted hover:text-fg',
            )}
          >
            {t.label}
          </button>
        ))}
        {traceId ? <span className="ml-auto truncate pr-1 font-mono text-xs text-muted">trace {traceId}</span> : null}
      </div>
      {tab === 'retrieval' ? <RetrievalPanel trace={trace} latencyMs={latencyMs} /> : <TraceTimeline trace={trace} />}
    </div>
  )
}
