import { useCallback, useRef } from 'react'
import { streamChat } from '../lib/api'
import type { AgentStep, Reference, StreamEvent } from '../types/api'

interface StreamCallbacks {
  onContent: (content: string) => void
  onSessionId: (id: string) => void
  onAgentStep: (step: AgentStep) => void
  onReferences: (refs: Reference[]) => void
  onResolvedMode?: (mode: string) => void
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
      const refs: Reference[] = []

      streamChat(
        query,
        sessionId,
        mode,
        undefined,
        (evt: StreamEvent & { error?: string }) => {
          if (evt.type === 'session_id' && evt.session_id) {
            callbacks.onSessionId(evt.session_id)
          } else if (evt.type === 'mode_resolved' && evt.resolved_mode) {
            callbacks.onResolvedMode?.(evt.resolved_mode)
          } else if (evt.type === 'source') {
            refs.push({
              document_id: evt.doc_id || '',
              document_name: evt.doc_id || 'Document',
              knowledge_base_id: '',
              knowledge_base_name: '',
              chunk_index: refs.length,
              content: evt.content || '',
              score: Math.max(0, 1 - refs.length * 0.1),
            })
            callbacks.onReferences([...refs])
          } else if (evt.type === 'content' && evt.content) {
            callbacks.onContent(evt.content)
          } else if (evt.type === 'thought' || evt.type === 'action' || evt.type === 'observation') {
            callbacks.onAgentStep({
              type: evt.type as AgentStep['type'],
              content: evt.content || '',
              tool_name: evt.tool_name,
              tool_input: evt.tool_input,
              timestamp: new Date().toISOString(),
            })
          } else if (evt.type === 'references') {
            callbacks.onReferences(evt.references || [])
          } else if (evt.type === 'status') {
            callbacks.onStatus(evt.content || '')
          } else if (evt.type === 'done') {
            callbacks.onDone()
          } else if (evt.type === 'error') {
            callbacks.onError(new Error(evt.error || 'Unknown error'))
          }
        },
        controller.signal,
      ).catch((err) => {
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
