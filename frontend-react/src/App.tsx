import { useCallback, useEffect, useState } from 'react';
import { endpoints } from './hooks/endpoints';
import { useAuthStore } from './store/auth';
import { useWorkspaceStore } from './store/workspace';
import { Sidebar } from './components/layout/Sidebar';
import { TopBar } from './components/layout/TopBar';
import { EvidenceRail } from './components/layout/EvidenceRail';
import { ChatPage } from './pages/ChatPage';
import { KnowledgePage } from './pages/KnowledgePage';
import { WikiPageView } from './pages/WikiPageView';
import { GraphPage } from './pages/GraphPage';
import { SettingsPage } from './pages/SettingsPage';
import { LoginPage } from './pages/LoginPage';
import type { ReferenceDocument, TraceStep } from './types/api';

export function App() {
  const view = useWorkspaceStore((state) => state.view);
  const setView = useWorkspaceStore((state) => state.setView);
  const { user, token, setAuth, logout } = useAuthStore();
  const [authChecked, setAuthChecked] = useState(false);
  const [authRequired, setAuthRequired] = useState(false);
  const [references, setReferences] = useState<ReferenceDocument[]>([]);
  const [trace, setTrace] = useState<TraceStep[]>([]);
  const [streaming, setStreaming] = useState(false);
  const [chatMode, setChatMode] = useState('agentic');
  const [forceCitation, setForceCitation] = useState(true);
  const [evidenceOptions, setEvidenceOptions] = useState({ showRetrieval: true, showRerank: true, showTrace: true, showContext: true });

  useEffect(() => {
    endpoints.me()
      .then((response) => setAuth(token, response.user))
      .catch((error) => {
        if (error instanceof Error && error.message.includes('Bearer')) setAuthRequired(true);
      })
      .finally(() => setAuthChecked(true));
  }, []);

  const updateEvidence = useCallback((nextReferences: ReferenceDocument[], nextTrace: TraceStep[], nextStreaming: boolean) => {
    setReferences(nextReferences);
    setTrace(nextTrace);
    setStreaming(nextStreaming);
  }, []);

  if (!authChecked) {
    return <main className="grid min-h-screen place-items-center font-display text-2xl font-semibold">连接后端能力面…</main>;
  }

  if (authRequired && !user) {
    return <LoginPage />;
  }

  return (
    <main className="grid h-screen gap-4 p-4 lg:grid-cols-[18rem_minmax(0,1fr)_22rem]">
      <Sidebar active={view} onChange={setView} />
      <section className="flex min-h-0 flex-col gap-4">
        <TopBar user={user} onLogout={logout} />
        <div className="min-h-0 flex-1">
          {view === 'chat' ? <ChatPage onEvidence={updateEvidence} mode={chatMode} forceCitation={forceCitation} /> : null}
          {view === 'knowledge' ? <KnowledgePage /> : null}
          {view === 'wiki' ? <WikiPageView /> : null}
          {view === 'graph' ? <GraphPage /> : null}
          {view === 'settings' ? <SettingsPage mode={chatMode} forceCitation={forceCitation} evidenceOptions={evidenceOptions} onModeChange={setChatMode} onForceCitationChange={setForceCitation} onEvidenceOptionsChange={setEvidenceOptions} /> : null}
        </div>
      </section>
      <EvidenceRail references={references} trace={trace} streaming={streaming} options={evidenceOptions} />
    </main>
  );
}
