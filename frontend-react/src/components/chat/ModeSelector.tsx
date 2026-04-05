import { Sparkles, Workflow, Bot } from 'lucide-react'
import type { ChatMode } from '../../types/api'
import { useChatStore } from '../../stores/chat-store'

const modes: { value: ChatMode; label: string; icon: typeof Workflow }[] = [
  { value: 'auto', label: 'Auto', icon: Sparkles },
  { value: 'pipeline', label: 'Pipeline', icon: Workflow },
  { value: 'agentic', label: 'Agentic', icon: Bot },
]

const modeLabels: Record<string, string> = {
  pipeline: 'Pipeline',
  agentic: 'Agentic',
}

export default function ModeSelector() {
  const mode = useChatStore((s) => s.mode)
  const setMode = useChatStore((s) => s.setMode)
  const resolvedMode = useChatStore((s) => s.resolvedMode)

  return (
    <div className="inline-flex items-center gap-1 bg-[var(--color-bg-tertiary)] p-1 rounded-lg border border-[var(--color-border-subtle)]">
      {modes.map(({ value, label, icon: Icon }) => {
        const isActive = mode === value
        const showResolved = isActive && value === 'auto' && resolvedMode && resolvedMode !== 'auto'
        return (
          <button
            key={value}
            onClick={() => setMode(value)}
            className={`flex items-center gap-2 px-3.5 py-2 rounded-md text-sm font-medium whitespace-nowrap transition-colors ${
              isActive
                ? 'bg-[var(--color-bg-card)] shadow-sm text-[var(--color-text-primary)] border border-[var(--color-border-subtle)]'
                : 'text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)]'
            }`}
          >
            <Icon size={16} />
            <span>{showResolved ? `Auto → ${modeLabels[resolvedMode] || resolvedMode}` : label}</span>
          </button>
        )
      })}
    </div>
  )
}
