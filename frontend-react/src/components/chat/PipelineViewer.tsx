import { useState } from 'react'
import { ChevronDown, ChevronRight, Search, FileSearch, ArrowUpDown, Sparkles } from 'lucide-react'

interface PipelineStep {
  name: string
  status: 'pending' | 'running' | 'done'
  detail?: string
}

interface Props {
  status: string
  steps?: PipelineStep[]
}

const pipelineStages = [
  { key: 'rewrite', label: 'Query Rewrite', icon: Search },
  { key: 'retrieve', label: 'Retrieval', icon: FileSearch },
  { key: 'rerank', label: 'Reranking', icon: ArrowUpDown },
  { key: 'generate', label: 'Generation', icon: Sparkles },
]

export default function PipelineViewer({ status }: Props) {
  const [expanded, setExpanded] = useState(true)

  if (!status) return null

  // Determine current stage from status string
  const currentStage = status.toLowerCase()

  return (
    <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-secondary)] overflow-hidden">
      <button
        onClick={() => setExpanded(!expanded)}
        className="flex items-center gap-2 w-full px-3 py-2 text-xs font-medium text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-tertiary)] transition-colors"
      >
        {expanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
        <Sparkles size={14} className="text-amber-400" />
        Pipeline Status
        <span className="ml-auto text-[10px] text-[var(--color-text-muted)]">{status}</span>
      </button>

      {expanded && (
        <div className="px-3 pb-3">
          <div className="flex items-center gap-1">
            {pipelineStages.map(({ key, label, icon: Icon }, i) => {
              const isActive = currentStage.includes(key)
              const isPast = pipelineStages.findIndex((s) => currentStage.includes(s.key)) > i

              return (
                <div key={key} className="flex items-center gap-1 flex-1">
                  <div
                    className={`flex items-center gap-1.5 px-2 py-1.5 rounded text-xs transition-all ${
                      isActive
                        ? 'bg-amber-500/15 text-amber-400 font-medium'
                        : isPast
                        ? 'text-green-400 opacity-70'
                        : 'text-[var(--color-text-muted)] opacity-40'
                    }`}
                  >
                    <Icon size={12} />
                    <span className="hidden sm:inline">{label}</span>
                    {isActive && <span className="w-1.5 h-1.5 rounded-full bg-amber-400 animate-pulse" />}
                  </div>
                  {i < pipelineStages.length - 1 && (
                    <div className={`flex-shrink-0 w-4 h-px ${isPast ? 'bg-green-500/50' : 'bg-[var(--color-border)]'}`} />
                  )}
                </div>
              )
            })}
          </div>
        </div>
      )}
    </div>
  )
}
