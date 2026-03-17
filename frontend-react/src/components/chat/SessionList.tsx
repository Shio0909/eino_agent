import { useEffect } from 'react'
import { Trash2 } from 'lucide-react'
import { useChatStore } from '../../stores/chat-store'
import { truncate } from '../../lib/utils'

export default function SessionList() {
  const { sessions, currentSessionId, loadSessions, selectSession, deleteSession } =
    useChatStore()

  useEffect(() => {
    loadSessions()
  }, [loadSessions])

  return (
    <div className="space-y-0.5">
      {sessions.map((session) => (
        <div
          key={session.id}
          onClick={() => selectSession(session.id)}
          className={`group flex items-center gap-2 px-4 py-2 rounded-lg cursor-pointer transition-colors ${
            session.id === currentSessionId
              ? 'bg-[var(--color-bg-hover)] text-[var(--color-text-primary)]'
              : 'text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-hover)]'
          }`}
        >
          <span className="flex-1 truncate text-sm">{truncate(session.title, 24)}</span>
          <button
            onClick={(e) => {
              e.stopPropagation()
              deleteSession(session.id)
            }}
            className="opacity-0 group-hover:opacity-100 p-1 rounded hover:text-[var(--color-error)] transition-all shrink-0"
          >
            <Trash2 size={14} />
          </button>
        </div>
      ))}

      {sessions.length === 0 && (
        <p className="text-center text-sm text-[var(--color-text-muted)] py-8">暂无对话</p>
      )}
    </div>
  )
}
