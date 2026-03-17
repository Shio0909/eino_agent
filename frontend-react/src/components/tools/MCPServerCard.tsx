import { Server, KeyRound, Plug } from 'lucide-react'
import type { MCPServer } from '../../types/api'

interface Props {
  server: MCPServer
}

export default function MCPServerCard({ server }: Props) {
  return (
    <div className="rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-secondary)] p-4">
      <div className="flex items-start justify-between mb-3">
        <div className="flex items-center gap-2">
          <div className="w-9 h-9 rounded-lg bg-purple-500/15 flex items-center justify-center">
            <Server size={16} className="text-purple-400" />
          </div>
          <div>
            <h3 className="text-sm font-semibold text-[var(--color-text-primary)]">{server.name}</h3>
            <p className="text-xs text-[var(--color-text-muted)] font-mono">{server.endpoint}</p>
          </div>
        </div>
        <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs bg-green-500/15 text-green-400">
          <Plug size={12} />
          Active
        </span>
      </div>

      <div className="space-y-1 text-xs text-[var(--color-text-muted)]">
        <p>transport: {server.transport}</p>
        <p className="inline-flex items-center gap-1">
          <KeyRound size={12} />
          {server.has_api_key ? 'with api key' : 'no api key'}
        </p>
      </div>
    </div>
  )
}
