import { Activity, FileText, Link2, Wrench } from 'lucide-react';
import { Badge } from '../ui/Badge';
import type { ReferenceDocument, TraceStep } from '../../types/api';

export function EvidenceRail({ references, trace, streaming }: { references: ReferenceDocument[]; trace: TraceStep[]; streaming?: boolean }) {
  return (
    <aside className="glass-panel flex h-full min-h-0 flex-col rounded-[2rem] p-5">
      <div className="flex items-center justify-between">
        <div>
          <p className="font-mono text-xs uppercase tracking-[0.22em] text-accent">Evidence</p>
          <h3 className="font-display text-2xl font-semibold">证据流</h3>
        </div>
        {streaming ? <Badge tone="warning">streaming</Badge> : <Badge tone="muted">ready</Badge>}
      </div>
      <div className="mt-5 min-h-0 flex-1 space-y-4 overflow-auto pr-1">
        <section>
          <h4 className="mb-2 flex items-center gap-2 text-sm font-semibold"><FileText className="h-4 w-4" />引用</h4>
          {references.length === 0 ? <p className="text-sm text-muted">暂无引用，发送问题后会在这里显示来源。</p> : references.map((ref, index) => (
            <article key={`${ref.id}-${index}`} className="mb-3 rounded-2xl border border-border/70 bg-surface/45 p-3">
              <div className="flex items-center justify-between gap-2">
                <Badge tone="primary">{Math.round((ref.score ?? 0) * 100) / 100}</Badge>
                <span className="truncate text-xs text-muted">{String(ref.metadata?.wiki_path ?? ref.source ?? ref.id)}</span>
              </div>
              <p className="mt-2 line-clamp-4 text-xs leading-5 text-muted">{ref.content}</p>
            </article>
          ))}
        </section>
        <section>
          <h4 className="mb-2 flex items-center gap-2 text-sm font-semibold"><Activity className="h-4 w-4" />Trace</h4>
          {trace.length === 0 ? <p className="text-sm text-muted">Agent 工具调用、检索阶段和耗时会在这里展开。</p> : trace.map((step, index) => (
            <article key={`${step.type}-${index}`} className="mb-3 rounded-2xl border border-border/70 bg-panel/55 p-3">
              <div className="flex items-center gap-2">
                {step.tool_name ? <Wrench className="h-4 w-4 text-accent" /> : <Link2 className="h-4 w-4 text-primary" />}
                <span className="text-sm font-semibold">{step.stage || step.type}</span>
                {step.latency_ms ? <Badge>{step.latency_ms}ms</Badge> : null}
              </div>
              {step.content ? <p className="mt-2 text-xs leading-5 text-muted">{step.content}</p> : null}
              {step.tool_name ? <p className="mt-2 font-mono text-xs text-muted">{step.tool_name}</p> : null}
            </article>
          ))}
        </section>
      </div>
    </aside>
  );
}
