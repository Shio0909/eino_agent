import { X, Hash, Type, FileText } from 'lucide-react'
import type { Chunk } from '../../types/api'

interface Props {
  chunks: Chunk[]
  documentName: string
  onClose: () => void
}

export default function ChunkPreview({ chunks, documentName, onClose }: Props) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
      <div className="w-full max-w-3xl max-h-[80vh] bg-[var(--color-bg-primary)] rounded-xl border border-[var(--color-border)] shadow-2xl flex flex-col">
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-4 border-b border-[var(--color-border)]">
          <div className="flex items-center gap-2">
            <FileText size={16} className="text-[var(--color-accent)]" />
            <h2 className="text-sm font-semibold text-[var(--color-text-primary)]">
              Chunks — {documentName}
            </h2>
            <span className="text-xs text-[var(--color-text-muted)]">({chunks.length} chunks)</span>
          </div>
          <button
            onClick={onClose}
            className="p-1.5 rounded-lg hover:bg-[var(--color-bg-tertiary)] text-[var(--color-text-muted)] transition-colors"
          >
            <X size={16} />
          </button>
        </div>

        {/* Chunks list */}
        <div className="flex-1 overflow-y-auto p-5 space-y-3">
          {chunks.map((chunk) => (
            <div
              key={chunk.id}
              className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-secondary)] p-4"
            >
              <div className="flex items-center gap-3 mb-2">
                <span className="flex items-center gap-1 text-xs font-medium text-[var(--color-accent)]">
                  <Hash size={12} />
                  Chunk {chunk.index + 1}
                </span>
                <span className="flex items-center gap-1 text-xs text-[var(--color-text-muted)]">
                  <Type size={12} />
                  {chunk.char_count} chars
                </span>
                {chunk.token_count > 0 && (
                  <span className="text-xs text-[var(--color-text-muted)]">
                    {chunk.token_count} tokens
                  </span>
                )}
              </div>
              <p className="text-xs text-[var(--color-text-secondary)] whitespace-pre-wrap leading-relaxed">
                {chunk.content}
              </p>
            </div>
          ))}

          {chunks.length === 0 && (
            <p className="text-center text-sm text-[var(--color-text-muted)] py-8">No chunks found</p>
          )}
        </div>
      </div>
    </div>
  )
}
