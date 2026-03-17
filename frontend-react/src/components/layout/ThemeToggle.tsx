import { Moon, Sun } from 'lucide-react'
import { useThemeStore } from '../../stores/theme-store'

export default function ThemeToggle({ collapsed = false }: { collapsed?: boolean }) {
  const { theme, toggle } = useThemeStore()

  return (
    <button
      onClick={toggle}
      className={`flex items-center gap-3 rounded-lg text-sm text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-hover)] hover:text-[var(--color-text-primary)] transition-colors ${
        collapsed ? 'w-10 h-10 justify-center mx-auto' : 'w-full px-4 py-2.5'
      }`}
    >
      {theme === 'dark' ? <Sun size={20} /> : <Moon size={20} />}
      {!collapsed && <span>{theme === 'dark' ? '浅色模式' : '深色模式'}</span>}
    </button>
  )
}
