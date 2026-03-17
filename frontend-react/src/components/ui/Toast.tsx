import { useEffect, useState, useCallback } from 'react'
import { CheckCircle2, AlertCircle, Info, X } from 'lucide-react'
import { cn } from '../../lib/utils'

type ToastType = 'success' | 'error' | 'info'

interface Toast {
  id: string
  message: string
  type: ToastType
}

let addToastFn: ((message: string, type?: ToastType) => void) | null = null

export function toast(message: string, type: ToastType = 'success') {
  addToastFn?.(message, type)
}

const icons = {
  success: CheckCircle2,
  error: AlertCircle,
  info: Info,
}

const styles = {
  success: 'border-green-500/30 bg-green-500/10',
  error: 'border-[var(--color-error)]/30 bg-[var(--color-error)]/10',
  info: 'border-blue-500/30 bg-blue-500/10',
}

export function ToastContainer() {
  const [toasts, setToasts] = useState<Toast[]>([])

  const addToast = useCallback((message: string, type: ToastType = 'success') => {
    const id = Date.now().toString()
    setToasts((prev) => [...prev, { id, message, type }])
    setTimeout(() => {
      setToasts((prev) => prev.filter((t) => t.id !== id))
    }, 4000)
  }, [])

  useEffect(() => {
    addToastFn = addToast
    return () => { addToastFn = null }
  }, [addToast])

  if (toasts.length === 0) return null

  return (
    <div className="fixed bottom-6 right-6 z-[100] flex flex-col gap-2">
      {toasts.map((t) => {
        const Icon = icons[t.type]
        return (
          <div
            key={t.id}
            className={cn(
              'flex items-center gap-3 px-4 py-3 rounded-xl border shadow-lg backdrop-blur-sm bg-[var(--color-bg-card)] text-sm text-[var(--color-text-primary)] animate-in slide-in-from-right',
              styles[t.type],
            )}
          >
            <Icon size={16} className={t.type === 'success' ? 'text-green-400' : t.type === 'error' ? 'text-[var(--color-error)]' : 'text-blue-400'} />
            <span className="flex-1">{t.message}</span>
            <button
              onClick={() => setToasts((prev) => prev.filter((x) => x.id !== t.id))}
              className="p-0.5 rounded hover:bg-[var(--color-bg-tertiary)] text-[var(--color-text-muted)]"
            >
              <X size={14} />
            </button>
          </div>
        )
      })}
    </div>
  )
}
