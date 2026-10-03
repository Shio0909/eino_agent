import { useEffect, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { ArrowUp, Bot, ChevronDown, Database, Square, Zap } from 'lucide-react'
import { apiGet } from '@/lib/api'
import { cn } from '@/lib/cn'
import { useChatPrefs } from '@/store/chatPrefs'
import type { ChatMode } from '@/types/chat'
import type { KnowledgeBaseListResponse } from '@/types/knowledge'

const MODES: { id: ChatMode; label: string; hint: string; icon: typeof Zap }[] = [
  { id: 'pipeline', label: '检索问答', hint: '先检索知识库，再基于检索内容回答，速度快', icon: Zap },
  { id: 'agentic', label: '智能体', hint: '模型自行决定调用哪些工具，适合复杂问题，耗时更长', icon: Bot },
]

function KnowledgePicker() {
  const kbIds = useChatPrefs((s) => s.kbIds)
  const setKbIds = useChatPrefs((s) => s.setKbIds)
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)
  const kbs = useQuery({
    queryKey: ['knowledge-bases', 'picker'],
    queryFn: () => apiGet<KnowledgeBaseListResponse>('/knowledge-bases', { page_size: 100 }),
    staleTime: 30_000,
  })
  const list = kbs.data?.knowledge_bases ?? []
  // Ids of knowledge bases that were deleted or are no longer visible must not be sent.
  const valid = kbs.data ? kbIds.filter((id) => list.some((k) => k.id === id)) : kbIds

  useEffect(() => {
    if (kbs.data && valid.length !== kbIds.length) setKbIds(valid)
  }, [kbs.data, valid, kbIds.length, setKbIds])

  useEffect(() => {
    if (!open) return
    const onDown = (e: MouseEvent) => !ref.current?.contains(e.target as Node) && setOpen(false)
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && setOpen(false)
    document.addEventListener('mousedown', onDown)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onDown)
      document.removeEventListener('keydown', onKey)
    }
  }, [open])

  const label = valid.length === 0 ? '全部知识库' : valid.length === 1 ? (list.find((k) => k.id === valid[0])?.name ?? '1 个知识库') : `${valid.length} 个知识库`

  return (
    <div ref={ref} className="relative">
      <button
        onClick={() => setOpen((v) => !v)}
        aria-haspopup="listbox"
        aria-expanded={open}
        className="inline-flex h-8 max-w-48 items-center gap-1.5 rounded-lg border border-border px-2.5 text-[13px] hover:bg-surface-2"
      >
        <Database className="size-3.5 shrink-0 text-muted" />
        <span className="truncate">{label}</span>
        <ChevronDown className="size-3.5 shrink-0 text-muted" />
      </button>
      {open ? (
        <div className="absolute bottom-full left-0 z-20 mb-2 w-64 rounded-xl border border-border bg-surface p-1.5 shadow-lg">
          <p className="px-2.5 py-1.5 text-xs text-muted">不选表示在你能访问的全部知识库中检索</p>
          <div className="max-h-60 overflow-y-auto" role="listbox" aria-multiselectable="true">
            {kbs.isPending ? <p className="px-2.5 py-3 text-[13px] text-muted">加载中…</p> : null}
            {kbs.isError ? <p className="px-2.5 py-3 text-[13px] text-danger">知识库列表加载失败</p> : null}
            {kbs.data && list.length === 0 ? <p className="px-2.5 py-3 text-[13px] text-muted">还没有知识库</p> : null}
            {list.map((k) => {
              const checked = valid.includes(k.id)
              return (
                <label key={k.id} className="flex cursor-pointer items-center gap-2 rounded-lg px-2.5 py-1.5 text-[13px] hover:bg-surface-2">
                  <input
                    type="checkbox"
                    checked={checked}
                    onChange={() => setKbIds(checked ? valid.filter((id) => id !== k.id) : [...valid, k.id])}
                    className="size-4 accent-[var(--primary)]"
                  />
                  <span className="min-w-0 flex-1 truncate">{k.name}</span>
                  {k.mode === 'wiki' ? <span className="text-xs text-muted">Wiki</span> : null}
                </label>
              )
            })}
          </div>
          {valid.length > 0 ? (
            <button onClick={() => setKbIds([])} className="mt-1 w-full rounded-lg px-2.5 py-1.5 text-left text-[13px] text-primary hover:bg-surface-2">
              清除选择
            </button>
          ) : null}
        </div>
      ) : null}
    </div>
  )
}

interface ComposerProps {
  busy: boolean
  onSend: (text: string) => void
  onStop: () => void
}

export function Composer({ busy, onSend, onStop }: ComposerProps) {
  const mode = useChatPrefs((s) => s.mode)
  const setMode = useChatPrefs((s) => s.setMode)
  const [text, setText] = useState('')
  const ref = useRef<HTMLTextAreaElement>(null)
  const canSend = text.trim().length > 0 && !busy

  useEffect(() => {
    const el = ref.current
    if (!el) return
    el.style.height = 'auto'
    el.style.height = `${Math.min(el.scrollHeight, 200)}px`
  }, [text])

  function submit() {
    if (!canSend) return
    onSend(text.trim())
    setText('')
  }

  return (
    <div className="border-t border-border bg-bg px-4 pt-3 pb-4">
      <div className="mx-auto max-w-3xl rounded-2xl border border-border bg-surface shadow-sm focus-within:border-primary focus-within:ring-2 focus-within:ring-primary/20">
        <textarea
          ref={ref}
          value={text}
          rows={1}
          onChange={(e) => setText(e.target.value)}
          onKeyDown={(e) => {
            // Enter while an IME is composing confirms the candidate, not the message.
            if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) {
              e.preventDefault()
              submit()
            }
          }}
          placeholder="输入问题，Enter 发送，Shift+Enter 换行"
          aria-label="消息内容"
          className="block max-h-52 w-full resize-none rounded-t-2xl bg-transparent px-4 pt-3 pb-1 placeholder:text-muted/70 focus-visible:outline-none"
        />
        <div className="flex flex-wrap items-center gap-2 px-3 pb-2.5">
          <div className="inline-flex rounded-lg border border-border p-0.5" role="radiogroup" aria-label="回答模式">
            {MODES.map(({ id, label, hint, icon: Icon }) => (
              <button
                key={id}
                role="radio"
                aria-checked={mode === id}
                title={hint}
                disabled={busy}
                onClick={() => setMode(id)}
                className={cn(
                  'inline-flex h-7 items-center gap-1.5 rounded-md px-2.5 text-[13px] transition-colors disabled:opacity-60',
                  mode === id ? 'bg-primary-soft font-medium text-primary' : 'text-muted hover:text-fg',
                )}
              >
                <Icon className="size-3.5" />
                {label}
              </button>
            ))}
          </div>
          <KnowledgePicker />
          <div className="ml-auto">
            {busy ? (
              <button
                onClick={onStop}
                aria-label="停止生成"
                className="inline-flex size-8 items-center justify-center rounded-full bg-fg text-bg hover:opacity-85"
              >
                <Square className="size-3.5 fill-current" />
              </button>
            ) : (
              <button
                onClick={submit}
                disabled={!canSend}
                aria-label="发送"
                className="inline-flex size-8 items-center justify-center rounded-full bg-primary text-primary-fg transition-opacity hover:bg-primary-hover disabled:cursor-not-allowed disabled:opacity-40"
              >
                <ArrowUp className="size-4" />
              </button>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
