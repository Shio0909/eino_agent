import { lazy, Suspense } from 'react'
import { Navigate, Route, Routes } from 'react-router-dom'
import { AppLayout } from '@/components/layout/AppLayout'
import { RequireAuth } from '@/components/RequireAuth'
import { PageSpinner } from '@/components/ui/Spinner'
import { Toaster } from '@/components/ui/Toaster'

const LoginPage = lazy(() => import('@/pages/LoginPage'))
const ChatPage = lazy(() => import('@/pages/chat/ChatPage'))
const KnowledgeListPage = lazy(() => import('@/pages/knowledge/KnowledgeListPage'))
const KbLayout = lazy(() => import('@/pages/knowledge/KbLayout'))
const DocumentsTab = lazy(() => import('@/pages/knowledge/DocumentsTab'))
const WikiTab = lazy(() => import('@/pages/knowledge/WikiTab'))
const GraphTab = lazy(() => import('@/pages/knowledge/GraphTab'))
const ChunksPage = lazy(() => import('@/pages/knowledge/ChunksPage'))
const CodeReposPage = lazy(() => import('@/pages/code/CodeReposPage'))
const EvalPage = lazy(() => import('@/pages/eval/EvalPage'))
const ApprovalsPage = lazy(() => import('@/pages/approvals/ApprovalsPage'))
const SettingsPage = lazy(() => import('@/pages/settings/SettingsPage'))
const SystemPage = lazy(() => import('@/pages/system/SystemPage'))

export default function App() {
  return (
    <>
      <Suspense fallback={<PageSpinner />}>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route element={<RequireAuth />}>
            <Route element={<AppLayout />}>
              <Route index element={<Navigate to="/chat" replace />} />
              <Route path="/chat/:sessionId?" element={<ChatPage />} />
              <Route path="/knowledge" element={<KnowledgeListPage />} />
              <Route path="/knowledge/:kbId" element={<KbLayout />}>
                <Route index element={<DocumentsTab />} />
                <Route path="wiki" element={<WikiTab />} />
                <Route path="graph" element={<GraphTab />} />
              </Route>
              <Route path="/knowledge/:kbId/documents/:docId" element={<ChunksPage />} />
              <Route path="/code" element={<CodeReposPage />} />
              <Route path="/eval" element={<EvalPage />} />
              <Route path="/approvals" element={<ApprovalsPage />} />
              <Route path="/settings" element={<SettingsPage />} />
              <Route path="/system" element={<SystemPage />} />
              <Route path="*" element={<Navigate to="/chat" replace />} />
            </Route>
          </Route>
        </Routes>
      </Suspense>
      <Toaster />
    </>
  )
}
