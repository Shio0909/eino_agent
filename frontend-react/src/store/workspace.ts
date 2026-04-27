import { create } from 'zustand';

export type AppView = 'chat' | 'knowledge' | 'wiki' | 'graph' | 'settings';

interface WorkspaceState {
  view: AppView;
  selectedKnowledgeBaseId: string | null;
  selectedSessionId: string | null;
  setView: (view: AppView) => void;
  setSelectedKnowledgeBaseId: (id: string | null) => void;
  setSelectedSessionId: (id: string | null) => void;
}

export const useWorkspaceStore = create<WorkspaceState>((set) => ({
  view: 'chat',
  selectedKnowledgeBaseId: null,
  selectedSessionId: null,
  setView: (view) => set({ view }),
  setSelectedKnowledgeBaseId: (selectedKnowledgeBaseId) => set({ selectedKnowledgeBaseId }),
  setSelectedSessionId: (selectedSessionId) => set({ selectedSessionId }),
}));
