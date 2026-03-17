import { useCallback, useState } from 'react'
import { Upload, FileText, X, Loader2 } from 'lucide-react'

interface Props {
  onUpload: (file: File) => Promise<void>
}

export default function DocumentUploader({ onUpload }: Props) {
  const [dragOver, setDragOver] = useState(false)
  const [uploading, setUploading] = useState(false)
  const [fileName, setFileName] = useState('')

  const handleFile = useCallback(
    async (file: File) => {
      setUploading(true)
      setFileName(file.name)
      try {
        await onUpload(file)
      } finally {
        setUploading(false)
        setFileName('')
      }
    },
    [onUpload],
  )

  const handleDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault()
      setDragOver(false)
      const file = e.dataTransfer.files?.[0]
      if (file) handleFile(file)
    },
    [handleFile],
  )

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (file) handleFile(file)
    e.target.value = ''
  }

  return (
    <div
      onDragOver={(e) => {
        e.preventDefault()
        setDragOver(true)
      }}
      onDragLeave={() => setDragOver(false)}
      onDrop={handleDrop}
      className={`relative rounded-xl border-2 border-dashed p-8 text-center transition-all ${
        dragOver
          ? 'border-[var(--color-accent)] bg-[var(--color-accent-light)]'
          : 'border-[var(--color-border)] hover:border-[var(--color-border-subtle)]'
      }`}
    >
      {uploading ? (
        <div className="flex flex-col items-center gap-2">
          <Loader2 size={24} className="text-[var(--color-accent)] animate-spin" />
          <p className="text-sm text-[var(--color-text-secondary)]">Uploading {fileName}...</p>
        </div>
      ) : (
        <>
          <Upload size={24} className="mx-auto mb-2 text-[var(--color-text-muted)]" />
          <p className="text-sm text-[var(--color-text-secondary)] mb-1">
            Drag & drop a file here, or{' '}
            <label className="text-[var(--color-accent)] hover:underline cursor-pointer">
              browse
              <input type="file" className="hidden" onChange={handleChange} accept=".txt,.md,.pdf,.doc,.docx,.csv,.json" />
            </label>
          </p>
          <p className="text-xs text-[var(--color-text-muted)]">
            Supports: TXT, MD, PDF, DOCX, CSV, JSON (max 50MB)
          </p>
        </>
      )}
    </div>
  )
}
