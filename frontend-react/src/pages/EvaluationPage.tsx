import { RefreshCw } from 'lucide-react';
import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { endpoints } from '../hooks/endpoints';
import { Button } from '../components/ui/Button';
import { Card } from '../components/ui/Card';
import { Badge } from '../components/ui/Badge';

export function EvaluationPage() {
  const reports = useQuery({ queryKey: ['eval-reports'], queryFn: endpoints.evalReports });
  const [selectedName, setSelectedName] = useState<string | null>(null);
  const detail = useQuery({ queryKey: ['eval-report', selectedName], queryFn: () => endpoints.evalReport(selectedName as string), enabled: Boolean(selectedName) });
  const report = detail.data?.report;
  const metric = (key: string) => typeof report?.[key] === 'number' ? Number(report[key]).toFixed(4) : '—';
  const retrieval = report?.retrieval_metrics as Record<string, unknown> | undefined;
  const retrievalMetric = (key: string) => typeof retrieval?.[key] === 'number' ? Number(retrieval[key]).toFixed(4) : '—';
  const latency = report?.latency_ms as { p50?: number; p95?: number } | undefined;
  const cases = Array.isArray(report?.details) ? report.details as Array<Record<string, unknown>> : [];
  return <div className="space-y-4">
    <Card className="flex items-center justify-between p-6">
      <div><p className="font-mono text-xs uppercase tracking-[0.22em] text-accent">Regression</p><h3 className="mt-2 font-display text-3xl font-semibold">评测中心</h3><p className="mt-2 text-sm text-muted">查看每次 Pipeline / Agentic 回归运行的配置、指标和结果文件。</p></div>
      <Button variant="secondary" onClick={() => reports.refetch()}><RefreshCw className="mr-2 h-4 w-4" />刷新</Button>
    </Card>
    <Card className="p-5"><div className="mb-4 flex items-center justify-between"><h4 className="font-display text-2xl font-semibold">评测报告</h4><Badge tone={reports.isLoading ? 'warning' : 'success'}>{reports.data?.reports?.length ?? 0} reports</Badge></div>
      {reports.data?.reports?.length ? <div className="space-y-2">{reports.data.reports.map((item) => <button type="button" key={item.name} onClick={() => setSelectedName(item.name)} className={`flex w-full items-center justify-between rounded-2xl border px-4 py-3 text-left ${selectedName === item.name ? 'border-accent bg-accent/10' : 'border-border/70 bg-surface/45'}`}><div><p className="font-medium">{item.name}</p><p className="mt-1 text-xs text-muted">{item.modified_at ?? '未知时间'}</p></div><span className="font-mono text-xs text-muted">{item.size ? `${Math.round(item.size / 1024)} KB` : '报告'}</span></button>)}</div> : <p className="rounded-2xl bg-surface/45 p-5 text-sm text-muted">暂无报告。运行评测脚本后刷新。</p>}
    </Card>
    {selectedName ? <Card className="p-5"><div className="flex items-center justify-between"><h4 className="font-display text-2xl font-semibold">指标详情</h4><Badge tone={detail.isLoading ? 'warning' : 'success'}>{detail.isLoading ? 'loading' : 'loaded'}</Badge></div>{report ? <><div className="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-6">{[['Answer Relevancy', 'answer_relevancy'], ['Faithfulness', 'faithfulness']].map(([label, key]) => <div key={key} className="rounded-2xl bg-surface/45 p-4"><p className="text-xs text-muted">{label}</p><p className="mt-2 font-mono text-2xl font-semibold">{metric(key)}</p></div>)}{[['Recall', 'recall'], ['Precision', 'precision'], ['MRR', 'mrr'], ['nDCG@10', 'ndcg10'], ['MAP', 'map']].map(([label, key]) => <div key={key} className="rounded-2xl bg-surface/45 p-4"><p className="text-xs text-muted">{label}</p><p className="mt-2 font-mono text-2xl font-semibold">{retrievalMetric(key)}</p></div>)}<div className="rounded-2xl bg-surface/45 p-4"><p className="text-xs text-muted">Latency P50</p><p className="mt-2 font-mono text-2xl font-semibold">{latency?.p50 ? `${latency.p50}ms` : '—'}</p></div><div className="rounded-2xl bg-surface/45 p-4"><p className="text-xs text-muted">Latency P95</p><p className="mt-2 font-mono text-2xl font-semibold">{latency?.p95 ? `${latency.p95}ms` : '—'}</p></div><div className="rounded-2xl bg-surface/45 p-4"><p className="text-xs text-muted">Samples</p><p className="mt-2 font-mono text-2xl font-semibold">{String(report.samples ?? '—')}</p></div><div className="rounded-2xl bg-surface/45 p-4"><p className="text-xs text-muted">Mode</p><p className="mt-2 font-mono text-lg font-semibold">{String(report.mode ?? '—')}</p></div></div>{cases.length ? <div className="mt-4 overflow-auto rounded-2xl border border-border/70"><table className="w-full min-w-[42rem] text-left text-sm"><thead className="bg-panel text-xs text-muted"><tr><th className="px-3 py-2">Case</th><th className="px-3 py-2">Status</th><th className="px-3 py-2">Contexts</th><th className="px-3 py-2">Latency</th><th className="px-3 py-2">Faithfulness</th></tr></thead><tbody>{cases.map((item, index) => <tr key={String(item.id ?? index)} className="border-t border-border/60"><td className="px-3 py-2 font-mono text-xs">{String(item.id ?? `case-${index + 1}`)}</td><td className="px-3 py-2">{String(item.status ?? '—')}</td><td className="px-3 py-2">{String(item.contexts ?? '—')}</td><td className="px-3 py-2">{item.latency_ms ? `${String(item.latency_ms)}ms` : '—'}</td><td className="px-3 py-2">{typeof item.faithfulness === 'number' ? item.faithfulness.toFixed(4) : '—'}</td></tr>)}</tbody></table></div> : null}<pre className="mt-4 max-h-96 overflow-auto rounded-2xl bg-text/5 p-4 font-mono text-xs leading-5 text-muted">{JSON.stringify(report, null, 2)}</pre></> : detail.data?.content ? <pre className="mt-4 max-h-[36rem] overflow-auto whitespace-pre-wrap rounded-2xl bg-text/5 p-4 text-sm text-muted">{detail.data.content}</pre> : <p className="mt-4 text-sm text-muted">{detail.isLoading ? '读取报告…' : '该报告没有可解析的内容。'}</p>}</Card> : null}
  </div>;
}
