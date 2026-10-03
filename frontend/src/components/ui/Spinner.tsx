import { Loader2 } from 'lucide-react'
import { cn } from '@/lib/cn'

export function Spinner({ className }: { className?: string }) {
  return <Loader2 aria-label="加载中" className={cn('size-5 animate-spin text-muted', className)} />
}

/** Centered spinner for a page or panel that is still loading. */
export function PageSpinner() {
  return (
    <div className="flex h-full min-h-40 items-center justify-center">
      <Spinner className="size-6" />
    </div>
  )
}
