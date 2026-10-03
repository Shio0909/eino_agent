import { CircleAlert, CircleCheck, Info, X } from 'lucide-react'
import { useToasts, type ToastKind } from '@/store/toast'
import { cn } from '@/lib/cn'

const styles: Record<ToastKind, { icon: typeof Info; tone: string }> = {
  success: { icon: CircleCheck, tone: 'text-success' },
  error: { icon: CircleAlert, tone: 'text-danger' },
  info: { icon: Info, tone: 'text-primary' },
}

export function Toaster() {
  const { items, dismiss } = useToasts()
  return (
    <div className="pointer-events-none fixed right-4 bottom-4 z-[60] flex w-80 max-w-[calc(100vw-2rem)] flex-col gap-2" aria-live="polite">
      {items.map((t) => {
        const { icon: Icon, tone } = styles[t.kind]
        return (
          <div key={t.id} role="status" className="pointer-events-auto flex items-start gap-2.5 rounded-lg border border-border bg-surface px-3.5 py-3 shadow-lg">
            <Icon className={cn('mt-0.5 size-4 shrink-0', tone)} />
            <p className="min-w-0 flex-1 text-[13px] break-words">{t.message}</p>
            <button onClick={() => dismiss(t.id)} aria-label="关闭提示" className="text-muted hover:text-fg">
              <X className="size-3.5" />
            </button>
          </div>
        )
      })}
    </div>
  )
}
