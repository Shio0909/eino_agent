import { BookOpen } from 'lucide-react'
import { truncate } from '@/lib/format'
import type { SourceRef } from '@/lib/trace'

export function SourceList({ sources }: { sources: SourceRef[] }) {
  if (sources.length === 0) return null
  return (
    <section aria-label="引用来源">
      <h3 className="mb-1.5 flex items-center gap-1.5 text-xs font-medium text-muted">
        <BookOpen className="size-3.5" />
        引用来源 ({sources.length})
      </h3>
      <ol className="space-y-1.5">
        {sources.map((s, i) => (
          <li key={s.key} className="rounded-lg border border-border bg-surface">
            <details>
              <summary className="flex cursor-pointer items-center gap-2 px-3 py-2 text-[13px]">
                <span className="flex size-5 shrink-0 items-center justify-center rounded bg-primary-soft text-xs font-semibold text-primary">{i + 1}</span>
                <span className="min-w-0 flex-1 truncate font-medium">{s.name}</span>
                {s.score ? <span className="text-xs text-muted tabular-nums">{s.score.toFixed(3)}</span> : null}
              </summary>
              <p className="border-t border-border px-3 py-2 text-[13px] whitespace-pre-wrap text-muted">{truncate(s.content, 1200) || '（无正文）'}</p>
            </details>
          </li>
        ))}
      </ol>
    </section>
  )
}
