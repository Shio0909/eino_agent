import { cn } from '../../lib/utils'

interface TabsProps {
  tabs: readonly string[]
  active: string
  onChange: (tab: string) => void
  className?: string
}

export function Tabs({ tabs, active, onChange, className }: TabsProps) {
  return (
    <div className={cn('flex gap-1 overflow-x-auto', className)}>
      {tabs.map((tab) => (
        <button
          key={tab}
          onClick={() => onChange(tab)}
          className={cn(
            'px-5 py-3 text-sm whitespace-nowrap font-medium border-b-2 transition-colors',
            active === tab
              ? 'border-[var(--color-accent)] text-[var(--color-accent)]'
              : 'border-transparent text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)]',
          )}
        >
          {tab}
        </button>
      ))}
    </div>
  )
}
