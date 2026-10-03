import { useCallback, useEffect, useRef, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { apiGet, errorMessage } from '@/lib/api'
import { applyFrame, failTurn, finishStream, frameSessionId, newTurn, settleApproval, turnFromMessage, type Turn } from '@/lib/chatTurn'
import { streamPost } from '@/lib/sse'
import { useChatPrefs } from '@/store/chatPrefs'
import type { ChatMessage, ChatRequest, MessagesResponse } from '@/types/chat'

export type ChatItem =
  | { id: string; role: 'user'; content: string }
  | { id: string; role: 'assistant'; turn: Turn; latencyMs?: number }

let seq = 0
const uid = () => `local-${++seq}`

function itemFromMessage(m: ChatMessage): ChatItem {
  if (m.role === 'assistant') return { id: m.id, role: 'assistant', turn: turnFromMessage(m), latencyMs: m.latency_ms }
  return { id: m.id, role: 'user', content: m.content }
}

/**
 * State of one conversation: loads the stored messages of a session, streams
 * new answers, and lets the caller stop or switch away from a running one.
 */
export function useChat(routeSessionId: string | undefined, openSession: (id: string) => void) {
  const qc = useQueryClient()
  const [items, setItems] = useState<ChatItem[]>([])
  const [loading, setLoading] = useState(false)
  const [loadError, setLoadError] = useState<string>()
  const abortRef = useRef<AbortController | null>(null)
  // The session a running stream created: the route moves to it, but its
  // messages are already on screen, so they must not be fetched again.
  const ownSession = useRef<string>(undefined)

  const busy = items.some((i) => i.role === 'assistant' && i.turn.status === 'streaming')

  useEffect(() => {
    if (routeSessionId && routeSessionId === ownSession.current) return
    abortRef.current?.abort()
    ownSession.current = undefined
    setLoadError(undefined)
    if (!routeSessionId) {
      // oxlint-disable-next-line react/set-state-in-effect
      setItems([])
      setLoading(false)
      return
    }
    const ctrl = new AbortController()
    // Fetching on a route change is the external sync this effect exists for, so
    // the reset that goes with it belongs here too.
    // oxlint-disable-next-line react/set-state-in-effect
    setItems([])
    setLoading(true)
    apiGet<MessagesResponse>(`/sessions/${routeSessionId}/messages`, undefined, ctrl.signal)
      .then((res) => {
        setItems((res.messages ?? []).filter((m) => m.role !== 'system').map(itemFromMessage))
        setLoading(false)
      })
      .catch((err) => {
        if (ctrl.signal.aborted) return
        setLoadError(errorMessage(err))
        setLoading(false)
      })
    return () => ctrl.abort()
  }, [routeSessionId])

  useEffect(() => () => abortRef.current?.abort(), [])

  const send = useCallback(
    async (query: string) => {
      const { mode, kbIds } = useChatPrefs.getState()
      const assistantId = uid()
      setItems((prev) => [...prev, { id: uid(), role: 'user', content: query }, { id: assistantId, role: 'assistant', turn: newTurn() }])
      const update = (fn: (t: Turn) => Turn) =>
        setItems((prev) => prev.map((i) => (i.id === assistantId && i.role === 'assistant' ? { ...i, turn: fn(i.turn) } : i)))

      const ctrl = new AbortController()
      abortRef.current = ctrl
      const body: ChatRequest = {
        query,
        mode,
        session_id: routeSessionId ?? ownSession.current,
        knowledge_base_ids: kbIds.length ? kbIds : undefined,
      }
      const started = Date.now()
      try {
        for await (const frame of streamPost('/chat/stream', body, ctrl.signal)) {
          const sid = frameSessionId(frame)
          if (sid && !routeSessionId && sid !== ownSession.current) {
            ownSession.current = sid
            openSession(sid)
            void qc.invalidateQueries({ queryKey: ['sessions'] })
          }
          update((t) => applyFrame(t, frame))
        }
        update((t) => finishStream(t, 'closed'))
      } catch (err) {
        if (ctrl.signal.aborted) update((t) => finishStream(t, 'aborted'))
        else update((t) => failTurn(t, errorMessage(err)))
      } finally {
        const elapsed = Date.now() - started
        setItems((prev) => prev.map((i) => (i.id === assistantId && i.role === 'assistant' ? { ...i, latencyMs: elapsed } : i)))
        if (abortRef.current === ctrl) abortRef.current = null
        void qc.invalidateQueries({ queryKey: ['sessions'] })
      }
    },
    [routeSessionId, openSession, qc],
  )

  const stop = useCallback(() => abortRef.current?.abort(), [])

  const settle = useCallback((itemId: string, approvalId: string, status: 'approved' | 'rejected', reason?: string) => {
    setItems((prev) =>
      prev.map((i) => (i.id === itemId && i.role === 'assistant' ? { ...i, turn: settleApproval(i.turn, approvalId, status, reason) } : i)),
    )
  }, [])

  return { items, loading, loadError, busy, send, stop, settle }
}
