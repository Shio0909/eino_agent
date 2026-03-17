import { FileText, Trash2, Eye, Loader2, AlertCircle, CheckCircle2, Clock } from 'lucide-react'
import type { Document } from '../../types/api'
import { formatFileSize, formatTime } from '../../lib/utils'

interface Props {
  documents: Document[]
  onDelete: (docId: string) => void
  onViewChunks: (docId: string) => void
}

const statusConfig: Record<string, { icon: any, color: string, bg: string, spin?: boolean }> = {
  pending: { icon: Clock, color: 'text-yellow-400', bg: 'bg-yellow-500/15' },
  parsing: { icon: Loader2, color: 'text-blue-400', bg: 'bg-blue-500/15', spin: true },
  parsed: { icon: CheckCircle2, color: 'text-green-400', bg: 'bg-green-500/15' },
  parse_failed: { icon: AlertCircle, color: 'text-red-400', bg: 'bg-red-500/15' },
  embedding: { icon: Loader2, color: 'text-blue-400', bg: 'bg-blue-500/15', spin: true },
  completed: { icon: CheckCircle2, color: 'text-green-400', bg: 'bg-green-500/15' },
  embed_failed: { icon: AlertCircle, color: 'text-red-400', bg: 'bg-red-500/15' },
}

export default function DocumentTable({ documents, onDelete, onViewChunks }: Props) {
  if (documents.length === 0) {
    return (
      <div className="text-center py-12 text-[var(--color-text-muted)]">
        <FileText size={32} className="mx-auto mb-2 opacity-40" />
        <p className="text-sm">No documents uploaded yet</p>
      </div>
    )
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-[var(--color-border)] text-left text-xs text-[var(--color-text-muted)]">
            <th className="pb-2 font-medium">Name</th>
            <th className="pb-2 font-medium">Type</th>
            <th className="pb-2 font-medium">Size</th>
            <th className="pb-2 font-medium">Chunks</th>
            <th className="pb-2 font-medium">Status</th>
            <th className="pb-2 font-medium">Uploaded</th>
            <th className="pb-2 font-medium text-right">Actions</th>
          </tr>
        </thead>
        <tbody>
          {documents.map((doc) => {
            const st = statusConfig[doc.parse_status] || { icon: AlertCircle, color: 'text-gray-400', bg: 'bg-gray-500/15' }
            const Icon = st.icon
            return (
              <tr key={doc.id} className="border-b border-[var(--color-border)] hover:bg-[var(--color-bg-tertiary)] transition-colors">
                <td className="py-3 pr-4">
                  <div className="flex items-center gap-2">
                    <FileText size={14} className="text-[var(--color-text-muted)]" />
                    <span className="text-[var(--color-text-primary)] truncate max-w-[200px]">{doc.filename}</span>
                  </div>
                </td>
                <td className="py-3 pr-4 text-[var(--color-text-muted)]">
                  <span className="px-1.5 py-0.5 rounded bg-[var(--color-bg-tertiary)] text-xs">{doc.file_type}</span>
                </td>
                <td className="py-3 pr-4 text-[var(--color-text-muted)]">{formatFileSize(doc.file_size)}</td>
                <td className="py-3 pr-4 text-[var(--color-text-muted)]">{doc.chunk_count}</td>
                <td className="py-3 pr-4">
                  <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs ${st.bg} ${st.color}`}>
                    <Icon size={12} className={(st as any).spin ? 'animate-spin' : ''} />
                    {doc.parse_status}
                  </span>
                  {doc.error_message && (
                    <p className="text-[10px] text-red-400 mt-0.5">{doc.error_message}</p>
                  )}
                </td>
                <td className="py-3 pr-4 text-[var(--color-text-muted)] text-xs">{formatTime(doc.created_at)}</td>
                <td className="py-3 text-right">
                  <div className="flex items-center justify-end gap-1">
                    {doc.parse_status === 'completed' && (
                      <button
                        onClick={() => onViewChunks(doc.id)}
                        className="p-1.5 rounded hover:bg-[var(--color-bg-secondary)] text-[var(--color-text-muted)] hover:text-indigo-400 transition-colors"
                        title="View chunks"
                      >
                        <Eye size={14} />
                      </button>
                    )}
                    <button
                      onClick={() => onDelete(doc.id)}
                      className="p-1.5 rounded hover:bg-red-500/20 text-[var(--color-text-muted)] hover:text-red-400 transition-colors"
                      title="Delete document"
                    >
                      <Trash2 size={14} />
                    </button>
                  </div>
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}
