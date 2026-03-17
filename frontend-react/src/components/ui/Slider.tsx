import { cn } from '../../lib/utils'

interface SliderProps {
  value: number
  onChange: (value: number) => void
  min: number
  max: number
  step: number
  label?: string
  showValue?: boolean
  className?: string
}

export function Slider({ value, onChange, min, max, step, label, showValue = true, className }: SliderProps) {
  return (
    <div className={cn('space-y-2', className)}>
      {(label || showValue) && (
        <div className="flex items-center justify-between">
          {label && <label className="text-sm font-medium text-[var(--color-text-secondary)]">{label}</label>}
          {showValue && (
            <span className="text-sm font-mono text-[var(--color-text-muted)] tabular-nums">{value}</span>
          )}
        </div>
      )}
      <input
        type="range"
        min={min}
        max={max}
        step={step}
        value={value}
        onChange={(e) => onChange(Number(e.target.value))}
        className="w-full accent-[var(--color-accent)] h-1.5 rounded-full appearance-none bg-[var(--color-bg-tertiary)] cursor-pointer"
      />
    </div>
  )
}
