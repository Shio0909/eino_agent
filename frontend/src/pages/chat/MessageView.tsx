import { memo, useState } from 'react'
import { AlertTriangle, Check, ChevronDown, Copy, Loader2, Sparkles, Square, TriangleAlert } from 'lucide-react'
import { Markdown } from '@/components/Markdown'
import { cn } from '@/lib/cn'
import { formatDuration } from '@/lib/format'
import type { Turn } from '@/lib/chatTurn'
import { AgentSteps } from '@/pages/chat/AgentSteps'
import { ApprovalCard } from '@/pages/chat/ApprovalCard'
import { ProcessPanel } from '@/pages/chat/ProcessPanel'
import { SourceList } from '@/pages/chat/SourceList'

export function UserMessage({ content }: { content: string }) {
  return (
    <div className="flex justify-end">
      <div className="max-w-[85%] rounded-2xl rounded-br-md bg-primary px-4 py-2.5 whitespace-pre-wrap text-primary-fg [overflow-wrap:anywhere]">{content}</div>
    </div>
  )
}

function CopyButton({ text }: { text: string }) {
  const [done, setDone] = useState(false)
  return (
    <button
      aria-label="复制回答"
      onClick={() => {
        void navigator.clipboard?.writeText(text).then(() => {
          setDone(true)
          setTimeout(() => setDone(false), 1500)
        })
      }}
      className="inline-flex items-center gap-1 rounded-md px-1.5 py-1 text-xs text-muted hover:bg-surface-2 hover:text-fg"
    >
      {done ? <Check className="size-3.5" /> : <Copy className="size-3.5" />}
      {done ? '已复制' : '复制'}
    </button>
  )
}

interface AssistantMessageProps {
  turn: Turn
  latencyMs?: number
  onSettleApproval: (approvalId: string, status: 'approved' | 'rejected', reason?: string) => void
}

export const AssistantMessage = memo(function AssistantMessage({ turn, latencyMs, onSettleApproval }: AssistantMessageProps) {
  const [showProcess, setShowProcess] = useState(false)
  const streaming = turn.status === 'streaming'
  const hasProcess = turn.trace.length > 0
  const waitingForApproval = turn.approvals.some((a) => a.status === 'pending')

  return (
    <div className="flex gap-3">
      <div className="mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-full bg-primary-soft text-primary">
        <Sparkles className="size-4" />
      </div>
      <div className="min-w-0 flex-1 space-y-3">
        {turn.rewrite && streaming ? <p className="text-xs text-muted">改写后的问题：{turn.rewrite}</p> : null}

        <AgentSteps steps={turn.steps} />

        {turn.approvals.map((a) => (
          <ApprovalCard key={a.id} approval={a} onSettled={(status, reason) => onSettleApproval(a.id, status, reason)} />
        ))}

        {turn.warnings.map((w, i) => (
          <div key={i} className="flex gap-2 rounded-lg bg-warning-soft px-3 py-2 text-[13px] text-warning">
            <TriangleAlert className="mt-0.5 size-4 shrink-0" />
            {w}
          </div>
        ))}

        {turn.grounding === 'insufficient_evidence' ? (
          <div className="flex gap-2 rounded-lg bg-warning-soft px-3 py-2 text-[13px] text-warning">
            <AlertTriangle className="mt-0.5 size-4 shrink-0" />
            知识库里没有找到足够的依据，下面的回答没有基于检索内容。
          </div>
        ) : null}

        {turn.content ? <Markdown text={turn.content} /> : null}

        {streaming ? (
          <div className="flex items-center gap-2 text-[13px] text-muted" role="status">
            <Loader2 className="size-4 animate-spin" />
            {waitingForApproval ? '等待你审批工具调用…' : `${turn.phase || '处理中'}…`}
          </div>
        ) : null}

        {turn.status === 'aborted' ? (
          <p className="flex items-center gap-1.5 text-[13px] text-muted">
            <Square className="size-3.5" />
            已停止生成
          </p>
        ) : null}

        {turn.status === 'error' ? (
          <div className="flex gap-2 rounded-lg bg-danger-soft px-3 py-2 text-[13px] text-danger" role="alert">
            <TriangleAlert className="mt-0.5 size-4 shrink-0" />
            {turn.error}
          </div>
        ) : null}

        <SourceList sources={turn.sources} />

        {!streaming ? (
          <div className="flex flex-wrap items-center gap-1 text-xs text-muted">
            {turn.content ? <CopyButton text={turn.content} /> : null}
            {hasProcess ? (
              <button
                onClick={() => setShowProcess((v) => !v)}
                aria-expanded={showProcess}
                className="inline-flex items-center gap-1 rounded-md px-1.5 py-1 hover:bg-surface-2 hover:text-fg"
              >
                <ChevronDown className={cn('size-3.5 transition-transform', showProcess && 'rotate-180')} />
                查看过程
              </button>
            ) : null}
            {latencyMs ? <span className="px-1.5 tabular-nums">用时 {formatDuration(latencyMs)}</span> : null}
          </div>
        ) : null}

        {showProcess && !streaming ? <ProcessPanel trace={turn.trace} traceId={turn.traceId} latencyMs={latencyMs} /> : null}
      </div>
    </div>
  )
})
