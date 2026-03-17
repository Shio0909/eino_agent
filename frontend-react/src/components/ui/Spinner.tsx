import { Loader2 } from 'lucide-react'
import { cn } from '../../lib/utils'

interface SpinnerProps {
  size?: number
  className?: string
}

export function Spinner({ size = 24, className }: SpinnerProps) {
  return <Loader2 size={size} className={cn('animate-spin text-[var(--color-accent)]', className)} />
}

export function PageSpinner() {
  return (
    <div className="flex items-center justify-center h-full">
      <Spinner size={28} className="text-[var(--color-text-muted)]" />
    </div>
  )
}
