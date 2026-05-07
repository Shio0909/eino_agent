import { FormEvent, useEffect, useState } from 'react';
import { ArrowLeft, UploadCloud, Link as LinkIcon, Plus } from 'lucide-react';
import { endpoints } from '../hooks/endpoints';
import { useCreateKnowledgeBase, useDocuments, useKnowledgeBases, useOpsStatus } from '../hooks/queries';
import { useWorkspaceStore } from '../store/workspace';
import { Badge } from '../components/ui/Badge';
import { Button } from '../components/ui/Button';
import { Card } from '../components/ui/Card';
import { EmptyState } from '../components/ui/EmptyState';
import { Input } from '../components/ui/Input';
import { compactNumber, formatDate, statusTone } from '../lib/format';

export function KnowledgePage() {
  const selectedKnowledgeBaseId = useWorkspaceStore((state) => state.selectedKnowledgeBaseId);
  const setSelectedKnowledgeBaseId = useWorkspaceStore((state) => state.setSelectedKnowledgeBaseId);
  const { data, refetch } = useKnowledgeBases();
  const createKb = useCreateKnowledgeBase();
  const documents = useDocuments(selectedKnowledgeBaseId);
  const { settings } = useOpsStatus();
  const docreader = settings.data?.settings?.docreader;
  const [name, setName] = useState('');
  const [mode, setMode] = useState<'vector' | 'wiki'>('vector');
  const [description, setDescription] = useState('');
  const [url, setUrl] = useState('');
  const [title, setTitle] = useState('');
  const [message, setMessage] = useState<{ text: string; tone: 'warning' | 'success' | 'error' } | null>(null);

  const kbs = data?.knowledge_bases ?? [];
  const selected = kbs.find((kb) => kb.id === selectedKnowledgeBaseId);
  const docItems = documents.data?.documents ?? [];

  useEffect(() => {
    if (!message || docItems.length === 0) return;
    const hasProcessing = docItems.some((doc) => !['completed', 'failed'].includes(documentStatus(doc)));
    const hasFailed = docItems.some((doc) => documentStatus(doc) === 'failed');
    if (hasFailed) {
      setMessage({ text: '导入失败，请查看文档状态或后端日志。', tone: 'error' });
    } else if (!hasProcessing) {
      setMessage({ text: '文档已完成解析、切块和基础入库；上下文增强会在后台继续处理。', tone: 'success' });
    }
  }, [docItems, message]);

  const create = async (event: FormEvent) => {
    event.preventDefault();
    if (!name.trim()) return;
    const kb = await createKb.mutateAsync({ name, description, mode });
    setSelectedKnowledgeBaseId(kb.id);
    setName('');
    setDescription('');
    setMode('vector');
  };

  const upload = async (file?: File) => {
    if (!file || !selected?.id) return;
    setMessage({ text: selected.mode === 'wiki' ? 'Wiki 模式：文件已提交，LLM 编译中…' : '文件已提交，正在解析、切块并写入基础索引；上下文增强将在后台继续…', tone: 'warning' });
    await endpoints.uploadDocument(selected.id, file);
    documents.refetch();
    refetch();
  };

  const importUrl = async (event: FormEvent) => {
    event.preventDefault();
    if (!url.trim() || !selected?.id) return;
    setMessage({ text: selected.mode === 'wiki' ? 'URL 已提交，Wiki 页面编译中…' : 'URL 已提交，正在导入、切块并写入基础索引；上下文增强将在后台继续…', tone: 'warning' });
    await endpoints.importUrl(selected.id, url, title || url);
    setUrl('');
    setTitle('');
    documents.refetch();
  };

  return (
    <div className="grid h-full min-h-0 gap-4 xl:grid-cols-[minmax(0,1fr)_24rem]">
      <Card className="flex min-h-0 flex-col p-5">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <p className="font-mono text-xs uppercase tracking-[0.22em] text-accent">Knowledge Bases</p>
            <h3 className="font-display text-3xl font-semibold">知识库</h3>
          </div>
          <Badge>{data?.total ?? 0} total</Badge>
        </div>
        <div className="mt-5 min-h-0 flex-1 overflow-y-auto pr-1">
          {selected ? (
            <div className="rounded-3xl border border-border/70 bg-surface/35 p-5">
              <button type="button" onClick={() => setSelectedKnowledgeBaseId(null)} className="focus-ring mb-4 inline-flex items-center gap-2 rounded-xl px-3 py-2 text-sm font-semibold text-primary hover:bg-text/5">
                <ArrowLeft className="h-4 w-4" /> 返回全部知识库
              </button>
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <div className="flex items-center gap-2">
                    <h4 className="font-display text-2xl font-semibold">{selected.name}</h4>
                    <Badge tone={selected.mode === 'wiki' ? 'accent' : 'primary'}>{selected.mode}</Badge>
                  </div>
                  <p className="mt-2 text-sm text-muted">{selected.description || '暂无描述'}</p>
                </div>
                <div className="grid grid-cols-2 gap-2 text-xs text-muted">
                  <span>{compactNumber(selected.document_count)} docs</span>
                  <span>{compactNumber(selected.chunk_count)} chunks</span>
                </div>
              </div>
            </div>
          ) : (
            <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
              {kbs.map((kb) => (
                <button key={kb.id} onClick={() => setSelectedKnowledgeBaseId(kb.id)} className="focus-ring rounded-3xl border border-border/70 bg-surface/45 p-4 text-left transition hover:-translate-y-0.5 hover:bg-panel/80">
                  <div className="flex items-start justify-between gap-2">
                    <h4 className="font-display text-xl font-semibold">{kb.name}</h4>
                    <Badge tone={kb.mode === 'wiki' ? 'accent' : 'primary'}>{kb.mode}</Badge>
                  </div>
                  <p className="mt-2 line-clamp-2 text-sm text-muted">{kb.description || '暂无描述'}</p>
                  <div className="mt-4 grid grid-cols-2 gap-2 text-xs text-muted">
                    <span>{compactNumber(kb.document_count)} docs</span>
                    <span>{compactNumber(kb.chunk_count)} chunks</span>
                  </div>
                  {kb.embed_stale ? <Badge tone="warning" className="mt-3">embedding stale</Badge> : null}
                </button>
              ))}
            </div>
          )}
        </div>
        <div className="mt-6 min-h-0 shrink-0">
          {!selected ? <EmptyState title="选择一个知识库" description="点击上方知识库卡片查看文档、上传文件或导入 URL。" /> : (
            <div>
              <div className="flex items-center justify-between">
                <h4 className="font-display text-2xl font-semibold">{selected.name} · 文档</h4>
                <span className="text-sm text-muted">{formatDate(selected.updated_at)}</span>
              </div>
              {message ? <p className={`mt-3 rounded-2xl px-4 py-3 text-sm ${messageToneClass(message.tone)}`}>{message.text}</p> : null}
              <div className="mt-4 max-h-72 overflow-auto rounded-3xl border border-border/70">
                <table className="w-full min-w-[42rem] text-left text-sm">
                  <thead className="sticky top-0 z-10 bg-panel text-xs uppercase tracking-wide text-muted">
                    <tr><th className="px-4 py-3">文档</th><th className="px-4 py-3">状态</th><th className="px-4 py-3">Chunks</th><th className="px-4 py-3">更新时间</th></tr>
                  </thead>
                  <tbody>
                    {(docItems).map((doc) => (
                      <tr key={doc.id} className="border-t border-border/70">
                        <td className="px-4 py-3 font-medium">{doc.title || doc.filename || doc.source || doc.id}</td>
                        <td className="px-4 py-3">
                          <div className="flex flex-col gap-1">
                            <Badge tone={statusTone(documentStatus(doc))}>{doc.stage || documentStatus(doc) || 'unknown'}</Badge>
                            {doc.enrichment_status ? <Badge tone={statusTone(doc.enrichment_status)}>增强 {doc.enrichment_status}</Badge> : null}
                          </div>
                        </td>
                        <td className="px-4 py-3 text-muted">{doc.chunk_count ?? 0}</td>
                        <td className="px-4 py-3 text-muted">{formatDate(doc.updated_at)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}
        </div>
      </Card>
      <div className="space-y-4">
        <Card className="p-5">
          <h3 className="flex items-center gap-2 font-display text-2xl font-semibold"><Plus className="h-5 w-5" />创建知识库</h3>
          <form className="mt-4 space-y-3" onSubmit={create}>
            <Input value={name} onChange={(event) => setName(event.target.value)} placeholder="知识库名称" />
            <Input value={description} onChange={(event) => setDescription(event.target.value)} placeholder="描述" />
            <select value={mode} onChange={(event) => setMode(event.target.value as 'vector' | 'wiki')} className="focus-ring w-full rounded-xl border-border/80 bg-panel/80 text-sm">
              <option value="vector">vector · 语义检索</option>
              <option value="wiki">wiki · LLM 编译页面</option>
            </select>
            <Button className="w-full" disabled={createKb.isPending}>创建</Button>
          </form>
        </Card>
        <Card className="p-5">
          <h3 className="flex items-center gap-2 font-display text-2xl font-semibold"><UploadCloud className="h-5 w-5" />上传文件</h3>
          {docreader ? (
            <div className="mt-4 rounded-2xl border border-border/70 bg-surface/45 p-3 text-xs text-muted">
              <div className="flex items-center justify-between gap-2">
                <span className="font-semibold text-text">文档解析</span>
                <Badge tone={docreader.active ? 'success' : 'warning'}>{docreader.active ? 'ready' : 'fallback'}</Badge>
              </div>
              <div className="mt-2 grid grid-cols-2 gap-2">
                <span>模式 {docreader.mode || 'unknown'}</span>
                <span>主解析 {docreader.primary || 'local'}</span>
                <span>降级 {docreader.fallback || 'none'}</span>
                <span>{docreader.mineru_endpoint || docreader.endpoint || '未配置端点'}</span>
              </div>
            </div>
          ) : null}
          <input className="mt-4 block w-full cursor-pointer rounded-2xl border border-dashed border-border/80 bg-surface/45 p-4 text-sm" type="file" onChange={(event) => upload(event.target.files?.[0])} disabled={!selected} />
        </Card>
        <Card className="p-5">
          <h3 className="flex items-center gap-2 font-display text-2xl font-semibold"><LinkIcon className="h-5 w-5" />URL 导入</h3>
          <form className="mt-4 space-y-3" onSubmit={importUrl}>
            <Input value={url} onChange={(event) => setUrl(event.target.value)} placeholder="https://example.com/doc" />
            <Input value={title} onChange={(event) => setTitle(event.target.value)} placeholder="标题，可选" />
            <Button className="w-full" disabled={!selected}>导入 URL</Button>
          </form>
        </Card>
      </div>
    </div>
  );
}

function documentStatus(doc: { status?: string; parse_status?: string }) {
  return doc.status || doc.parse_status || '';
}

function messageToneClass(tone: 'warning' | 'success' | 'error') {
  if (tone === 'success') return 'bg-success/10 text-success';
  if (tone === 'error') return 'bg-error/10 text-error';
  return 'bg-warning/10 text-warning';
}
