import { useState } from 'react'
import { ChevronDown, ChevronRight, Brain, Wrench, Eye } from 'lucide-react'
import type { AgentStep } from '../../types/api'

interface Props {
  steps: AgentStep[]
  live?: boolean
}

const stepConfig = {
  thought: { icon: Brain, label: 'Thinking', color: 'text-purple-400', bg: 'bg-purple-500/10', border: 'border-purple-500/30' },
  action: { icon: Wrench, label: 'Action', color: 'text-blue-400', bg: 'bg-blue-500/10', border: 'border-blue-500/30' },
  observation: { icon: Eye, label: 'Observation', color: 'text-green-400', bg: 'bg-green-500/10', border: 'border-green-500/30' },
}

export default function AgentStepCard({ steps, live }: Props) {
  const [expanded, setExpanded] = useState(true)

  if (steps.length === 0) return null

  return (
    <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-secondary)] overflow-hidden">
      <button
        onClick={() => setExpanded(!expanded)}
        className="flex items-center gap-2 w-full px-3 py-2 text-xs font-medium text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-tertiary)] transition-colors"
      >
        {expanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
        <Brain size={14} className="text-purple-400" />
        Agent Steps ({steps.length})
        {live && <span className="ml-auto w-2 h-2 rounded-full bg-green-400 animate-pulse" />}
      </button>

      {expanded && (
        <div className="px-3 pb-3">
          <div className="relative">
            {/* Timeline line */}
            <div className="absolute left-[11px] top-0 bottom-0 w-px bg-[var(--color-border)]" />

            {steps.map((step, i) => {
              const config = stepConfig[step.type]
              const Icon = config.icon
              return (
                <div key={i} className="relative flex gap-3 pb-3 last:pb-0">
                  <div className={`flex-shrink-0 w-6 h-6 rounded-full ${config.bg} border ${config.border} flex items-center justify-center z-10`}>
                    <Icon size={12} className={config.color} />
                  </div>
                  <div className="flex-1 min-w-0 pt-0.5">
                    <div className="flex items-center gap-2 mb-1">
                      <span className={`text-xs font-medium ${config.color}`}>{config.label}</span>
                      {step.tool_name && (
                        <span className="text-xs px-1.5 py-0.5 rounded bg-[var(--color-bg-tertiary)] text-[var(--color-text-muted)]">
                          {step.tool_name}
                        </span>
                      )}
                    </div>
                    <p className="text-xs text-[var(--color-text-secondary)] whitespace-pre-wrap break-words">
                      {step.content}
                    </p>
                    {step.tool_input && (
                      <pre className="mt-1 text-xs p-2 rounded bg-[var(--color-bg-primary)] text-[var(--color-text-muted)] overflow-x-auto">
                        {step.tool_input}
                      </pre>
                    )}
                  </div>
                </div>
              )
            })}
          </div>
        </div>
      )}
    </div>
  )
}
