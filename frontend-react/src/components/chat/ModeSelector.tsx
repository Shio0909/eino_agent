import { Workflow, Bot, GitBranch } from 'lucide-react'
import type { ChatMode } from '../../types/api'
import { useChatStore } from '../../stores/chat-store'

const modes: { value: ChatMode; label: string; icon: typeof Workflow }[] = [
  { value: 'pipeline', label: 'Pipeline', icon: Workflow },
  { value: 'agent', label: 'Agent', icon: Bot },
  { value: 'agentic_rag', label: 'Agentic RAG', icon: GitBranch },
]

export default function ModeSelector() {
  const { mode, setMode } = useChatStore()

  return (
    <div className="inline-flex items-center gap-1 bg-[var(--color-bg-tertiary)] p-1 rounded-lg border border-[var(--color-border-subtle)]">
      {modes.map(({ value, label, icon: Icon }) => (
        <button
          key={value}
          onClick={() => setMode(value)}
          className={`flex items-center gap-2 px-3.5 py-2 rounded-md text-sm font-medium whitespace-nowrap transition-colors ${
            mode === value
              ? 'bg-[var(--color-bg-card)] shadow-sm text-[var(--color-text-primary)] border border-[var(--color-border-subtle)]'
              : 'text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)]'
          }`}
        >
          <Icon size={16} />
          <span>{label}</span>
        </button>
      ))}
    </div>
  )
}
