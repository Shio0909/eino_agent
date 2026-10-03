import { describe, expect, it } from 'vitest'
import {
  describeGraphBuild,
  fitTransform,
  hitTest,
  layoutGraph,
  layoutRounds,
  neighborsOf,
  nodeRadius,
  normalizeGraph,
  type GraphEdge,
  type GraphNode,
} from '@/lib/graph'

const node = (id: string, degree = 1): GraphNode => ({ id, label: id.toUpperCase(), degree, chunk_count: 1 })
const edge = (source: string, target: string, label = 'REL'): GraphEdge => ({ source, target, label })
const dist = (a: { x: number; y: number }, b: { x: number; y: number }) => Math.hypot(a.x - b.x, a.y - b.y)

describe('normalizeGraph', () => {
  it('treats the null nodes and edges of an empty graph as empty lists', () => {
    expect(normalizeGraph({ nodes: null, edges: null })).toEqual({ nodes: [], edges: [] })
    expect(normalizeGraph({} as never)).toEqual({ nodes: [], edges: [] })
  })

  it('drops edges whose ends are not in the node list, and self loops', () => {
    const g = normalizeGraph({ nodes: [node('a'), node('b')], edges: [edge('a', 'b'), edge('a', 'zzz'), edge('zzz', 'b'), edge('a', 'a')] })
    expect(g.edges).toEqual([edge('a', 'b')])
  })

  it('keeps the first of several nodes with the same id', () => {
    const g = normalizeGraph({ nodes: [{ ...node('a'), label: 'first' }, { ...node('a'), label: 'second' }], edges: [] })
    expect(g.nodes).toHaveLength(1)
    expect(g.nodes[0].label).toBe('first')
  })
})

describe('layoutGraph', () => {
  const size = { width: 1000, height: 700 }

  it('returns no positions for no nodes and the centre for a single node', () => {
    expect(layoutGraph([], [], size).size).toBe(0)
    const one = layoutGraph([node('a')], [], size).get('a')!
    expect(one).toEqual({ x: 500, y: 350 })
  })

  it('is deterministic', () => {
    const nodes = ['a', 'b', 'c', 'd'].map((id) => node(id))
    const edges = [edge('a', 'b'), edge('b', 'c'), edge('c', 'd')]
    expect([...layoutGraph(nodes, edges, size)]).toEqual([...layoutGraph(nodes, edges, size)])
  })

  it('places every node at a finite point inside the area, apart from the others', () => {
    const nodes = Array.from({ length: 40 }, (_, i) => node(`n${i}`))
    const edges = nodes.slice(1).map((n, i) => edge(`n${i}`, n.id))
    const pos = layoutGraph(nodes, edges, size)
    const pts = [...pos.values()]
    expect(pts).toHaveLength(40)
    for (const p of pts) {
      expect(Number.isFinite(p.x) && Number.isFinite(p.y)).toBe(true)
      expect(p.x).toBeGreaterThanOrEqual(0)
      expect(p.x).toBeLessThanOrEqual(size.width)
      expect(p.y).toBeGreaterThanOrEqual(0)
      expect(p.y).toBeLessThanOrEqual(size.height)
    }
    for (let i = 0; i < pts.length; i++) for (let j = i + 1; j < pts.length; j++) expect(dist(pts[i], pts[j])).toBeGreaterThan(1)
  })

  it('pulls connected nodes together: two cliques joined by one edge separate', () => {
    const left = ['a1', 'a2', 'a3', 'a4', 'a5']
    const right = ['b1', 'b2', 'b3', 'b4', 'b5']
    const clique = (ids: string[]) => ids.flatMap((s, i) => ids.slice(i + 1).map((t) => edge(s, t)))
    // Interleaved so that the starting ring does not already group each clique together.
    const interleaved = left.flatMap((a, i) => [a, right[i]])
    const pos = layoutGraph(interleaved.map((id) => node(id)), [...clique(left), ...clique(right), edge('a1', 'b1')], size)
    const mean = (pairs: [string, string][]) => pairs.reduce((sum, [a, b]) => sum + dist(pos.get(a)!, pos.get(b)!), 0) / pairs.length
    const within = [...left.flatMap((a, i) => left.slice(i + 1).map((b) => [a, b] as [string, string])), ...right.flatMap((a, i) => right.slice(i + 1).map((b) => [a, b] as [string, string]))]
    const across = left.flatMap((a) => right.map((b) => [a, b] as [string, string]))
    expect(mean(within)).toBeLessThan(mean(across))
  })

  it('keeps the two ends of an edge closer than the same two nodes without it', () => {
    const nodes = [node('a'), node('b')]
    const joined = layoutGraph(nodes, [edge('a', 'b')], size)
    const apart = layoutGraph(nodes, [], size)
    expect(dist(joined.get('a')!, joined.get('b')!)).toBeLessThan(dist(apart.get('a')!, apart.get('b')!))
  })

  it('lays a scrambled chain out so that neighbours are much closer than strangers', () => {
    const order = [0, 7, 3, 10, 5, 1, 8, 4, 11, 2, 9, 6] // chain order, listed out of order
    const ids = order.map((i) => `n${i}`)
    const edges = Array.from({ length: 11 }, (_, i) => edge(`n${i}`, `n${i + 1}`))
    const pos = layoutGraph(ids.map((id) => node(id)), edges, size)
    const edgeMean = edges.reduce((sum, e) => sum + dist(pos.get(e.source)!, pos.get(e.target)!), 0) / edges.length
    let pairs = 0
    let pairSum = 0
    for (let i = 0; i < ids.length; i++)
      for (let j = i + 1; j < ids.length; j++) {
        pairSum += dist(pos.get(ids[i])!, pos.get(ids[j])!)
        pairs++
      }
    expect(edgeMean).toBeLessThan((pairSum / pairs) * 0.5)
  })

  it('keeps separate components close instead of pushing them to opposite edges', () => {
    const a = ['a1', 'a2', 'a3', 'a4'].map((id) => node(id))
    const b = ['b1', 'b2', 'b3', 'b4'].map((id) => node(id))
    const ring = (ns: GraphNode[]) => ns.map((n, i) => edge(n.id, ns[(i + 1) % ns.length].id))
    const pos = layoutGraph([...a, ...b], [...ring(a), ...ring(b)], size)
    const centre = (ns: GraphNode[]) => ({
      x: ns.reduce((sum, n) => sum + pos.get(n.id)!.x, 0) / ns.length,
      y: ns.reduce((sum, n) => sum + pos.get(n.id)!.y, 0) / ns.length,
    })
    expect(dist(centre(a), centre(b))).toBeLessThan(450)
  })

  it('handles disconnected nodes without producing NaN', () => {
    const pos = layoutGraph([node('a'), node('b'), node('c')], [], size)
    for (const p of pos.values()) expect(Number.isFinite(p.x) && Number.isFinite(p.y)).toBe(true)
  })

  it('does fewer rounds on bigger graphs, but never fewer than 40 or more than 300', () => {
    expect(layoutRounds(10)).toBe(300)
    expect(layoutRounds(500)).toBe(240)
    expect(layoutRounds(1000)).toBe(60)
    expect(layoutRounds(5000)).toBe(40)
  })

  it('copes with a thousand nodes', () => {
    const nodes = Array.from({ length: 1000 }, (_, i) => node(`n${i}`))
    const edges = nodes.slice(1).map((n, i) => edge(`n${Math.floor(i / 2)}`, n.id))
    const pos = layoutGraph(nodes, edges, size)
    expect(pos.size).toBe(1000)
  })
})

describe('nodeRadius', () => {
  it('grows with degree but stays within bounds', () => {
    expect(nodeRadius(0, 10)).toBe(6)
    expect(nodeRadius(10, 10)).toBe(16)
    expect(nodeRadius(5, 10)).toBeGreaterThan(nodeRadius(1, 10))
    expect(nodeRadius(5, 10)).toBeLessThan(nodeRadius(10, 10))
  })
  it('does not divide by zero when no node has an edge', () => {
    expect(nodeRadius(0, 0)).toBe(6)
  })
})

describe('fitTransform', () => {
  it('scales and centres the points inside the view with padding', () => {
    const t = fitTransform([{ x: 0, y: 0 }, { x: 1000, y: 500 }], 400, 300, 20)
    expect(t.scale).toBeCloseTo(0.36) // (400 - 2*20) / 1000
    expect(t.tx).toBeCloseTo(20)
    expect(1000 * t.scale + t.tx).toBeCloseTo(380)
    expect((t.ty + (500 * t.scale + t.ty)) / 2).toBeCloseTo(150) // vertically centred
  })

  it('caps the zoom so a tiny graph is not blown up', () => {
    expect(fitTransform([{ x: 5, y: 5 }], 400, 300, 20).scale).toBeLessThanOrEqual(2)
  })

  it('returns an identity-like transform centred on the view for no points', () => {
    const t = fitTransform([], 400, 300, 20)
    expect(t.scale).toBe(1)
    expect(t.tx).toBe(0)
    expect(t.ty).toBe(0)
  })
})

describe('hitTest', () => {
  const pts = [
    { id: 'a', x: 100, y: 100, r: 10 },
    { id: 'b', x: 105, y: 100, r: 10 },
    { id: 'c', x: 300, y: 300, r: 10 },
  ]
  const identity = { scale: 1, tx: 0, ty: 0 }

  it('finds the node under the pointer, preferring the nearest centre', () => {
    expect(hitTest(pts, identity, 300, 305)).toBe('c')
    expect(hitTest(pts, identity, 101, 100)).toBe('a')
    expect(hitTest(pts, identity, 104, 100)).toBe('b')
  })
  it('returns null on empty space', () => {
    expect(hitTest(pts, identity, 200, 200)).toBeNull()
  })
  it('accounts for pan and zoom', () => {
    const t = { scale: 2, tx: 10, ty: -20 }
    // c is drawn at (300*2+10, 300*2-20) = (610, 580) with radius 10 * scale? radius stays in screen pixels
    expect(hitTest(pts, t, 610, 580)).toBe('c')
    expect(hitTest(pts, t, 300, 300)).toBeNull()
  })
  it('allows a little slack around small nodes so they are easy to hit', () => {
    expect(hitTest(pts, identity, 300 + 13, 300)).toBe('c')
    expect(hitTest(pts, identity, 300 + 20, 300)).toBeNull()
  })
})

describe('neighborsOf', () => {
  const nodes = [node('a'), node('b'), node('c')]
  const edges = [edge('a', 'b', 'USES'), edge('c', 'a', 'DEPENDS_ON'), edge('b', 'c')]

  it('lists the other end of each edge with the relation and its direction', () => {
    expect(neighborsOf(nodes, edges, 'a')).toEqual([
      { id: 'b', label: 'B', relation: 'USES', direction: 'out' },
      { id: 'c', label: 'C', relation: 'DEPENDS_ON', direction: 'in' },
    ])
  })
  it('returns nothing for an unknown or isolated node', () => {
    expect(neighborsOf(nodes, edges, 'zzz')).toEqual([])
    expect(neighborsOf([...nodes, node('d')], edges, 'd')).toEqual([])
  })
})

describe('describeGraphBuild', () => {
  it('summarises the counts and converts the nanosecond duration', () => {
    const text = describeGraphBuild({
      chunk_count: 120,
      result: { total_chunks: 120, processed_nodes: 300, processed_relations: 250, failed_chunks: 0, duration: 12_345_678_900 },
    })
    expect(text).toBe('图谱构建完成：120 个片段，300 个节点，250 条关系，耗时 12.3 s')
  })
  it('mentions chunks that failed', () => {
    const text = describeGraphBuild({
      chunk_count: 10,
      result: { total_chunks: 10, processed_nodes: 5, processed_relations: 4, failed_chunks: 3, duration: 500_000_000 },
    })
    expect(text).toContain('3 个片段处理失败')
    expect(text).toContain('耗时 500 ms')
  })
  it('copes with a response that has no result', () => {
    expect(describeGraphBuild({ chunk_count: 7 })).toBe('图谱构建完成：7 个片段')
  })
})
