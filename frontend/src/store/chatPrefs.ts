import { create } from 'zustand'
import type { ChatMode } from '@/types/chat'

const KEY = 'eino.chatPrefs'

interface Persisted {
  mode: ChatMode
  kbIds: string[]
}

function load(): Persisted {
  try {
    const raw = JSON.parse(localStorage.getItem(KEY) ?? '{}') as Partial<Persisted>
    return {
      mode: raw.mode === 'agentic' ? 'agentic' : 'pipeline',
      kbIds: Array.isArray(raw.kbIds) ? raw.kbIds.filter((id): id is string => typeof id === 'string') : [],
    }
  } catch {
    return { mode: 'pipeline', kbIds: [] }
  }
}

interface ChatPrefsState extends Persisted {
  setMode: (mode: ChatMode) => void
  setKbIds: (ids: string[]) => void
}

/** Answer mode and knowledge-base scope chosen in the composer; kept across reloads. */
export const useChatPrefs = create<ChatPrefsState>((set, get) => {
  const save = () => {
    const { mode, kbIds } = get()
    localStorage.setItem(KEY, JSON.stringify({ mode, kbIds }))
  }
  return {
    ...load(),
    setMode: (mode) => {
      set({ mode })
      save()
    },
    setKbIds: (kbIds) => {
      set({ kbIds })
      save()
    },
  }
})
