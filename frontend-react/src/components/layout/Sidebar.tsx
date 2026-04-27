import { BookOpen, Bot, Database, GitBranch, Settings2 } from 'lucide-react';
import { clsx } from 'clsx';
import type { AppView } from '../../store/workspace';

const navItems: Array<{ id: AppView; label: string; description: string; icon: typeof Bot }> = [
  { id: 'chat', label: 'Chat', description: '问答与证据流', icon: Bot },
  { id: 'knowledge', label: 'Knowledge', description: '知识库与导入', icon: Database },
  { id: 'wiki', label: 'Wiki', description: '页面浏览与搜索', icon: BookOpen },
  { id: 'graph', label: 'Graph', description: '实体关系与来源图谱', icon: GitBranch },
];

interface SidebarProps {
  active: AppView;
  onChange: (view: AppView) => void;
}

export function Sidebar({ active, onChange }: SidebarProps) {
  return (
    <aside className="glass-panel flex h-full flex-col rounded-[2rem] p-4">
      <div className="px-2 py-3">
        <p className="font-mono text-xs uppercase tracking-[0.28em] text-accent">Eino</p>
        <h1 className="mt-2 font-display text-3xl font-bold leading-none tracking-tight">RAG Agent</h1>
        <p className="mt-3 text-sm leading-6 text-muted">企业知识中台控制台：导入、浏览、问答与审计。</p>
      </div>
      <nav className="mt-6 space-y-2">
        {navItems.map((item) => {
          const Icon = item.icon;
          return (
            <button
              key={item.id}
              onClick={() => onChange(item.id)}
              className={clsx('focus-ring flex w-full items-center gap-3 rounded-2xl px-3 py-3 text-left transition', active === item.id ? 'bg-text text-surface shadow-soft' : 'text-text hover:bg-text/5')}
            >
              <Icon className="h-5 w-5 shrink-0" />
              <span>
                <span className="block text-sm font-semibold">{item.label}</span>
                <span className={clsx('block text-xs', active === item.id ? 'text-surface/70' : 'text-muted')}>{item.description}</span>
              </span>
            </button>
          );
        })}
      </nav>
      <button type="button" onClick={() => onChange('settings')} className="focus-ring mt-auto rounded-3xl border border-border/70 bg-surface/55 p-4 text-left transition hover:bg-panel/80">
        <div className="flex items-center gap-2">
          <Settings2 className="h-4 w-4 text-accent" />
          <p className="font-display text-lg font-semibold">设置</p>
        </div>
        <p className="mt-2 text-xs leading-5 text-muted">打开设置页调整对话、证据栏和运行状态。</p>
      </button>
    </aside>
  );
}
