import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { Menu, Sparkles, X } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { Spinner } from '@/components/ui/Spinner'
import { Composer } from '@/pages/chat/Composer'
import { AssistantMessage, UserMessage } from '@/pages/chat/MessageView'
import { SessionList } from '@/pages/chat/SessionList'
import { useChat } from '@/pages/chat/useChat'

function Welcome() {
  return (
    <div className="flex h-full flex-col items-center justify-center gap-3 px-6 text-center">
      <div className="flex size-12 items-center justify-center rounded-2xl bg-primary-soft text-primary">
        <Sparkles className="size-6" />
      </div>
      <h1 className="text-lg font-semibold">有什么想问的？</h1>
      <p className="max-w-md text-[13px] text-muted">
        “检索问答”会先在知识库里找依据再回答；“智能体”会自己决定调用哪些工具。回答下方可以查看引用来源和完整的检索过程。
      </p>
    </div>
  )
}

export default function ChatPage() {
  const { sessionId } = useParams()
  const navigate = useNavigate()
  const [drawer, setDrawer] = useState(false)
  const openSession = useCallback((id: string) => navigate(`/chat/${id}`, { replace: true }), [navigate])
  const { items, loading, loadError, busy, send, stop, settle } = useChat(sessionId, openSession)

  const scroller = useRef<HTMLDivElement>(null)
  const stick = useRef(true)

  useLayoutEffect(() => {
    const el = scroller.current
    if (el && stick.current) el.scrollTop = el.scrollHeight
  }, [items])

  // Switching conversations starts at the end of the history.
  useEffect(() => {
    stick.current = true
  }, [sessionId])

  return (
    <div className="flex h-full">
      <aside className="hidden w-64 shrink-0 border-r border-border bg-surface md:block">
        <SessionList activeId={sessionId} />
      </aside>

      {drawer ? (
        <div className="fixed inset-0 z-40 md:hidden">
          <div className="absolute inset-0 bg-black/45" onClick={() => setDrawer(false)} aria-hidden />
          <aside className="absolute inset-y-0 left-0 w-72 border-r border-border bg-surface shadow-xl">
            <div className="flex h-12 items-center justify-between px-4">
              <span className="font-semibold">历史对话</span>
              <Button variant="ghost" size="icon" onClick={() => setDrawer(false)} aria-label="关闭">
                <X className="size-4" />
              </Button>
            </div>
            <div className="h-[calc(100%-3rem)]">
              <SessionList activeId={sessionId} onNavigate={() => setDrawer(false)} />
            </div>
          </aside>
        </div>
      ) : null}

      <section className="flex min-w-0 flex-1 flex-col">
        <div className="flex h-11 items-center border-b border-border px-3 md:hidden">
          <Button variant="ghost" size="sm" icon={<Menu className="size-4" />} onClick={() => setDrawer(true)}>
            历史对话
          </Button>
        </div>

        <div
          ref={scroller}
          className="min-h-0 flex-1 overflow-y-auto"
          onScroll={(e) => {
            const el = e.currentTarget
            stick.current = el.scrollHeight - el.scrollTop - el.clientHeight < 80
          }}
        >
          {loading ? (
            <div className="flex h-full items-center justify-center">
              <Spinner className="size-6" />
            </div>
          ) : loadError ? (
            <div className="flex h-full flex-col items-center justify-center gap-3 px-6 text-center">
              <p className="text-danger">{loadError}</p>
              <Button onClick={() => navigate('/chat')}>回到新对话</Button>
            </div>
          ) : items.length === 0 ? (
            <Welcome />
          ) : (
            <div className="mx-auto max-w-3xl space-y-6 px-4 py-6">
              {items.map((item) =>
                item.role === 'user' ? (
                  <UserMessage key={item.id} content={item.content} />
                ) : (
                  <AssistantMessage
                    key={item.id}
                    turn={item.turn}
                    latencyMs={item.latencyMs}
                    onSettleApproval={(approvalId, status, reason) => settle(item.id, approvalId, status, reason)}
                  />
                ),
              )}
            </div>
          )}
        </div>

        <Composer
          busy={busy}
          onSend={(text) => {
            stick.current = true
            void send(text)
          }}
          onStop={stop}
        />
      </section>
    </div>
  )
}
