import { useMemo, useState } from 'react';
import { BookOpen, Search } from 'lucide-react';
import { useKnowledgeBases, useWikiPage, useWikiPages } from '../hooks/queries';
import { useWorkspaceStore } from '../store/workspace';
import { Badge } from '../components/ui/Badge';
import { Card } from '../components/ui/Card';
import { EmptyState } from '../components/ui/EmptyState';
import { Input } from '../components/ui/Input';
import { renderSafeMarkdown } from '../lib/markdown';

export function WikiPageView() {
  const selectedKnowledgeBaseId = useWorkspaceStore((state) => state.selectedKnowledgeBaseId);
  const setSelectedKnowledgeBaseId = useWorkspaceStore((state) => state.setSelectedKnowledgeBaseId);
  const { data: kbData } = useKnowledgeBases();
  const wikiKbs = (kbData?.knowledge_bases ?? []).filter((kb) => kb.mode === 'wiki');
  const activeKbId = selectedKnowledgeBaseId && wikiKbs.some((kb) => kb.id === selectedKnowledgeBaseId) ? selectedKnowledgeBaseId : wikiKbs[0]?.id;
  const pages = useWikiPages(activeKbId);
  const [path, setPath] = useState<string | null>(null);
  const [query, setQuery] = useState('');
  const pageList = pages.data?.pages ?? [];
  const activePath = path ?? pageList.find((page) => page.path === 'index.md')?.path ?? pageList[0]?.path;
  const page = useWikiPage(activeKbId, activePath);
  const filtered = useMemo(() => pageList.filter((item) => `${item.title ?? ''} ${item.path}`.toLowerCase().includes(query.toLowerCase())), [pageList, query]);

  if (wikiKbs.length === 0) {
    return <EmptyState title="暂无 Wiki 知识库" description="在 Knowledge 页面创建 wiki 模式知识库并导入文件后，这里会展示页面树。" />;
  }

  return (
    <div className="grid h-full min-h-0 gap-4 lg:grid-cols-[21rem_minmax(0,1fr)]">
      <Card className="flex min-h-0 flex-col p-5">
        <div className="flex items-center gap-2">
          <BookOpen className="h-5 w-5 text-accent" />
          <h3 className="font-display text-2xl font-semibold">Wiki Browser</h3>
        </div>
        <select value={activeKbId} onChange={(event) => setSelectedKnowledgeBaseId(event.target.value)} className="focus-ring mt-4 w-full rounded-xl border-border/80 bg-panel/80 text-sm">
          {wikiKbs.map((kb) => <option key={kb.id} value={kb.id}>{kb.name}</option>)}
        </select>
        <div className="relative mt-4">
          <Search className="pointer-events-none absolute left-3 top-2.5 h-4 w-4 text-muted" />
          <Input className="pl-9" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="搜索页面路径" />
        </div>
        <div className="mt-4 min-h-0 flex-1 space-y-2 overflow-y-auto pr-1">
          {filtered.map((item) => (
            <button key={item.path} onClick={() => setPath(item.path)} className="focus-ring w-full rounded-2xl border border-border/70 bg-surface/45 p-3 text-left hover:bg-text/5">
              <span className="block truncate text-sm font-semibold">{item.title || item.path}</span>
              <span className="font-mono text-xs text-muted">{item.path}</span>
            </button>
          ))}
        </div>
      </Card>
      <Card className="min-h-0 overflow-auto p-7">
        {page.data ? (
          <article>
            <div className="mb-6 flex flex-wrap items-center justify-between gap-3">
              <div>
                <Badge tone="accent">{page.data.path}</Badge>
                <h2 className="mt-3 font-display text-4xl font-bold tracking-tight">{page.data.title || page.data.path}</h2>
              </div>
            </div>
            <div className="prose prose-slate max-w-none dark:prose-invert prose-headings:font-display prose-a:text-primary" dangerouslySetInnerHTML={{ __html: renderSafeMarkdown(page.data.content || page.data.excerpt || '') }} />
          </article>
        ) : <EmptyState title="选择一个 Wiki 页面" description="默认优先打开 index.md。" />}
      </Card>
    </div>
  );
}
