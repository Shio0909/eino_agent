import { formatDuration } from '@/lib/format'
import type { GraphBuildResponse, GraphEdge, GraphNode, GraphResponse } from '@/types/knowledge'

export type { GraphEdge, GraphNode }

export interface Point {
  x: number
  y: number
}

export interface Transform {
  scale: number
  tx: number
  ty: number
}

export interface PlacedNode extends Point {
  id: string
  /** Radius in screen pixels. */
  r: number
}

/** Fills in the null lists of an empty graph and drops what could not be drawn. */
export function normalizeGraph(data: GraphResponse): { nodes: GraphNode[]; edges: GraphEdge[] } {
  const seen = new Set<string>()
  const nodes: GraphNode[] = []
  for (const n of data.nodes ?? []) {
    if (seen.has(n.id)) continue
    seen.add(n.id)
    nodes.push(n)
  }
  const edges = (data.edges ?? []).filter((e) => e.source !== e.target && seen.has(e.source) && seen.has(e.target))
  return { nodes, edges }
}

/** Small seeded generator so the same graph always gets the same starting layout. */
function mulberry32(seed: number): () => number {
  let a = seed >>> 0
  return () => {
    a = (a + 0x6d2b79f5) >>> 0
    let t = a
    t = Math.imul(t ^ (t >>> 15), t | 1)
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61)
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}

const MARGIN = 10
const SPACING = 0.4
const GRAVITY = 0.6

export function layoutRounds(n: number): number {
  return Math.max(40, Math.min(300, Math.floor(6e7 / (n * n))))
}

/**
 * Force-directed layout (Fruchterman-Reingold): every pair repels and every edge pulls,
 * and a pull towards the centre keeps unconnected parts from being pushed out to the edges.
 * Pair work grows with the square of the node count, so large graphs get fewer rounds.
 */
export function layoutGraph(nodes: GraphNode[], edges: GraphEdge[], size: { width: number; height: number }): Map<string, Point> {
  const out = new Map<string, Point>()
  const n = nodes.length
  const cx = size.width / 2
  const cy = size.height / 2
  if (n === 0) return out
  if (n === 1) return out.set(nodes[0].id, { x: cx, y: cy })

  const rand = mulberry32(n)
  const xs = new Float64Array(n)
  const ys = new Float64Array(n)
  const radius = Math.min(size.width, size.height) * 0.4
  for (let i = 0; i < n; i++) {
    const angle = (2 * Math.PI * i) / n
    const r = radius * (0.6 + 0.4 * rand())
    xs[i] = cx + r * Math.cos(angle)
    ys[i] = cy + r * Math.sin(angle)
  }

  const index = new Map(nodes.map((node, i) => [node.id, i]))
  const links = edges.flatMap((e) => {
    const a = index.get(e.source)
    const b = index.get(e.target)
    return a === undefined || b === undefined ? [] : [[a, b] as const]
  })

  const k = Math.sqrt((size.width * size.height) / n) * SPACING
  const rounds = layoutRounds(n)
  const dx = new Float64Array(n)
  const dy = new Float64Array(n)

  for (let round = 0; round < rounds; round++) {
    const temperature = (Math.min(size.width, size.height) / 8) * (1 - round / rounds) + 0.5
    dx.fill(0)
    dy.fill(0)

    for (let i = 0; i < n; i++) {
      for (let j = i + 1; j < n; j++) {
        let vx = xs[i] - xs[j]
        let vy = ys[i] - ys[j]
        let d2 = vx * vx + vy * vy
        if (d2 < 0.01) {
          // Coincident nodes: split them along a direction that depends on the pair.
          vx = ((i * 7 + j) % 5) - 2 + 0.5
          vy = ((i + j * 3) % 5) - 2 + 0.5
          d2 = vx * vx + vy * vy
        }
        const f = (k * k) / d2
        dx[i] += vx * f
        dy[i] += vy * f
        dx[j] -= vx * f
        dy[j] -= vy * f
      }
    }
    for (const [a, b] of links) {
      const vx = xs[a] - xs[b]
      const vy = ys[a] - ys[b]
      const d = Math.sqrt(vx * vx + vy * vy) || 0.01
      const f = d / k
      dx[a] -= vx * f
      dy[a] -= vy * f
      dx[b] += vx * f
      dy[b] += vy * f
    }
    for (let i = 0; i < n; i++) {
      dx[i] += (cx - xs[i]) * GRAVITY
      dy[i] += (cy - ys[i]) * GRAVITY
      const len = Math.sqrt(dx[i] * dx[i] + dy[i] * dy[i]) || 1
      const step = Math.min(len, temperature)
      xs[i] = Math.min(size.width - MARGIN, Math.max(MARGIN, xs[i] + (dx[i] / len) * step))
      ys[i] = Math.min(size.height - MARGIN, Math.max(MARGIN, ys[i] + (dy[i] / len) * step))
    }
  }

  nodes.forEach((node, i) => out.set(node.id, { x: xs[i], y: ys[i] }))
  return out
}

const MIN_RADIUS = 6
const MAX_RADIUS = 16

/** Busier nodes are drawn larger. */
export function nodeRadius(degree: number, maxDegree: number): number {
  if (maxDegree <= 0) return MIN_RADIUS
  return MIN_RADIUS + (MAX_RADIUS - MIN_RADIUS) * (Math.min(degree, maxDegree) / maxDegree)
}

const MAX_FIT_SCALE = 2

/** Scale and offset that fit the points inside a view, centred, without zooming small graphs past 2x. */
export function fitTransform(points: Point[], width: number, height: number, padding: number): Transform {
  if (points.length === 0) return { scale: 1, tx: 0, ty: 0 }
  let minX = Infinity
  let maxX = -Infinity
  let minY = Infinity
  let maxY = -Infinity
  for (const p of points) {
    minX = Math.min(minX, p.x)
    maxX = Math.max(maxX, p.x)
    minY = Math.min(minY, p.y)
    maxY = Math.max(maxY, p.y)
  }
  const w = maxX - minX
  const h = maxY - minY
  const sx = w > 0 ? (width - 2 * padding) / w : Infinity
  const sy = h > 0 ? (height - 2 * padding) / h : Infinity
  const scale = Math.min(sx, sy, MAX_FIT_SCALE)
  return { scale, tx: width / 2 - ((minX + maxX) / 2) * scale, ty: height / 2 - ((minY + maxY) / 2) * scale }
}

const HIT_SLACK = 4

/** The node under a screen point (the nearest centre when several overlap), or null. */
export function hitTest(nodes: PlacedNode[], t: Transform, px: number, py: number): string | null {
  let best: string | null = null
  let bestDist = Infinity
  for (const n of nodes) {
    const d = Math.hypot(n.x * t.scale + t.tx - px, n.y * t.scale + t.ty - py)
    if (d <= n.r + HIT_SLACK && d < bestDist) {
      best = n.id
      bestDist = d
    }
  }
  return best
}

export interface Neighbor {
  id: string
  label: string
  relation: string
  direction: 'in' | 'out'
}

/** The nodes joined to `id` by an edge, in edge order. */
export function neighborsOf(nodes: GraphNode[], edges: GraphEdge[], id: string): Neighbor[] {
  const labels = new Map(nodes.map((n) => [n.id, n.label]))
  const out: Neighbor[] = []
  for (const e of edges) {
    if (e.source === id) out.push({ id: e.target, label: labels.get(e.target) ?? e.target, relation: e.label, direction: 'out' })
    else if (e.target === id) out.push({ id: e.source, label: labels.get(e.source) ?? e.source, relation: e.label, direction: 'in' })
  }
  return out
}

export function describeGraphBuild(res: GraphBuildResponse): string {
  const head = `图谱构建完成：${res.chunk_count} 个片段`
  const r = res.result
  if (!r) return head
  const failed = r.failed_chunks > 0 ? `，${r.failed_chunks} 个片段处理失败` : ''
  return `${head}，${r.processed_nodes} 个节点，${r.processed_relations} 条关系${failed}，耗时 ${formatDuration(r.duration / 1e6)}`
}
