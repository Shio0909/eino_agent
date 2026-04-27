import { useMemo, useState } from 'react';
import { GitBranch, Network, RefreshCw } from 'lucide-react';
import { useGraph, useKnowledgeBases } from '../hooks/queries';
import { useWorkspaceStore } from '../store/workspace';
import { Badge } from '../components/ui/Badge';
import { Button } from '../components/ui/Button';
import { Card } from '../components/ui/Card';
import { EmptyState } from '../components/ui/EmptyState';
import type { GraphData, GraphNode } from '../types/api';

interface PositionedNode extends GraphNode {
  x: number;
  y: number;
}

export function GraphPage() {
  const selectedKnowledgeBaseId = useWorkspaceStore((state) => state.selectedKnowledgeBaseId);
  const setSelectedKnowledgeBaseId = useWorkspaceStore((state) => state.setSelectedKnowledgeBaseId);
  const { data: kbData } = useKnowledgeBases();
  const kbs = kbData?.knowledge_bases ?? [];
  const activeKbId = selectedKnowledgeBaseId && kbs.some((kb) => kb.id === selectedKnowledgeBaseId) ? selectedKnowledgeBaseId : kbs[0]?.id;
  const graph = useGraph(activeKbId, 200);
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null);
  const layout = useMemo(() => buildLayout(graph.data), [graph.data]);
  const selectedNode = layout.nodes.find((node) => node.id === selectedNodeId) ?? layout.nodes[0];
  const selectedRelations = selectedNode ? layout.edges.filter((edge) => edge.source === selectedNode.id || edge.target === selectedNode.id) : [];

  if (kbs.length === 0) {
    return <EmptyState title="暂无知识库" description="先在 Knowledge 页面创建知识库并导入内容，再查看知识图谱。" />;
  }

  return (
    <div className="grid h-full min-h-0 gap-4 xl:grid-cols-[minmax(0,1fr)_22rem]">
      <Card className="flex min-h-0 flex-col p-5">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <p className="font-mono text-xs uppercase tracking-[0.22em] text-accent">GraphRAG</p>
            <h3 className="mt-2 flex items-center gap-2 font-display text-3xl font-semibold"><GitBranch className="h-6 w-6 text-primary" />知识图谱</h3>
            <p className="mt-2 text-sm text-muted">展示当前知识库的实体、关系和图谱密度，点击节点查看邻接关系。</p>
          </div>
          <div className="flex items-center gap-2">
            <Badge tone={graph.isLoading ? 'warning' : 'success'}>{graph.isLoading ? 'loading' : `${layout.nodes.length} nodes`}</Badge>
            <Button variant="ghost" onClick={() => graph.refetch()} disabled={graph.isFetching}><RefreshCw className="h-4 w-4" />刷新</Button>
          </div>
        </div>

        <div className="mt-4 flex flex-wrap items-center gap-3">
          <select value={activeKbId} onChange={(event) => setSelectedKnowledgeBaseId(event.target.value)} className="focus-ring min-w-64 rounded-xl border-border/80 bg-panel/80 text-sm">
            {kbs.map((kb) => <option key={kb.id} value={kb.id}>{kb.name}</option>)}
          </select>
          <Badge tone="accent">{layout.edges.length} relations</Badge>
        </div>

        <div className="mt-5 min-h-0 flex-1 rounded-3xl border border-border/70 bg-surface/35 p-3">
          {graph.isError ? <EmptyState title="图谱加载失败" description="确认 GraphRAG 已启用，并且该知识库已完成图谱构建。" /> : <GraphCanvas graph={layout} selectedNodeId={selectedNode?.id} onSelect={setSelectedNodeId} />}
        </div>
      </Card>

      <Card className="min-h-0 overflow-auto p-5">
        <div className="flex items-center gap-2">
          <Network className="h-5 w-5 text-accent" />
          <h4 className="font-display text-2xl font-semibold">节点详情</h4>
        </div>
        {!selectedNode ? <EmptyState title="暂无图谱数据" description="构建 GraphRAG 后，实体和关系会显示在这里。" /> : (
          <div className="mt-5 space-y-4">
            <section className="rounded-3xl border border-border/70 bg-surface/45 p-4">
              <Badge tone="primary">{selectedNode.id}</Badge>
              <h5 className="mt-3 font-display text-2xl font-semibold">{selectedNode.label}</h5>
              <div className="mt-4 grid grid-cols-2 gap-2 text-sm text-muted">
                <span>degree {selectedNode.degree}</span>
                <span>{selectedNode.chunk_count} chunks</span>
              </div>
            </section>
            <section className="rounded-3xl border border-border/70 bg-surface/45 p-4">
              <h5 className="text-sm font-semibold">邻接关系</h5>
              <div className="mt-3 space-y-2">
                {selectedRelations.length === 0 ? <p className="text-sm text-muted">暂无直接关系。</p> : selectedRelations.map((edge, index) => (
                  <button key={`${edge.source}-${edge.target}-${index}`} onClick={() => setSelectedNodeId(edge.source === selectedNode.id ? edge.target : edge.source)} className="focus-ring w-full rounded-2xl bg-panel/55 p-3 text-left transition hover:bg-text/5">
                    <span className="block text-xs font-semibold text-primary">{edge.label || 'RELATED'}</span>
                    <span className="mt-1 block truncate font-mono text-xs text-muted">{edge.source} → {edge.target}</span>
                  </button>
                ))}
              </div>
            </section>
          </div>
        )}
      </Card>
    </div>
  );
}

function GraphCanvas({ graph, selectedNodeId, onSelect }: { graph: ReturnType<typeof buildLayout>; selectedNodeId?: string; onSelect: (id: string) => void }) {
  if (graph.nodes.length === 0) {
    return <EmptyState title="暂无图谱数据" description="该知识库还没有可视化节点。可以先完成文档导入和 GraphRAG 构建。" />;
  }

  return (
    <svg viewBox="0 0 1100 760" className="h-full min-h-[34rem] w-full">
      <defs>
        <marker id="graph-arrow" markerHeight="8" markerWidth="8" orient="auto" refX="7" refY="4">
          <path d="M0,0 L8,4 L0,8 Z" className="fill-muted" />
        </marker>
      </defs>
      {graph.edges.map((edge, index) => {
        const source = graph.nodeMap.get(edge.source);
        const target = graph.nodeMap.get(edge.target);
        if (!source || !target) return null;
        const midX = (source.x + target.x) / 2;
        const midY = (source.y + target.y) / 2;
        return (
          <g key={`${edge.source}-${edge.target}-${index}`}>
            <line x1={source.x} y1={source.y} x2={target.x} y2={target.y} stroke="currentColor" strokeWidth="1.5" markerEnd="url(#graph-arrow)" className="text-border" />
            {edge.label ? <text x={midX} y={midY - 6} textAnchor="middle" className="fill-muted text-[10px]">{edge.label}</text> : null}
          </g>
        );
      })}
      {graph.nodes.map((node) => {
        const selected = node.id === selectedNodeId;
        const radius = Math.min(26, 12 + node.degree + node.chunk_count * 0.35);
        return (
          <g key={node.id} role="button" tabIndex={0} onClick={() => onSelect(node.id)} onKeyDown={(event) => event.key === 'Enter' && onSelect(node.id)} className="cursor-pointer outline-none">
            <circle cx={node.x} cy={node.y} r={radius} className={selected ? 'fill-primary stroke-text' : 'fill-panel stroke-primary'} strokeWidth={selected ? 3 : 2} />
            <text x={node.x} y={node.y + 4} textAnchor="middle" className={selected ? 'fill-white text-xs font-semibold' : 'fill-text text-xs font-semibold'}>{compactLabel(node.label)}</text>
          </g>
        );
      })}
    </svg>
  );
}

export function buildLayout(data?: GraphData) {
  const nodes = data?.nodes ?? [];
  const edges = data?.edges ?? [];
  const centerX = 550;
  const centerY = 380;
  const sorted = [...nodes].sort((left, right) => right.degree - left.degree || right.chunk_count - left.chunk_count);
  const positioned = sorted.map((node, index) => {
    if (index === 0) return { ...node, x: centerX, y: centerY };

    let ringIndex = 1;
    let firstIndexInRing = 1;
    let ringCapacity = 6;
    while (index >= firstIndexInRing + ringCapacity) {
      firstIndexInRing += ringCapacity;
      ringIndex += 1;
      ringCapacity = ringIndex * 6;
    }

    const positionInRing = index - firstIndexInRing;
    const angle = (positionInRing / ringCapacity) * Math.PI * 2 - Math.PI / 2;
    const radius = 86 + ringIndex * 96;

    return { ...node, x: centerX + Math.cos(angle) * radius, y: centerY + Math.sin(angle) * radius };
  });

  return {
    nodes: positioned,
    edges,
    nodeMap: new Map<string, PositionedNode>(positioned.map((node) => [node.id, node])),
  };
}

function compactLabel(value: string) {
  return value.length > 8 ? `${value.slice(0, 8)}…` : value;
}
