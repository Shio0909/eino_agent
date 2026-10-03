import { useState } from 'react'
import { NavLink, useNavigate } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { MessageSquarePlus, Trash2 } from 'lucide-react'
import { ConfirmDialog } from '@/components/ui/Dialog'
import { Button } from '@/components/ui/Button'
import { Spinner } from '@/components/ui/Spinner'
import { apiDelete, apiGet, errorMessage } from '@/lib/api'
import { cn } from '@/lib/cn'
import { formatRelative } from '@/lib/format'
import { toast } from '@/store/toast'
import type { Session, SessionsResponse } from '@/types/chat'

interface SessionListProps {
  activeId?: string
  onNavigate?: () => void
}

export function SessionList({ activeId, onNavigate }: SessionListProps) {
  const navigate = useNavigate()
  const qc = useQueryClient()
  const [pendingDelete, setPendingDelete] = useState<Session | null>(null)

  const sessions = useQuery({ queryKey: ['sessions'], queryFn: () => apiGet<SessionsResponse>('/sessions') })

  const remove = useMutation({
    mutationFn: (id: string) => apiDelete(`/sessions/${id}`),
    onSuccess: (_, id) => {
      void qc.invalidateQueries({ queryKey: ['sessions'] })
      setPendingDelete(null)
      if (id === activeId) navigate('/chat')
      toast.success('对话已删除')
    },
    onError: (err) => toast.error(errorMessage(err)),
  })

  const list = sessions.data?.sessions ?? []

  return (
    <div className="flex h-full flex-col">
      <div className="p-3">
        <Button
          variant="primary"
          className="w-full"
          icon={<MessageSquarePlus className="size-4" />}
          onClick={() => {
            navigate('/chat')
            onNavigate?.()
          }}
        >
          新对话
        </Button>
      </div>

      <nav className="min-h-0 flex-1 space-y-0.5 overflow-y-auto px-2 pb-3" aria-label="历史对话">
        {sessions.isPending ? (
          <div className="flex justify-center py-6">
            <Spinner />
          </div>
        ) : sessions.isError ? (
          <div className="px-3 py-4 text-[13px] text-danger">
            {errorMessage(sessions.error)}
            <button className="ml-2 underline" onClick={() => void sessions.refetch()}>
              重试
            </button>
          </div>
        ) : list.length === 0 ? (
          <p className="px-3 py-6 text-center text-[13px] text-muted">{sessions.data?.message ?? '还没有对话'}</p>
        ) : (
          list.map((s) => (
            <div key={s.id} className="group relative">
              <NavLink
                to={`/chat/${s.id}`}
                onClick={onNavigate}
                className={cn(
                  'block rounded-lg px-3 py-2 pr-9 transition-colors',
                  s.id === activeId ? 'bg-primary-soft text-primary' : 'hover:bg-surface-2',
                )}
              >
                <p className="truncate text-[13.5px] font-medium">{s.title || '新对话'}</p>
                <p className={cn('text-xs', s.id === activeId ? 'text-primary/70' : 'text-muted')}>{formatRelative(s.updated_at)}</p>
              </NavLink>
              <button
                aria-label={`删除对话 ${s.title || '新对话'}`}
                onClick={() => setPendingDelete(s)}
                className="absolute top-1/2 right-1.5 -translate-y-1/2 rounded-md p-1.5 text-muted opacity-0 transition-opacity group-hover:opacity-100 hover:bg-danger-soft hover:text-danger focus-visible:opacity-100 max-md:opacity-100"
              >
                <Trash2 className="size-3.5" />
              </button>
            </div>
          ))
        )}
      </nav>

      <ConfirmDialog
        open={!!pendingDelete}
        title="删除对话"
        danger
        confirmLabel="删除"
        loading={remove.isPending}
        message={`确定删除“${pendingDelete?.title || '新对话'}”吗？消息和过程记录会一并删除，无法恢复。`}
        onConfirm={() => pendingDelete && remove.mutate(pendingDelete.id)}
        onClose={() => setPendingDelete(null)}
      />
    </div>
  )
}
