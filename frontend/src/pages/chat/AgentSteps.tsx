import { AlertCircle, CheckCircle2, CircleSlash, Loader2, Wrench } from 'lucide-react'
import type { AgentStep } from '@/lib/chatTurn'
import { formatDuration, prettyJson, truncate } from '@/lib/format'

const ICON = {
  running: <Loader2 className="size-4 animate-spin text-primary" />,
  done: <CheckCircle2 className="size-4 text-success" />,
  error: <AlertCircle className="size-4 text-danger" />,
  aborted: <CircleSlash className="size-4 text-muted" />,
}

const LABEL = { running: '执行中', done: '完成', error: '失败', aborted: '已中止' }

export function AgentSteps({ steps }: { steps: AgentStep[] }) {
  if (steps.length === 0) return null
  return (
    <section aria-label="工具调用">
      <h3 className="mb-1.5 flex items-center gap-1.5 text-xs font-medium text-muted">
        <Wrench className="size-3.5" />
        工具调用 ({steps.length})
      </h3>
      <ol className="space-y-1.5">
        {steps.map((s) => (
          <li key={s.id} className="rounded-lg border border-border bg-surface">
            <details>
              <summary className="flex cursor-pointer items-center gap-2 px-3 py-2 text-[13px]">
                {ICON[s.status]}
                <code className="font-mono font-medium">{s.toolName || '工具'}</code>
                <span className="text-xs text-muted">{LABEL[s.status]}</span>
                {s.latencyMs !== undefined ? <span className="ml-auto text-xs text-muted tabular-nums">{formatDuration(s.latencyMs)}</span> : null}
              </summary>
              <div className="space-y-2 border-t border-border px-3 py-2">
                {s.toolInput ? (
                  <div>
                    <p className="mb-1 text-xs text-muted">参数</p>
                    <pre className="max-h-40 overflow-auto rounded-lg bg-code p-2 font-mono text-xs whitespace-pre-wrap">{prettyJson(s.toolInput)}</pre>
                  </div>
                ) : null}
                {s.observation || s.error ? (
                  <div>
                    <p className="mb-1 text-xs text-muted">{s.status === 'error' ? '错误' : '结果'}</p>
                    <pre
                      className={`max-h-48 overflow-auto rounded-lg p-2 font-mono text-xs whitespace-pre-wrap ${s.status === 'error' ? 'bg-danger-soft text-danger' : 'bg-code'}`}
                    >
                      {truncate(s.error || s.observation || '', 3000)}
                    </pre>
                  </div>
                ) : null}
              </div>
            </details>
          </li>
        ))}
      </ol>
    </section>
  )
}
