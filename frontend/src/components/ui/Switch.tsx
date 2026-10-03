import { cn } from '@/lib/cn'

interface SwitchProps {
  checked: boolean
  onChange: (checked: boolean) => void
  label: string
  description?: string
  disabled?: boolean
  className?: string
}

/** A labelled on/off control; the whole row toggles it. */
export function Switch({ checked, onChange, label, description, disabled, className }: SwitchProps) {
  return (
    <label className={cn('flex cursor-pointer items-center justify-between gap-4 py-2 has-[:disabled]:cursor-not-allowed has-[:disabled]:opacity-60', className)}>
      <span className="min-w-0">
        <span className="block text-[13px] font-medium">{label}</span>
        {description ? <span className="block text-xs text-muted">{description}</span> : null}
      </span>
      <input type="checkbox" role="switch" className="peer sr-only" checked={checked} disabled={disabled} onChange={(e) => onChange(e.target.checked)} />
      <span
        aria-hidden
        className={cn(
          'relative h-5 w-9 shrink-0 rounded-full bg-border transition-colors peer-checked:bg-primary',
          'peer-focus-visible:ring-2 peer-focus-visible:ring-primary/40',
          "after:absolute after:top-0.5 after:left-0.5 after:size-4 after:rounded-full after:bg-white after:shadow after:transition-transform after:content-['']",
          'peer-checked:after:translate-x-4',
        )}
      />
    </label>
  )
}
