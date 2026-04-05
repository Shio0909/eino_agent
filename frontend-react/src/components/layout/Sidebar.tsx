import { NavLink, useNavigate } from 'react-router-dom'
import { MessageSquare, BookOpen, Wrench, Settings, Activity, Plus, PanelLeftClose, PanelLeft, FolderGit } from 'lucide-react'
import { useState } from 'react'
import ThemeToggle from './ThemeToggle'
import { useChatStore } from '../../stores/chat-store'
import SessionList from '../chat/SessionList'

const navItems = [
  { to: '/', icon: MessageSquare, label: '对话' },
  { to: '/knowledge', icon: BookOpen, label: '知识库' },
  { to: '/tools', icon: Wrench, label: '工具' },
  { to: '/code-repos', icon: FolderGit, label: '代码仓库' },
  { to: '/system', icon: Activity, label: '系统' },
]

export default function Sidebar() {
  const [collapsed, setCollapsed] = useState(false)
  const { clearCurrent } = useChatStore()
  const navigate = useNavigate()

  const handleNewChat = () => {
    clearCurrent()
    navigate('/')
  }

  return (
    <aside
      className={`flex flex-col border-r border-[var(--color-border)] bg-[var(--color-bg-secondary)] transition-all duration-200 ${
        collapsed ? 'w-[60px]' : 'w-[260px]'
      }`}
    >
      {/* Header */}
      <div className="flex items-center justify-between h-14 px-4">
        {!collapsed && (
          <span className="text-base font-semibold text-[var(--color-text-primary)]">Eino Agent</span>
        )}
        <button
          onClick={() => setCollapsed(!collapsed)}
          className="p-2 rounded-lg hover:bg-[var(--color-bg-hover)] text-[var(--color-text-muted)] transition-colors"
        >
          {collapsed ? <PanelLeft size={20} /> : <PanelLeftClose size={20} />}
        </button>
      </div>

      {/* New Chat */}
      <div className="px-3 mb-3">
        <button
          onClick={handleNewChat}
          className={`flex items-center gap-2.5 font-medium text-[var(--color-text-primary)] rounded-lg border border-[var(--color-border)] hover:bg-[var(--color-bg-hover)] transition-colors ${
            collapsed ? 'w-10 h-10 justify-center mx-auto text-sm' : 'w-full px-4 py-2.5 text-sm'
          }`}
        >
          <Plus size={18} />
          {!collapsed && <span>新对话</span>}
        </button>
      </div>

      {/* Nav */}
      <nav className="px-3 space-y-1">
        {navItems.map(({ to, icon: Icon, label }) => (
          <NavLink
            key={to}
            to={to}
            end={to === '/'}
            className={({ isActive }) =>
              `flex items-center gap-3 rounded-lg transition-colors ${
                collapsed ? 'w-10 h-10 justify-center mx-auto' : 'px-4 py-2.5'
              } ${
                isActive
                  ? 'bg-[var(--color-accent-light)] text-[var(--color-accent)] font-medium'
                  : 'text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-hover)] hover:text-[var(--color-text-primary)]'
              }`
            }
            title={collapsed ? label : undefined}
          >
            <Icon size={20} />
            {!collapsed && <span className="text-sm">{label}</span>}
          </NavLink>
        ))}
      </nav>

      {/* Divider */}
      {!collapsed && <div className="mx-4 my-4 border-t border-[var(--color-border)]" />}

      {/* Recent Chats */}
      {!collapsed && (
        <div className="flex-1 overflow-hidden flex flex-col px-3">
          <div className="text-xs font-medium text-[var(--color-text-muted)] uppercase tracking-wider px-4 mb-2">最近对话</div>
          <div className="flex-1 overflow-y-auto">
            <SessionList />
          </div>
        </div>
      )}

      {/* Footer */}
      <div className="mt-auto border-t border-[var(--color-border)] p-3 space-y-1">
        <NavLink
          to="/settings"
          className={({ isActive }) =>
            `flex items-center gap-3 rounded-lg transition-colors ${
              collapsed ? 'w-10 h-10 justify-center mx-auto' : 'px-4 py-2.5'
            } ${
              isActive
                ? 'bg-[var(--color-accent-light)] text-[var(--color-accent)] font-medium'
                : 'text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-hover)] hover:text-[var(--color-text-primary)]'
            }`
          }
          title={collapsed ? '设置' : undefined}
        >
          <Settings size={20} />
          {!collapsed && <span className="text-sm">设置</span>}
        </NavLink>
        <ThemeToggle collapsed={collapsed} />
      </div>
    </aside>
  )
}
