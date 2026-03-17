import { BookOpen, FileText, Layers, Trash2 } from 'lucide-react'
import type { KnowledgeBase } from '../../types/api'
import { formatTime } from '../../lib/utils'

interface Props {
  kb: KnowledgeBase
  selected: boolean
  onSelect: () => void
  onDelete: () => void
}

export default function KBCard({ kb, selected, onSelect, onDelete }: Props) {
  return (
    <div
      onClick={onSelect}
      className={`group relative rounded-xl border p-4 cursor-pointer transition-all hover:shadow-md ${
        selected
          ? 'border-[var(--color-accent)] bg-[var(--color-accent-light)] shadow-sm'
          : 'border-[var(--color-border)] bg-[var(--color-bg-secondary)] hover:border-[var(--color-border-subtle)]'
      }`}
    >
      <div className="flex items-start justify-between mb-3">
        <div className="flex items-center gap-2">
          <div className="w-9 h-9 rounded-lg bg-[var(--color-accent-light)] flex items-center justify-center">
            <BookOpen size={16} className="text-[var(--color-accent)]" />
          </div>
          <div>
            <h3 className="text-sm font-semibold text-[var(--color-text-primary)]">{kb.name}</h3>
            <p className="text-xs text-[var(--color-text-muted)]">{formatTime(kb.created_at)}</p>
          </div>
        </div>
        <button
          onClick={(e) => {
            e.stopPropagation()
            onDelete()
          }}
          className="opacity-0 group-hover:opacity-100 p-1.5 rounded-lg hover:bg-red-500/20 hover:text-red-400 text-[var(--color-text-muted)] transition-all"
        >
          <Trash2 size={14} />
        </button>
      </div>

      {kb.description && (
        <p className="text-xs text-[var(--color-text-secondary)] mb-3 line-clamp-2">{kb.description}</p>
      )}

      <div className="flex items-center gap-4 text-xs text-[var(--color-text-muted)]">
        <span className="flex items-center gap-1">
          <FileText size={12} /> {kb.document_count} docs
        </span>
        <span className="flex items-center gap-1">
          <Layers size={12} /> {kb.chunk_count} chunks
        </span>
      </div>

      {kb.embedding_model && (
        <div className="mt-2">
          <span className="text-[10px] px-1.5 py-0.5 rounded bg-[var(--color-bg-tertiary)] text-[var(--color-text-muted)]">
            {kb.embedding_model}
          </span>
        </div>
      )}
    </div>
  )
}
