import { create } from 'zustand'
import type { Session, Message, ChatMode, Reference, AgentStep } from '../types/api'
import * as api from '../lib/api'

interface ChatState {
  sessions: Session[]
  currentSessionId: string
  messages: Message[]
  mode: ChatMode
  resolvedMode: string
  streaming: boolean
  streamContent: string
  agentSteps: AgentStep[]
  references: Reference[]
  pipelineStatus: string
  forceCitation: boolean
  enableSkills: boolean
  selectedSkills: string[]
  selectedKBIds: string[]

  // Actions
  setMode: (m: ChatMode) => void
  setForceCitation: (enabled: boolean) => void
  setEnableSkills: (enabled: boolean) => void
  toggleSkill: (name: string) => void
  toggleKB: (id: string) => void
  setSelectedKBIds: (ids: string[]) => void
  loadSessions: () => Promise<void>
  selectSession: (id: string) => Promise<void>
  createSession: (title: string) => Promise<string>
  deleteSession: (id: string) => Promise<void>
  sendMessage: (query: string, signal?: AbortSignal) => Promise<void>
  clearCurrent: () => void
}

export const useChatStore = create<ChatState>((set, get) => ({
  sessions: [],
  currentSessionId: '',
  messages: [],
  mode: 'auto',
  resolvedMode: '',
  streaming: false,
  streamContent: '',
  agentSteps: [],
  references: [],
  pipelineStatus: '',
  forceCitation: false,
  enableSkills: false,
  selectedSkills: ['citation-generator'],
  selectedKBIds: [],

  setMode: (m) => set({ mode: m, resolvedMode: '' }),
  setForceCitation: (enabled) => set({ forceCitation: enabled }),
  setEnableSkills: (enabled) => set({ enableSkills: enabled }),
  toggleSkill: (name) =>
    set((state) => {
      const exists = state.selectedSkills.includes(name)
      return {
        selectedSkills: exists
          ? state.selectedSkills.filter((item) => item !== name)
          : [...state.selectedSkills, name],
      }
    }),
  toggleKB: (id) =>
    set((state) => {
      const exists = state.selectedKBIds.includes(id)
      return {
        selectedKBIds: exists
          ? state.selectedKBIds.filter((item) => item !== id)
          : [...state.selectedKBIds, id],
      }
    }),
  setSelectedKBIds: (ids) => set({ selectedKBIds: ids }),

  loadSessions: async () => {
    try {
      const res = await api.getSessions()
      set({ sessions: res.sessions || [] })
    } catch {
      /* ignore */
    }
  },

  selectSession: async (id) => {
    set({ currentSessionId: id, messages: [], agentSteps: [], references: [], pipelineStatus: '' })
    if (!id) return
    try {
      const res = await api.getSessionMessages(id)
      set({ messages: res.messages || [] })
    } catch {
      /* ignore */
    }
  },

  createSession: async (title) => {
    const res = await api.createSession(title)
    await get().loadSessions()
    return res.id || res.session?.id || ''
  },

  deleteSession: async (id) => {
    await api.deleteSession(id)
    if (get().currentSessionId === id) {
      set({ currentSessionId: '', messages: [] })
    }
    await get().loadSessions()
  },

  sendMessage: async (query, signal) => {
    const { mode, currentSessionId, messages, forceCitation, enableSkills, selectedSkills, selectedKBIds } = get()

    // Add user message optimistically
    const userMsg: Message = {
      id: 'temp-' + Date.now(),
      session_id: currentSessionId,
      role: 'user',
      content: query,
      mode,
      created_at: new Date().toISOString(),
    }
    set({
      messages: [...messages, userMsg],
      streaming: true,
      streamContent: '',
      agentSteps: [],
      references: [],
      pipelineStatus: '',
    })

    let sessionId = currentSessionId
    let fullContent = ''
    const steps: AgentStep[] = []
    let refs: Reference[] = []

    try {
      await api.streamChat(query, sessionId, mode, {
        forceCitation,
        enableSkills,
        selectedSkills,
        knowledgeBaseIds: selectedKBIds.length > 0 ? selectedKBIds : undefined,
      }, (evt) => {
        if (evt.type === 'session_id' && evt.session_id) {
          sessionId = evt.session_id
          set({ currentSessionId: sessionId })
        } else if (evt.type === 'mode_resolved' && (evt as any).resolved_mode) {
          set({ resolvedMode: (evt as any).resolved_mode })
        } else if (evt.type === 'source') {
          const nextRef: Reference = {
            document_id: (evt as any).doc_id || '',
            document_name: (evt as any).doc_id || 'Document',
            knowledge_base_id: '',
            knowledge_base_name: '',
            chunk_index: refs.length,
            content: evt.content || '',
            score: Math.max(0, 1 - refs.length * 0.1),
          }
          refs = [...refs, nextRef]
          set({ references: refs })
        } else if (evt.type === 'content' && evt.content) {
          fullContent += evt.content
          set({ streamContent: fullContent })
        } else if (evt.type === 'thought' || evt.type === 'action' || evt.type === 'observation') {
          const step: AgentStep = {
            type: evt.type as AgentStep['type'],
            content: evt.content || '',
            tool_name: (evt as any).tool_name,
            tool_input: (evt as any).tool_input,
            timestamp: new Date().toISOString(),
          }
          steps.push(step)
          set({ agentSteps: [...steps] })
        } else if (evt.type === 'references') {
          refs = (evt as any).references || []
          set({ references: refs })
        } else if (evt.type === 'status' && evt.content) {
          set({ pipelineStatus: evt.content })
        }
      }, signal)
    } catch (err: any) {
      if (err.name === 'AbortError') {
        set({ streaming: false, streamContent: '', pipelineStatus: '' })
        return
      }
      fullContent = fullContent || `Error: ${err.message}`
    }

    // Finalize: add assistant message
    const assistantMsg: Message = {
      id: 'temp-' + Date.now(),
      session_id: sessionId,
      role: 'assistant',
      content: fullContent,
      mode,
      references: refs.length > 0 ? refs : undefined,
      agent_steps: steps.length > 0 ? steps : undefined,
      created_at: new Date().toISOString(),
    }

    set((s) => ({
      messages: [...s.messages, assistantMsg],
      streaming: false,
      streamContent: '',
      pipelineStatus: '',
    }))

    // Reload sessions to pick up new session
    get().loadSessions()
  },

  clearCurrent: () =>
    set({
      currentSessionId: '',
      messages: [],
      agentSteps: [],
      references: [],
      streamContent: '',
      pipelineStatus: '',
    }),
}))
