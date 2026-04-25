import { LogOut, ShieldCheck } from 'lucide-react';
import { Button } from '../ui/Button';
import type { User } from '../../types/api';

export function TopBar({ user, onLogout }: { user: User | null; onLogout: () => void }) {
  return (
    <header className="flex flex-wrap items-center justify-between gap-4">
      <div>
        <p className="font-mono text-xs uppercase tracking-[0.24em] text-muted">Workspace</p>
        <h2 className="font-display text-4xl font-bold tracking-tight">知识工作台</h2>
      </div>
      <div className="flex items-center gap-3 rounded-2xl border border-border/80 bg-panel/70 px-3 py-2">
        <ShieldCheck className="h-4 w-4 text-success" />
        <span className="text-sm font-medium">{user ? `${user.id} · ${user.role} · tenant ${user.tenant_id}` : 'Auth disabled / anonymous'}</span>
        {user ? <Button variant="ghost" className="px-2 py-1" onClick={onLogout}><LogOut className="h-4 w-4" /></Button> : null}
      </div>
    </header>
  );
}
