import { FormEvent, useState } from 'react';
import { UploadCloud, Link as LinkIcon, Plus } from 'lucide-react';
import { endpoints } from '../hooks/endpoints';
import { useCreateKnowledgeBase, useDocuments, useKnowledgeBases } from '../hooks/queries';
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
  const [name, setName] = useState('');
  const [mode, setMode] = useState<'vector' | 'wiki'>('vector');
  const [description, setDescription] = useState('');
  const [url, setUrl] = useState('');
  const [title, setTitle] = useState('');
  const [message, setMessage] = useState('');

  const kbs = data?.knowledge_bases ?? [];
  const selected = kbs.find((kb) => kb.id === selectedKnowledgeBaseId) ?? kbs[0];
  if (!selectedKnowledgeBaseId && selected?.id) setSelectedKnowledgeBaseId(selected.id);

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
    setMessage(selected.mode === 'wiki' ? 'Wiki 模式：文件已提交，LLM 编译中…' : '文件已提交，正在解析并切块…');
    await endpoints.uploadDocument(selected.id, file);
    documents.refetch();
    refetch();
  };

  const importUrl = async (event: FormEvent) => {
    event.preventDefault();
    if (!url.trim() || !selected?.id) return;
    setMessage(selected.mode === 'wiki' ? 'URL 已提交，Wiki 页面编译中…' : 'URL 已提交，正在导入…');
    await endpoints.importUrl(selected.id, url, title || url);
    setUrl('');
    setTitle('');
    documents.refetch();
  };

  return (
    <div className="grid h-full min-h-0 gap-4 xl:grid-cols-[minmax(0,1fr)_24rem]">
      <Card className="min-h-0 p-5">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <p className="font-mono text-xs uppercase tracking-[0.22em] text-accent">Knowledge Bases</p>
            <h3 className="font-display text-3xl font-semibold">知识库</h3>
          </div>
          <Badge>{data?.total ?? 0} total</Badge>
        </div>
        <div className="mt-5 grid gap-3 md:grid-cols-2 xl:grid-cols-3">
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
        <div className="mt-6">
          {!selected ? <EmptyState title="还没有知识库" description="先在右侧创建 vector 或 wiki 知识库。" /> : (
            <div>
              <div className="flex items-center justify-between">
                <h4 className="font-display text-2xl font-semibold">{selected.name} · 文档</h4>
                <span className="text-sm text-muted">{formatDate(selected.updated_at)}</span>
              </div>
              {message ? <p className="mt-3 rounded-2xl bg-warning/10 px-4 py-3 text-sm text-warning">{message}</p> : null}
              <div className="mt-4 overflow-hidden rounded-3xl border border-border/70">
                <table className="w-full text-left text-sm">
                  <thead className="bg-text/5 text-xs uppercase tracking-wide text-muted">
                    <tr><th className="px-4 py-3">文档</th><th className="px-4 py-3">状态</th><th className="px-4 py-3">Chunks</th><th className="px-4 py-3">更新时间</th></tr>
                  </thead>
                  <tbody>
                    {(documents.data?.documents ?? []).map((doc) => (
                      <tr key={doc.id} className="border-t border-border/70">
                        <td className="px-4 py-3 font-medium">{doc.title || doc.filename || doc.source || doc.id}</td>
                        <td className="px-4 py-3"><Badge tone={statusTone(doc.status)}>{doc.stage || doc.status || 'unknown'}</Badge></td>
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
