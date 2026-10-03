import { useEffect, useRef, useState } from 'react'
import { NavLink, Outlet, useLocation, useNavigate } from 'react-router-dom'
import { LogOut, Menu, Moon, Sparkles, Sun, User } from 'lucide-react'
import { NAV_ITEMS } from '@/components/layout/nav'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { useIsAdmin } from '@/hooks/useIsAdmin'
import { cn } from '@/lib/cn'
import { useAuth } from '@/store/auth'
import { useTheme } from '@/store/theme'

function Sidebar({ onNavigate }: { onNavigate?: () => void }) {
  const isAdmin = useIsAdmin()
  const items = NAV_ITEMS.filter((i) => !i.adminOnly || isAdmin)
  return (
    <div className="flex h-full flex-col">
      <div className="flex h-14 items-center gap-2.5 px-5">
        <div className="flex size-8 items-center justify-center rounded-lg bg-primary text-primary-fg">
          <Sparkles className="size-4" />
        </div>
        <span className="font-semibold">Eino RAG</span>
      </div>
      <nav className="flex-1 space-y-0.5 overflow-y-auto px-3 py-2" aria-label="主导航">
        {items.map(({ to, label, icon: Icon }) => (
          <NavLink
            key={to}
            to={to}
            onClick={onNavigate}
            className={({ isActive }) =>
              cn(
                'flex items-center gap-2.5 rounded-lg px-3 py-2 text-[13.5px] font-medium transition-colors',
                isActive ? 'bg-primary-soft text-primary' : 'text-muted hover:bg-surface-2 hover:text-fg',
              )
            }
          >
            <Icon className="size-4" />
            {label}
          </NavLink>
        ))}
      </nav>
    </div>
  )
}

function UserMenu() {
  const user = useAuth((s) => s.user)
  const logout = useAuth((s) => s.logout)
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const onDown = (e: MouseEvent) => !ref.current?.contains(e.target as Node) && setOpen(false)
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && setOpen(false)
    document.addEventListener('mousedown', onDown)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onDown)
      document.removeEventListener('keydown', onKey)
    }
  }, [open])

  if (!user) return null
  return (
    <div ref={ref} className="relative">
      <button
        onClick={() => setOpen((v) => !v)}
        aria-haspopup="menu"
        aria-expanded={open}
        className="flex h-9 items-center gap-2 rounded-lg px-2 hover:bg-surface-2"
      >
        <span className="flex size-7 items-center justify-center rounded-full bg-primary-soft text-primary">
          <User className="size-4" />
        </span>
        <span className="hidden max-w-32 truncate text-[13px] font-medium sm:block">{user.id}</span>
      </button>
      {open ? (
        <div role="menu" className="absolute right-0 z-30 mt-1.5 w-56 rounded-xl border border-border bg-surface p-1.5 shadow-lg">
          <div className="px-3 py-2">
            <p className="truncate font-medium">{user.id}</p>
            <div className="mt-1 flex items-center gap-1.5">
              <Badge variant={user.role === 'admin' ? 'primary' : 'neutral'}>{user.role === 'admin' ? '管理员' : '用户'}</Badge>
              <span className="text-xs text-muted">租户 {user.tenant_id}</span>
            </div>
          </div>
          <div className="my-1 h-px bg-border" />
          <button
            role="menuitem"
            onClick={() => logout()}
            className="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-left text-[13px] hover:bg-surface-2"
          >
            <LogOut className="size-4 text-muted" />
            退出登录
          </button>
        </div>
      ) : null}
    </div>
  )
}

export function AppLayout() {
  const { theme, toggle } = useTheme()
  const [drawer, setDrawer] = useState(false)
  const location = useLocation()
  const navigate = useNavigate()
  const title = NAV_ITEMS.find((i) => location.pathname.startsWith(i.to))?.label ?? ''

  // The chat page manages its own scrolling and wants the full height.
  const fullBleed = location.pathname.startsWith('/chat')

  return (
    <div className="flex h-full">
      <aside className="hidden w-56 shrink-0 border-r border-border bg-surface md:block">
        <Sidebar />
      </aside>

      {drawer ? (
        <div className="fixed inset-0 z-40 md:hidden">
          <div className="absolute inset-0 bg-black/45" onClick={() => setDrawer(false)} aria-hidden />
          <aside className="absolute inset-y-0 left-0 w-60 border-r border-border bg-surface shadow-xl">
            <Sidebar onNavigate={() => setDrawer(false)} />
          </aside>
        </div>
      ) : null}

      <div className="flex min-w-0 flex-1 flex-col">
        <header className="flex h-14 shrink-0 items-center justify-between gap-3 border-b border-border bg-surface px-4">
          <div className="flex items-center gap-2">
            <Button variant="ghost" size="icon" className="md:hidden" onClick={() => setDrawer(true)} aria-label="打开导航">
              <Menu className="size-4" />
            </Button>
            <button onClick={() => navigate('/chat')} className="font-semibold md:hidden">
              Eino RAG
            </button>
            <span className="hidden text-[13px] text-muted md:block">{title}</span>
          </div>
          <div className="flex items-center gap-1.5">
            <Button variant="ghost" size="icon" onClick={toggle} aria-label={theme === 'dark' ? '切换到浅色模式' : '切换到深色模式'}>
              {theme === 'dark' ? <Sun className="size-4" /> : <Moon className="size-4" />}
            </Button>
            <UserMenu />
          </div>
        </header>
        <main className={cn('min-h-0 flex-1', fullBleed ? 'overflow-hidden' : 'overflow-y-auto')}>
          {fullBleed ? <Outlet /> : (
            <div className="mx-auto w-full max-w-6xl px-4 py-6 sm:px-6">
              <Outlet />
            </div>
          )}
        </main>
      </div>
    </div>
  )
}
