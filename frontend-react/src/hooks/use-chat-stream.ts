import { useCallback, useRef } from 'react'
import { streamChat } from '../lib/api'
import type { AgentStep, Reference } from '../types/api'

interface StreamCallbacks {
  onContent: (content: string) => void
  onSessionId: (id: string) => void
  onAgentStep: (step: AgentStep) => void
  onReferences: (refs: Reference[]) => void
  onStatus: (status: string) => void
  onDone: () => void
  onError: (err: Error) => void
}

export function useChatStream() {
  const abortRef = useRef<AbortController | null>(null)

  const send = useCallback(
    (query: string, sessionId: string, mode: string, callbacks: StreamCallbacks) => {
      // Abort any existing stream
      abortRef.current?.abort()
      const controller = new AbortController()
      abortRef.current = controller

      streamChat(
        query,
        sessionId,
        mode,
        undefined,
        (evt: { type: string; content?: string; session_id?: string; error?: string }) => {
          if (evt.type === 'session_id' && evt.session_id) {
            callbacks.onSessionId(evt.session_id)
          } else if (evt.type === 'content' && evt.content) {
            callbacks.onContent(evt.content)
          } else if (evt.type === 'thought' || evt.type === 'action' || evt.type === 'observation') {
            callbacks.onAgentStep({
              type: evt.type as AgentStep['type'],
              content: evt.content || '',
              tool_name: (evt as any).tool_name,
              tool_input: (evt as any).tool_input,
              timestamp: new Date().toISOString(),
            })
          } else if (evt.type === 'references') {
            callbacks.onReferences((evt as any).references || [])
          } else if (evt.type === 'status') {
            callbacks.onStatus(evt.content || '')
          } else if (evt.type === 'done') {
            callbacks.onDone()
          } else if (evt.type === 'error') {
            callbacks.onError(new Error(evt.error || 'Unknown error'))
          }
        },
        controller.signal,
      ).then(() => {
        callbacks.onDone()
      }).catch((err) => {
        if (err.name !== 'AbortError') {
          callbacks.onError(err)
        }
      })
    },
    [],
  )

  const abort = useCallback(() => {
    abortRef.current?.abort()
    abortRef.current = null
  }, [])

  return { send, abort }
}
