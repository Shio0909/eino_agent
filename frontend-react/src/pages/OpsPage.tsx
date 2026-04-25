import { Activity, Database, GitBranch, Settings2, Wrench } from 'lucide-react';
import { useOpsStatus } from '../hooks/queries';
import { Badge } from '../components/ui/Badge';
import { Card } from '../components/ui/Card';

function JsonPanel({ data }: { data: unknown }) {
  return <pre className="max-h-72 overflow-auto rounded-2xl bg-text/5 p-4 font-mono text-xs leading-5 text-muted">{JSON.stringify(data ?? {}, null, 2)}</pre>;
}

export function OpsPage() {
  const { mcp, settings, system, graph } = useOpsStatus();
  const cards = [
    { title: 'System', icon: Activity, data: system.data, loading: system.isLoading },
    { title: 'MCP', icon: Wrench, data: mcp.data, loading: mcp.isLoading },
    { title: 'GraphRAG', icon: GitBranch, data: graph.data, loading: graph.isLoading },
    { title: 'Settings', icon: Settings2, data: settings.data, loading: settings.isLoading },
  ];

  return (
    <div className="grid gap-4 xl:grid-cols-2">
      <Card className="p-6 xl:col-span-2">
        <p className="font-mono text-xs uppercase tracking-[0.22em] text-accent">Operations</p>
        <h3 className="mt-2 font-display text-3xl font-semibold">运行状态与配置概览</h3>
        <p className="mt-2 text-sm text-muted">默认只读展示，写配置和 MCP 导入应保留给 admin 操作，避免误改生产运行态。</p>
      </Card>
      {cards.map((item) => {
        const Icon = item.icon;
        return (
          <Card key={item.title} className="p-5">
            <div className="mb-4 flex items-center justify-between">
              <h4 className="flex items-center gap-2 font-display text-2xl font-semibold"><Icon className="h-5 w-5 text-primary" />{item.title}</h4>
              <Badge tone={item.loading ? 'warning' : 'success'}>{item.loading ? 'loading' : 'loaded'}</Badge>
            </div>
            <JsonPanel data={item.data} />
          </Card>
        );
      })}
    </div>
  );
}
