import { Activity, GitBranch, Settings2, Wrench } from 'lucide-react';
import { useOpsStatus } from '../hooks/queries';
import { Badge } from '../components/ui/Badge';
import { Card } from '../components/ui/Card';

interface EvidenceOptions {
  showRetrieval: boolean;
  showRerank: boolean;
  showTrace: boolean;
  showContext: boolean;
}

interface SettingsPageProps {
  mode: string;
  forceCitation: boolean;
  evidenceOptions: EvidenceOptions;
  onModeChange: (mode: string) => void;
  onForceCitationChange: (enabled: boolean) => void;
  onEvidenceOptionsChange: (options: EvidenceOptions) => void;
}

export function SettingsPage({ mode, forceCitation, evidenceOptions, onModeChange, onForceCitationChange, onEvidenceOptionsChange }: SettingsPageProps) {
  const { mcp, settings, system, graph } = useOpsStatus();

  const updateEvidenceOption = (key: keyof EvidenceOptions, value: boolean) => {
    onEvidenceOptionsChange({ ...evidenceOptions, [key]: value });
  };

  return (
    <div className="grid h-full min-h-0 gap-4 xl:grid-cols-[minmax(0,1fr)_24rem]">
      <Card className="min-h-0 overflow-auto p-6">
        <p className="font-mono text-xs uppercase tracking-[0.22em] text-accent">Settings</p>
        <h3 className="mt-2 font-display text-3xl font-semibold">设置</h3>
        <p className="mt-2 text-sm text-muted">这里调整前端会话体验、证据栏展示和运行能力状态。服务端持久配置仍遵循后端 admin 权限。</p>

        <div className="mt-6 grid gap-4 lg:grid-cols-2">
          <section className="rounded-3xl border border-border/70 bg-surface/45 p-5">
            <h4 className="font-display text-2xl font-semibold">对话行为</h4>
            <label className="mt-4 block text-sm font-semibold text-muted">默认对话模式</label>
            <select value={mode} onChange={(event) => onModeChange(event.target.value)} className="focus-ring mt-2 w-full rounded-xl border-border/80 bg-panel/80 text-sm">
              <option value="agentic">Agentic · 工具调用</option>
              <option value="pipeline">Pipeline RAG · 固定检索链路</option>
            </select>
            <SettingToggle label="强制知识库引用" description="选中知识库时要求回答基于检索证据，并显示未命中警告。" checked={forceCitation} onChange={onForceCitationChange} />
          </section>

          <section className="rounded-3xl border border-border/70 bg-surface/45 p-5">
            <h4 className="font-display text-2xl font-semibold">右侧证据栏</h4>
            <SettingToggle label="显示检索候选" description="展示初始召回 chunk、分数和来源。" checked={evidenceOptions.showRetrieval} onChange={(value) => updateEvidenceOption('showRetrieval', value)} />
            <SettingToggle label="显示重排过程" description="展示 rerank 前后候选顺序。" checked={evidenceOptions.showRerank} onChange={(value) => updateEvidenceOption('showRerank', value)} />
            <SettingToggle label="显示工具 Trace" description="展示 Agent action/observation 和耗时。" checked={evidenceOptions.showTrace} onChange={(value) => updateEvidenceOption('showTrace', value)} />
            <SettingToggle label="显示进入上下文" description="展示最终注入 prompt 的文档片段。" checked={evidenceOptions.showContext} onChange={(value) => updateEvidenceOption('showContext', value)} />
          </section>
        </div>
      </Card>

      <div className="min-h-0 space-y-4 overflow-auto">
        <StatusCard title="System" icon={Activity} data={system.data} loading={system.isLoading} />
        <StatusCard title="MCP" icon={Wrench} data={mcp.data} loading={mcp.isLoading} />
        <StatusCard title="GraphRAG" icon={GitBranch} data={graph.data} loading={graph.isLoading} />
        <StatusCard title="Raw Settings" icon={Settings2} data={settings.data} loading={settings.isLoading} />
      </div>
    </div>
  );
}

function SettingToggle({ label, description, checked, onChange }: { label: string; description: string; checked: boolean; onChange: (checked: boolean) => void }) {
  return (
    <label className="mt-4 flex items-start gap-3 rounded-2xl bg-panel/55 p-3">
      <input type="checkbox" checked={checked} onChange={(event) => onChange(event.target.checked)} className="mt-1 rounded border-border text-primary" />
      <span>
        <span className="block text-sm font-semibold">{label}</span>
        <span className="mt-1 block text-xs leading-5 text-muted">{description}</span>
      </span>
    </label>
  );
}

function StatusCard({ title, icon: Icon, data, loading }: { title: string; icon: typeof Activity; data: unknown; loading: boolean }) {
  return (
    <Card className="p-5">
      <div className="mb-3 flex items-center justify-between gap-2">
        <h4 className="flex items-center gap-2 font-display text-xl font-semibold"><Icon className="h-5 w-5 text-primary" />{title}</h4>
        <Badge tone={loading ? 'warning' : 'success'}>{loading ? 'loading' : 'loaded'}</Badge>
      </div>
      <pre className="max-h-48 overflow-auto rounded-2xl bg-text/5 p-3 font-mono text-[11px] leading-5 text-muted">{JSON.stringify(data ?? {}, null, 2)}</pre>
    </Card>
  );
}
