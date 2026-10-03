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
const ChunksPage = lazy(() => import('@/pages/knowledge/ChunksPage'))
const ComingSoon = lazy(() => import('@/pages/ComingSoon'))

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
              </Route>
              <Route path="/knowledge/:kbId/documents/:docId" element={<ChunksPage />} />
              <Route path="/code" element={<ComingSoon title="代码仓库" />} />
              <Route path="/eval" element={<ComingSoon title="评测报告" />} />
              <Route path="/approvals" element={<ComingSoon title="审批" />} />
              <Route path="/settings" element={<ComingSoon title="设置" />} />
              <Route path="/system" element={<ComingSoon title="系统" />} />
              <Route path="*" element={<Navigate to="/chat" replace />} />
            </Route>
          </Route>
        </Routes>
      </Suspense>
      <Toaster />
    </>
  )
}
