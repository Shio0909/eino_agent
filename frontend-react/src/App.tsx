import { BrowserRouter, Routes, Route } from 'react-router-dom'
import AppLayout from './components/layout/AppLayout'
import ChatPage from './pages/ChatPage'
import KnowledgePage from './pages/KnowledgePage'
import ToolsPage from './pages/ToolsPage'
import SettingsPage from './pages/SettingsPage'
import SystemPage from './pages/SystemPage'
import { ToastContainer } from './components/ui'
import { useThemeStore } from './stores/theme-store'
import { useEffect } from 'react'

export default function App() {
  const theme = useThemeStore((s) => s.theme)

  useEffect(() => {
    document.documentElement.classList.toggle('dark', theme === 'dark')
  }, [theme])

  return (
    <BrowserRouter>
      <AppLayout>
        <Routes>
          <Route path="/" element={<ChatPage />} />
          <Route path="/knowledge" element={<KnowledgePage />} />
          <Route path="/tools" element={<ToolsPage />} />
          <Route path="/settings" element={<SettingsPage />} />
          <Route path="/system" element={<SystemPage />} />
        </Routes>
      </AppLayout>
      <ToastContainer />
    </BrowserRouter>
  )
}
