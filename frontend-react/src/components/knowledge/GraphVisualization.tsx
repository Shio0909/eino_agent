import { useEffect, useState, useRef, useCallback } from 'react'
import ForceGraph2D from 'react-force-graph-2d'
import { Card, EmptyState } from '../ui'
import { Loader2, Network, ZoomIn, ZoomOut, Maximize2 } from 'lucide-react'
import * as api from '../../lib/api'

interface Props {
  kbId: string
}

interface GraphNode {
  id: string
  label: string
  degree: number
  chunk_count: number
  x?: number
  y?: number
}

interface GraphLink {
  source: string | GraphNode
  target: string | GraphNode
  label: string
}

export default function GraphVisualization({ kbId }: Props) {
  const [nodes, setNodes] = useState<GraphNode[]>([])
  const [links, setLinks] = useState<GraphLink[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [hoverNode, setHoverNode] = useState<GraphNode | null>(null)
  const graphRef = useRef<any>(null)
  const containerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    setError('')
    api.getGraphVisualization(kbId, 200).then((data) => {
      if (cancelled) return
      const graphNodes: GraphNode[] = (data.nodes || []).map((n) => ({
        id: n.id,
        label: n.label,
        degree: n.degree,
        chunk_count: n.chunk_count,
      }))
      const nodeIds = new Set(graphNodes.map((n) => n.id))
      const graphLinks: GraphLink[] = (data.edges || [])
        .filter((e) => nodeIds.has(e.source) && nodeIds.has(e.target))
        .map((e) => ({ source: e.source, target: e.target, label: e.label }))
      setNodes(graphNodes)
      setLinks(graphLinks)
      setLoading(false)
    }).catch((err) => {
      if (cancelled) return
      setError(err.message || '加载图谱失败')
      setLoading(false)
    })
    return () => { cancelled = true }
  }, [kbId])

  // Auto-fit when data loads
  useEffect(() => {
    if (nodes.length > 0 && graphRef.current) {
      setTimeout(() => graphRef.current?.zoomToFit(400, 40), 500)
    }
  }, [nodes])

  const handleZoomIn = useCallback(() => {
    const fg = graphRef.current
    if (fg) {
      const z = fg.zoom()
      fg.zoom(z * 1.5, 300)
    }
  }, [])

  const handleZoomOut = useCallback(() => {
    const fg = graphRef.current
    if (fg) {
      const z = fg.zoom()
      fg.zoom(z / 1.5, 300)
    }
  }, [])

  const handleFit = useCallback(() => {
    graphRef.current?.zoomToFit(400, 40)
  }, [])

  const nodeColor = useCallback((node: GraphNode) => {
    const d = node.degree || 1
    if (d >= 8) return '#ef4444'  // red — hub
    if (d >= 4) return '#f59e0b'  // amber
    if (d >= 2) return '#3b82f6'  // blue
    return '#6b7280'              // gray — leaf
  }, [])

  const nodeSize = useCallback((node: GraphNode) => {
    return Math.max(3, Math.min(12, 3 + (node.degree || 0)))
  }, [])

  if (loading) {
    return (
      <Card className="p-8 flex items-center justify-center gap-2 text-[var(--color-text-muted)]">
        <Loader2 size={18} className="animate-spin" />
        <span className="text-sm">加载图谱数据...</span>
      </Card>
    )
  }

  if (error) {
    return (
      <Card className="p-6">
        <p className="text-sm text-red-500">图谱加载失败: {error}</p>
      </Card>
    )
  }

  if (nodes.length === 0) {
    return (
      <Card className="p-6">
        <EmptyState
          icon={Network}
          title="暂无图谱数据"
          description="上传文档并构建图谱后，这里会显示实体关系图"
        />
      </Card>
    )
  }

  return (
    <Card className="relative overflow-hidden">
      <div className="px-4 py-3 border-b border-[var(--color-border-subtle)] flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Network size={16} className="text-[var(--color-accent)]" />
          <span className="text-sm font-medium text-[var(--color-text-primary)]">
            知识图谱预览
          </span>
          <span className="text-xs text-[var(--color-text-muted)]">
            {nodes.length} 节点 · {links.length} 关系
          </span>
        </div>
        <div className="flex items-center gap-1">
          <button onClick={handleZoomIn} className="p-1.5 rounded hover:bg-[var(--color-bg-tertiary)] text-[var(--color-text-muted)]" title="放大">
            <ZoomIn size={14} />
          </button>
          <button onClick={handleZoomOut} className="p-1.5 rounded hover:bg-[var(--color-bg-tertiary)] text-[var(--color-text-muted)]" title="缩小">
            <ZoomOut size={14} />
          </button>
          <button onClick={handleFit} className="p-1.5 rounded hover:bg-[var(--color-bg-tertiary)] text-[var(--color-text-muted)]" title="适配">
            <Maximize2 size={14} />
          </button>
        </div>
      </div>

      <div ref={containerRef} className="h-[420px] bg-[var(--color-bg-secondary)]">
        <ForceGraph2D
          ref={graphRef}
          graphData={{ nodes, links }}
          nodeLabel={(node: any) => `${node.label} (度: ${node.degree}, chunks: ${node.chunk_count})`}
          nodeColor={(node: any) => nodeColor(node)}
          nodeRelSize={1}
          nodeVal={(node: any) => nodeSize(node)}
          linkLabel={(link: any) => link.label}
          linkColor={() => 'rgba(156,163,175,0.4)'}
          linkDirectionalArrowLength={3}
          linkDirectionalArrowRelPos={1}
          onNodeHover={(node: any) => setHoverNode(node)}
          enableNodeDrag={true}
          enableZoomInteraction={true}
          cooldownTicks={80}
          width={containerRef.current?.clientWidth || 600}
          height={420}
        />
      </div>

      {hoverNode && (
        <div className="absolute bottom-2 left-2 bg-[var(--color-bg-primary)] border border-[var(--color-border)] rounded-lg px-3 py-2 shadow-lg text-xs">
          <p className="font-medium text-[var(--color-text-primary)]">{hoverNode.label}</p>
          <p className="text-[var(--color-text-muted)]">
            关联度: {hoverNode.degree} · 关联文档: {hoverNode.chunk_count}
          </p>
        </div>
      )}
    </Card>
  )
}
