import { memo, useState } from 'react'
import { Badge, type BadgeVariant } from '@/components/ui/Badge'
import { formatDuration, prettyJson, truncate } from '@/lib/format'
import type { TraceStep } from '@/types/chat'

const TYPE: Record<string, { label: string; variant: BadgeVariant }> = {
  status: { label: '状态', variant: 'neutral' },
  retrieval: { label: '检索', variant: 'primary' },
  rerank: { label: '重排', variant: 'primary' },
  context: { label: '上下文', variant: 'primary' },
  rewrite: { label: '改写', variant: 'primary' },
  source: { label: '来源', variant: 'success' },
  action: { label: '调用', variant: 'warning' },
  observation: { label: '结果', variant: 'success' },
  guardrail: { label: '护栏', variant: 'warning' },
  warning: { label: '警告', variant: 'warning' },
  error: { label: '错误', variant: 'danger' },
}

function hasDetail(step: TraceStep): boolean {
  return !!(step.content || step.tool_input || step.error || (step.metadata && Object.keys(step.metadata).length > 0))
}

function detailOf(step: TraceStep): string {
  const detail: Record<string, unknown> = {}
  if (step.content) detail.content = truncate(step.content, 1500)
  if (step.tool_input) detail.tool_input = step.tool_input
  if (step.error) detail.error = step.error
  if (step.metadata && Object.keys(step.metadata).length > 0) detail.metadata = step.metadata
  return prettyJson(detail)
}

const TraceRow = memo(function TraceRow({ step }: { step: TraceStep }) {
  // The JSON of a step can be large (retrieved chunks), so it is built only
  // once the row is opened.
  const [open, setOpen] = useState(false)
  const t = TYPE[step.type] ?? { label: step.type, variant: 'neutral' as const }
  const head = (
    <>
      <Badge variant={t.variant}>{t.label}</Badge>
      <span className="min-w-0 flex-1 truncate text-[13px]">{[step.stage, step.tool_name, step.summary].filter(Boolean).join(' · ') || '-'}</span>
      {step.latency_ms ? <span className="text-xs text-muted tabular-nums">{formatDuration(step.latency_ms)}</span> : null}
    </>
  )
  if (!hasDetail(step)) return <div className="flex items-center gap-2 px-2.5 py-1.5">{head}</div>
  return (
    <details onToggle={(e) => setOpen(e.currentTarget.open)}>
      <summary className="flex cursor-pointer items-center gap-2 px-2.5 py-1.5">{head}</summary>
      {open ? <pre className="max-h-64 overflow-auto border-t border-border bg-code p-2.5 font-mono text-xs whitespace-pre-wrap">{detailOf(step)}</pre> : null}
    </details>
  )
})

export function TraceTimeline({ trace }: { trace: TraceStep[] }) {
  if (trace.length === 0) return <p className="py-4 text-center text-[13px] text-muted">没有过程记录</p>
  return (
    <ol className="space-y-1">
      {trace.map((step, i) => (
        <li key={`${step.seq ?? i}-${i}`} className="rounded-lg border border-border">
          <TraceRow step={step} />
        </li>
      ))}
    </ol>
  )
}
