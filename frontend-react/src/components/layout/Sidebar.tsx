import { BookOpen, Bot, Database, Settings2 } from 'lucide-react';
import { clsx } from 'clsx';
import type { AppView } from '../../store/workspace';

const navItems: Array<{ id: AppView; label: string; description: string; icon: typeof Bot }> = [
  { id: 'chat', label: 'Chat', description: '问答与证据流', icon: Bot },
  { id: 'knowledge', label: 'Knowledge', description: '知识库与导入', icon: Database },
  { id: 'wiki', label: 'Wiki', description: '页面浏览与搜索', icon: BookOpen },
  { id: 'ops', label: 'Ops', description: '设置与能力状态', icon: Settings2 },
];

export function Sidebar({ active, onChange }: { active: AppView; onChange: (view: AppView) => void }) {
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
      <div className="mt-auto rounded-3xl border border-border/70 bg-surface/55 p-4">
        <p className="font-display text-lg font-semibold">Evidence Rail</p>
        <p className="mt-2 text-xs leading-5 text-muted">引用、工具步骤、Wiki 路径与导入状态统一沉淀在右侧证据栏。</p>
      </div>
    </aside>
  );
}
