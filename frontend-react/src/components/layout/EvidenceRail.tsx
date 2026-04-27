import { useState, type ReactNode } from 'react';
import { Activity, FileText, GitCompareArrows, Link2, Search, Wrench } from 'lucide-react';
import { renderSafeMarkdown } from '../../lib/markdown';
import { isLikelyGarbledSourceText } from '../../lib/sourceText';
import { Badge } from '../ui/Badge';
import type { ReferenceDocument, TraceStep } from '../../types/api';

interface TraceChunk {
  rank?: number;
  doc_id?: string;
  content?: string;
  score?: number;
  match_type?: string;
  source?: string;
  metadata?: Record<string, unknown>;
}

interface EvidenceRailOptions {
  showRetrieval: boolean;
  showRerank: boolean;
  showTrace: boolean;
  showContext: boolean;
}

interface EvidenceRailProps {
  references: ReferenceDocument[];
  trace: TraceStep[];
  streaming?: boolean;
  options?: EvidenceRailOptions;
}

const defaultOptions: EvidenceRailOptions = {
  showRetrieval: true,
  showRerank: true,
  showTrace: true,
  showContext: true,
};

export function EvidenceRail({ references, trace, streaming, options = defaultOptions }: EvidenceRailProps) {
  const [expandedSource, setExpandedSource] = useState<string | null>(null);
  const retrievalStep = trace.find((step) => step.stage === 'retrieved_candidates');
  const rerankSteps = trace.filter((step) => step.stage === 'rerank');
  const contextStep = trace.find((step) => step.stage === 'context_build');
  const timeline = trace.filter((step) => !['retrieved_candidates', 'rerank', 'context_build'].includes(step.stage ?? ''));
  const retrievedChunks = asChunks(retrievalStep?.metadata?.chunks);
  const rerankBefore = asChunks(rerankSteps.find((step) => Array.isArray(step.metadata?.before))?.metadata?.before);
  const rerankAfter = asChunks([...rerankSteps].reverse().find((step) => Array.isArray(step.metadata?.after))?.metadata?.after);
  const contextChunks = asChunks(contextStep?.metadata?.chunks);

  return (
    <aside className="glass-panel flex h-full min-h-0 flex-col rounded-[2rem] p-5">
      <div className="flex items-center justify-between">
        <div>
          <p className="font-mono text-xs uppercase tracking-[0.22em] text-accent">Evidence</p>
          <h3 className="font-display text-2xl font-semibold">证据流</h3>
        </div>
        {streaming ? <Badge tone="warning">streaming</Badge> : <Badge tone="muted">ready</Badge>}
      </div>
      <div className="mt-5 min-h-0 flex-1 space-y-4 overflow-auto pr-1">
        <section className="rounded-3xl border border-border/70 bg-surface/35 p-3">
          <h4 className="mb-2 flex items-center gap-2 text-sm font-semibold"><Activity className="h-4 w-4" />LLM 调用 / 思考链</h4>
          <div className="space-y-3">
            {options.showRetrieval ? (
              <RailGroup title="检索候选" icon={<Search className="h-4 w-4" />}>
                {retrievedChunks.length === 0 ? <p className="text-sm text-muted">检索到的 chunk、来源和分数会显示在这里。</p> : <ChunkList chunks={retrievedChunks} emptyText="暂无检索候选" />}
              </RailGroup>
            ) : null}
            {options.showRerank ? (
              <RailGroup title="重排" icon={<GitCompareArrows className="h-4 w-4" />}>
                {rerankBefore.length === 0 && rerankAfter.length === 0 ? <p className="text-sm text-muted">重排前后顺序会显示在这里。</p> : (
                  <div className="space-y-3">
                    <RankColumn title="Before" chunks={rerankBefore} />
                    <RankColumn title="After" chunks={rerankAfter} />
                  </div>
                )}
              </RailGroup>
            ) : null}
            {options.showTrace ? (
              <RailGroup title="Trace" icon={<Wrench className="h-4 w-4" />}>
                {timeline.length === 0 ? <p className="text-sm text-muted">Agent 工具调用、检索阶段和耗时会在这里展开。</p> : timeline.map((step, index) => (
                  <TraceCard key={`${step.type}-${index}`} step={step} />
                ))}
              </RailGroup>
            ) : null}
          </div>
        </section>

        <section className="rounded-3xl border border-border/70 bg-surface/35 p-3">
          <h4 className="mb-2 flex items-center gap-2 text-sm font-semibold"><FileText className="h-4 w-4" />文档来源页</h4>
          {references.length === 0 ? <p className="text-sm text-muted">暂无引用，发送问题后会在这里显示来源。</p> : references.map((ref, index) => {
            const sourceId = `source-${index + 1}`;
            const expanded = expandedSource === sourceId;
            const garbled = isLikelyGarbledSourceText(ref.content);
            return (
              <article id={sourceId} key={`${ref.id}-${index}`} className="mb-3 scroll-mt-4 rounded-2xl border border-border/70 bg-surface/45 p-3">
                <button type="button" onClick={() => setExpandedSource(expanded ? null : sourceId)} className="focus-ring w-full text-left">
                  <div className="flex items-center justify-between gap-2">
                    <div className="flex items-center gap-2">
                      <Badge tone="primary">来源{index + 1}</Badge>
                      {garbled ? <Badge tone="warning">疑似乱码</Badge> : null}
                    </div>
                    <span className="truncate text-xs text-muted">{sourceLabel(ref)}</span>
                  </div>
                  <p className={`${expanded ? '' : 'line-clamp-4'} mt-2 text-xs leading-5 text-muted`}>{sourcePreview(ref.content)}</p>
                  <p className="mt-2 text-[11px] font-semibold text-primary">{expanded ? '收起来源全文' : '展开来源全文'}</p>
                </button>
                {expanded ? <SourceContent content={ref.content} garbled={garbled} /> : null}
              </article>
            );
          })}
          {options.showContext ? (
            <RailGroup title="进入上下文" icon={<Link2 className="h-4 w-4" />}>
              <ChunkList chunks={contextChunks} emptyText="最终注入 prompt 的上下文 chunk 会显示在这里。" compact />
            </RailGroup>
          ) : null}
        </section>
      </div>
    </aside>
  );
}

function RailGroup({ title, icon, children }: { title: string; icon: ReactNode; children: ReactNode }) {
  return (
    <div className="rounded-2xl border border-border/60 bg-panel/45 p-3">
      <h5 className="mb-2 flex items-center gap-2 text-xs font-semibold text-muted">{icon}{title}</h5>
      {children}
    </div>
  );
}

function ChunkList({ chunks, emptyText, compact }: { chunks: TraceChunk[]; emptyText: string; compact?: boolean }) {
  if (chunks.length === 0) return <p className="text-sm text-muted">{emptyText}</p>;
  return (
    <div className="space-y-2">
      {chunks.map((chunk, index) => <ChunkCard key={`${chunk.doc_id}-${index}`} chunk={chunk} compact={compact} />)}
    </div>
  );
}

function RankColumn({ title, chunks }: { title: string; chunks: TraceChunk[] }) {
  return (
    <div className="rounded-2xl border border-border/70 bg-surface/35 p-3">
      <div className="mb-2 flex items-center justify-between">
        <span className="font-mono text-xs font-semibold uppercase tracking-[0.16em] text-muted">{title}</span>
        <Badge>{chunks.length}</Badge>
      </div>
      <ChunkList chunks={chunks} emptyText="暂无重排数据" compact />
    </div>
  );
}

function ChunkCard({ chunk, compact }: { chunk: TraceChunk; compact?: boolean }) {
  return (
    <article className="rounded-2xl border border-border/70 bg-surface/45 p-3">
      <div className="flex items-center justify-between gap-2">
        <div className="flex min-w-0 items-center gap-2">
          <Badge tone="accent">#{chunk.rank ?? '-'}</Badge>
          {chunk.match_type ? <Badge tone="primary">{chunk.match_type}</Badge> : null}
        </div>
        <span className="truncate font-mono text-[11px] text-muted">{chunk.doc_id ?? 'unknown'}</span>
      </div>
      <div className="mt-2 flex items-center justify-between gap-2 text-[11px] text-muted">
        <span className="truncate">{chunk.source || inferSource(chunk)}</span>
        <span className="font-mono">score {formatScore(chunk.score)}</span>
      </div>
      {!compact && chunk.content ? <p className="mt-2 line-clamp-4 text-xs leading-5 text-muted">{chunk.content}</p> : null}
    </article>
  );
}

function SourceContent({ content, garbled }: { content: string; garbled: boolean }) {
  if (garbled) {
    return (
      <div className="mt-3 rounded-2xl border border-warning/30 bg-warning/10 p-3">
        <p className="text-xs font-semibold text-warning">该来源文本疑似 PDF 解析乱码，请重新导入、启用 OCR 或检查后端解析器。</p>
        <pre className="mt-3 max-h-48 overflow-auto whitespace-pre-wrap rounded-xl bg-panel/70 p-3 font-mono text-[11px] leading-5 text-muted">{content}</pre>
      </div>
    );
  }

  return <div className="prose prose-slate mt-3 max-w-none rounded-2xl bg-panel/70 p-3 text-xs dark:prose-invert prose-headings:font-display prose-p:my-2 prose-ul:my-2 prose-ol:my-2 prose-li:my-0 prose-pre:whitespace-pre-wrap" dangerouslySetInnerHTML={{ __html: renderSafeMarkdown(content) }} />;
}

function TraceCard({ step }: { step: TraceStep }) {
  const parsed = parseObservation(step.content);
  return (
    <article className="mb-3 rounded-2xl border border-border/70 bg-panel/55 p-3">
      <div className="flex items-center gap-2">
        {step.tool_name ? <Wrench className="h-4 w-4 text-accent" /> : <Link2 className="h-4 w-4 text-primary" />}
        <span className="text-sm font-semibold">{traceTitle(step)}</span>
        {step.latency_ms ? <Badge>{step.latency_ms}ms</Badge> : null}
      </div>
      {step.tool_name ? <p className="mt-2 font-mono text-xs text-muted">{step.tool_name}</p> : null}
      {parsed.length > 0 ? (
        <div className="mt-2 space-y-2">
          {parsed.slice(0, 3).map((item, index) => (
            <div key={index} className="rounded-xl bg-surface/45 p-2">
              <div className="flex items-center justify-between gap-2 text-[11px] text-muted">
                <span className="truncate">{item.source || `result ${index + 1}`}</span>
                {typeof item.score === 'number' ? <span className="font-mono">{formatScore(item.score)}</span> : null}
              </div>
              <p className="mt-1 line-clamp-3 text-xs leading-5 text-muted">{item.content}</p>
            </div>
          ))}
          {parsed.length > 3 ? <p className="text-[11px] text-muted">还有 {parsed.length - 3} 条结果已折叠。</p> : null}
        </div>
      ) : step.content ? <p className="mt-2 line-clamp-4 text-xs leading-5 text-muted">{compactTraceContent(step.content)}</p> : null}
    </article>
  );
}

function traceTitle(step: TraceStep) {
  if (step.type === 'observation') return `observation · ${step.tool_name || step.stage || 'tool'}`;
  if (step.type === 'action') return `action · ${step.tool_name || step.stage || 'tool'}`;
  return step.stage || step.type;
}

function parseObservation(content?: string): TraceChunk[] {
  if (!content?.trim().startsWith('{')) return [];
  try {
    const data = JSON.parse(content) as { results?: unknown };
    return asChunks(data.results);
  } catch {
    return [];
  }
}

function compactTraceContent(content: string) {
  return content.length > 500 ? `${content.slice(0, 500)}…` : content;
}

function asChunks(value: unknown): TraceChunk[] {
  return Array.isArray(value) ? value.filter((item): item is TraceChunk => typeof item === 'object' && item !== null) : [];
}

function formatScore(score: unknown) {
  return typeof score === 'number' ? String(Math.round(score * 1000) / 1000) : '-';
}

function sourceLabel(ref: ReferenceDocument) {
  return String(ref.metadata?.wiki_path ?? ref.metadata?.source_filename ?? ref.metadata?.source ?? ref.source ?? ref.id);
}

function sourcePreview(content: string) {
  return content.replace(/\[\[([^\]|]+\|)?([^\]]+)\]\]/g, '$2');
}

function inferSource(chunk: TraceChunk) {
  const metadata = chunk.metadata ?? {};
  return String(metadata.wiki_path ?? metadata.source_filename ?? metadata.source ?? metadata.file_name ?? 'unknown source');
}
