import { useState, useCallback } from 'react'
import { ChevronDown, ChevronRight, FileText, BarChart3 } from 'lucide-react'
import type { Reference } from '../../types/api'

interface Props {
  references: Reference[]
}

export default function ReferencePanel({ references }: Props) {
  const [expanded, setExpanded] = useState(false)
  const [expandedItems, setExpandedItems] = useState<Set<number>>(new Set())

  const toggleItem = useCallback((index: number) => {
    setExpandedItems((prev) => {
      const next = new Set(prev)
      if (next.has(index)) next.delete(index)
      else next.add(index)
      return next
    })
  }, [])

  if (references.length === 0) return null

  return (
    <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-secondary)] overflow-hidden">
      <button
        onClick={() => setExpanded(!expanded)}
        className="flex items-center gap-2 w-full px-3 py-2 text-xs font-medium text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-tertiary)] transition-colors"
      >
        {expanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
        <FileText size={14} className="text-cyan-400" />
        引用来源 ({references.length})
      </button>

      {expanded && (
        <div className="px-3 pb-3 space-y-2">
          {references.map((ref, i) => {
            const isOpen = expandedItems.has(i)
            return (
              <div
                key={i}
                className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-primary)] overflow-hidden transition-all"
              >
                {/* Header — always visible, clickable */}
                <button
                  onClick={() => toggleItem(i)}
                  className="flex items-center justify-between w-full px-3 py-2.5 text-left hover:bg-[var(--color-bg-tertiary)] transition-colors"
                >
                  <div className="flex items-center gap-2 min-w-0">
                    {isOpen ? <ChevronDown size={12} className="shrink-0 text-[var(--color-text-muted)]" /> : <ChevronRight size={12} className="shrink-0 text-[var(--color-text-muted)]" />}
                    <FileText size={12} className="shrink-0 text-[var(--color-text-muted)]" />
                    <span className="text-xs font-medium text-[var(--color-text-primary)] truncate">
                      {ref.document_name}
                    </span>
                    <span className="text-xs text-[var(--color-text-muted)] shrink-0">
                      Chunk #{ref.chunk_index}
                    </span>
                  </div>
                  <div className="flex items-center gap-1.5 shrink-0 ml-2">
                    <BarChart3 size={12} className="text-[var(--color-text-muted)]" />
                    <span className={`text-xs font-medium ${
                      ref.score >= 0.8 ? 'text-green-400' : ref.score >= 0.5 ? 'text-yellow-400' : 'text-red-400'
                    }`}>
                      {(ref.score * 100).toFixed(1)}%
                    </span>
                  </div>
                </button>

                {/* Summary (collapsed) — 2-line preview */}
                {!isOpen && (
                  <div className="px-3 pb-2.5">
                    <p className="text-xs text-[var(--color-text-secondary)] line-clamp-2 whitespace-pre-wrap">
                      {ref.content}
                    </p>
                  </div>
                )}

                {/* Full content (expanded) */}
                {isOpen && (
                  <div className="px-3 pb-3 border-t border-[var(--color-border-subtle)]">
                    <p className="text-xs text-[var(--color-text-secondary)] whitespace-pre-wrap mt-2.5 leading-relaxed bg-[var(--color-bg-secondary)] rounded-lg p-3 max-h-[400px] overflow-y-auto">
                      {ref.content}
                    </p>
                    <p className="text-[10px] text-[var(--color-text-muted)] mt-1.5">
                      KB: {ref.knowledge_base_name}
                    </p>
                  </div>
                )}
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
