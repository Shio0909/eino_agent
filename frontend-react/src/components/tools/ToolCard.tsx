import { Wrench, Server, ChevronDown, ChevronRight } from 'lucide-react'
import { useState } from 'react'
import type { MCPTool } from '../../types/api'

interface Props {
  tool: MCPTool
}

export default function ToolCard({ tool }: Props) {
  const [expanded, setExpanded] = useState(false)

  return (
    <div className="rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-secondary)] p-4">
      <div className="flex items-start gap-3">
        <div className="w-8 h-8 rounded-lg bg-blue-500/15 flex items-center justify-center flex-shrink-0">
          <Wrench size={14} className="text-blue-400" />
        </div>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 mb-1">
            <h3 className="text-sm font-semibold text-[var(--color-text-primary)]">{tool.name}</h3>
            <span className="flex items-center gap-1 text-[10px] px-1.5 py-0.5 rounded bg-[var(--color-bg-tertiary)] text-[var(--color-text-muted)]">
              <Server size={10} />
              {tool.server_name}
            </span>
          </div>
          <p className="text-xs text-[var(--color-text-secondary)] mb-2">{tool.description}</p>

          {tool.input_schema && Object.keys(tool.input_schema).length > 0 && (
            <>
              <button
                onClick={() => setExpanded(!expanded)}
                className="flex items-center gap-1 text-xs text-[var(--color-text-muted)] hover:text-[var(--color-text-secondary)] transition-colors"
              >
                {expanded ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
                Input Schema
              </button>
              {expanded && (
                <pre className="mt-2 p-3 rounded-lg bg-[var(--color-bg-primary)] text-xs text-[var(--color-text-secondary)] overflow-x-auto">
                  {JSON.stringify(tool.input_schema, null, 2)}
                </pre>
              )}
            </>
          )}
        </div>
      </div>
    </div>
  )
}
