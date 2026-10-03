import { BarChart3, Database, GitBranch, MessageSquare, Server, Settings, ShieldCheck, type LucideIcon } from 'lucide-react'

export interface NavItem {
  to: string
  label: string
  icon: LucideIcon
  /** Hidden from users whose global role is not admin. */
  adminOnly?: boolean
}

export const NAV_ITEMS: NavItem[] = [
  { to: '/chat', label: '对话', icon: MessageSquare },
  { to: '/knowledge', label: '知识库', icon: Database },
  { to: '/code', label: '代码仓库', icon: GitBranch },
  { to: '/eval', label: '评测报告', icon: BarChart3 },
  { to: '/approvals', label: '审批', icon: ShieldCheck },
  { to: '/settings', label: '设置', icon: Settings, adminOnly: true },
  { to: '/system', label: '系统', icon: Server },
]
