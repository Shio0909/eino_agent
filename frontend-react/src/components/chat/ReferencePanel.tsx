import { useState } from 'react'
import { ChevronDown, ChevronRight, FileText, BarChart3 } from 'lucide-react'
import type { Reference } from '../../types/api'

interface Props {
  references: Reference[]
}

export default function ReferencePanel({ references }: Props) {
  const [expanded, setExpanded] = useState(false)

  if (references.length === 0) return null

  return (
    <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-secondary)] overflow-hidden">
      <button
        onClick={() => setExpanded(!expanded)}
        className="flex items-center gap-2 w-full px-3 py-2 text-xs font-medium text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-tertiary)] transition-colors"
      >
        {expanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
        <FileText size={14} className="text-cyan-400" />
        References ({references.length})
      </button>

      {expanded && (
        <div className="px-3 pb-3 space-y-2">
          {references.map((ref, i) => (
            <div
              key={i}
              className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-primary)] p-3"
            >
              <div className="flex items-center justify-between mb-2">
                <div className="flex items-center gap-2">
                  <FileText size={12} className="text-[var(--color-text-muted)]" />
                  <span className="text-xs font-medium text-[var(--color-text-primary)]">
                    {ref.document_name}
                  </span>
                  <span className="text-xs text-[var(--color-text-muted)]">
                    Chunk #{ref.chunk_index}
                  </span>
                </div>
                <div className="flex items-center gap-1">
                  <BarChart3 size={12} className="text-[var(--color-text-muted)]" />
                  <span className={`text-xs font-medium ${
                    ref.score >= 0.8 ? 'text-green-400' : ref.score >= 0.5 ? 'text-yellow-400' : 'text-red-400'
                  }`}>
                    {(ref.score * 100).toFixed(1)}%
                  </span>
                </div>
              </div>
              <p className="text-xs text-[var(--color-text-secondary)] line-clamp-3 whitespace-pre-wrap">
                {ref.content}
              </p>
              <p className="text-[10px] text-[var(--color-text-muted)] mt-1">
                KB: {ref.knowledge_base_name}
              </p>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
