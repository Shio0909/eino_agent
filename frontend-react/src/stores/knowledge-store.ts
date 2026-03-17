import { create } from 'zustand'
import type { KnowledgeBase, Document, Chunk } from '../types/api'
import * as api from '../lib/api'

interface KnowledgeState {
  knowledgeBases: KnowledgeBase[]
  currentKBId: string
  documents: Document[]
  chunks: Chunk[]
  loading: boolean

  loadKBs: () => Promise<void>
  selectKB: (id: string) => Promise<void>
  createKB: (name: string, description?: string) => Promise<void>
  deleteKB: (id: string) => Promise<void>
  loadDocuments: (kbId: string) => Promise<void>
  uploadDoc: (kbId: string, file: File) => Promise<void>
  deleteDoc: (kbId: string, docId: string) => Promise<void>
  loadChunks: (kbId: string, docId: string) => Promise<void>
  clearChunks: () => void
}

export const useKnowledgeStore = create<KnowledgeState>((set, get) => ({
  knowledgeBases: [],
  currentKBId: '',
  documents: [],
  chunks: [],
  loading: false,

  loadKBs: async () => {
    set({ loading: true })
    try {
      const res = await api.getKnowledgeBases()
      set({ knowledgeBases: res.knowledge_bases || [] })
    } finally {
      set({ loading: false })
    }
  },

  selectKB: async (id) => {
    set({ currentKBId: id, documents: [], chunks: [] })
    if (id) await get().loadDocuments(id)
  },

  createKB: async (name, description) => {
    await api.createKnowledgeBase({ name, description })
    await get().loadKBs()
  },

  deleteKB: async (id) => {
    await api.deleteKnowledgeBase(id)
    if (get().currentKBId === id) set({ currentKBId: '', documents: [], chunks: [] })
    await get().loadKBs()
  },

  loadDocuments: async (kbId) => {
    try {
      const res = await api.getDocuments(kbId)
      set({ documents: res.documents || [] })
    } catch {
      set({ documents: [] })
    }
  },

  uploadDoc: async (kbId, file) => {
    await api.uploadDocument(kbId, file)
    await get().loadDocuments(kbId)
    await get().loadKBs() // refresh counts
  },

  deleteDoc: async (kbId, docId) => {
    await api.deleteDocument(kbId, docId)
    await get().loadDocuments(kbId)
    await get().loadKBs()
  },

  loadChunks: async (kbId, docId) => {
    try {
      const res = await api.getDocumentChunks(kbId, docId)
      set({ chunks: res.chunks || [] })
    } catch {
      set({ chunks: [] })
    }
  },

  clearChunks: () => set({ chunks: [] }),
}))
