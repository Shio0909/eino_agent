import { useEffect, useMemo, useRef, useState } from 'react'
import { Maximize, Minus, Plus } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { fitTransform, hitTest, layoutGraph, neighborsOf, nodeRadius, type GraphEdge, type GraphNode, type PlacedNode, type Transform } from '@/lib/graph'
import { useTheme } from '@/store/theme'

interface GraphCanvasProps {
  nodes: GraphNode[]
  edges: GraphEdge[]
  selectedId: string | null
  onSelect: (id: string | null) => void
}

const WORLD = { width: 1000, height: 700 }
const PADDING = 40
const MIN_SCALE = 0.1
const MAX_SCALE = 6
const DRAG_THRESHOLD = 4

interface Palette {
  bg: string
  fg: string
  muted: string
  border: string
  primary: string
}

function readPalette(): Palette {
  const css = getComputedStyle(document.documentElement)
  const get = (name: string) => css.getPropertyValue(name).trim()
  return { bg: get('--surface'), fg: get('--fg'), muted: get('--muted'), border: get('--border'), primary: get('--primary') }
}

export function GraphCanvas({ nodes, edges, selectedId, onSelect }: GraphCanvasProps) {
  const theme = useTheme((s) => s.theme)
  const wrapRef = useRef<HTMLDivElement>(null)
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const [box, setBox] = useState({ w: 0, h: 0 })
  // The user's pan and zoom, tied to the graph it was made on; a different graph starts fitted again.
  const [view, setView] = useState<{ layout: unknown; t: Transform } | null>(null)
  const [hoverId, setHoverId] = useState<string | null>(null)
  const [dragging, setDragging] = useState(false)

  const layout = useMemo(() => {
    const pos = layoutGraph(nodes, edges, WORLD)
    const maxDegree = nodes.reduce((m, n) => Math.max(m, n.degree), 0)
    const placed: PlacedNode[] = nodes.map((n) => ({ id: n.id, ...pos.get(n.id)!, r: nodeRadius(n.degree, maxDegree) }))
    return { placed, byId: new Map(placed.map((p) => [p.id, p])), labels: new Map(nodes.map((n) => [n.id, n.label])) }
  }, [nodes, edges])

  useEffect(() => {
    const el = wrapRef.current
    if (!el) return
    const observer = new ResizeObserver(([entry]) => setBox({ w: Math.floor(entry.contentRect.width), h: Math.floor(entry.contentRect.height) }))
    observer.observe(el)
    return () => observer.disconnect()
  }, [])

  const current = view && view.layout === layout ? view.t : box.w > 0 ? fitTransform(layout.placed, box.w, box.h, PADDING) : null
  // Wheel events can arrive several times between renders, so zooming builds on the latest value, not the last render's.
  const latest = useRef<Transform | null>(null)
  useEffect(() => {
    latest.current = current
  })
  const apply = (t: Transform) => {
    latest.current = t
    setView({ layout, t })
  }

  const neighborIds = useMemo(() => {
    if (!selectedId) return null
    return new Set([selectedId, ...neighborsOf(nodes, edges, selectedId).map((n) => n.id)])
  }, [nodes, edges, selectedId])

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas || !current || box.w === 0) return
    const dpr = window.devicePixelRatio || 1
    canvas.width = box.w * dpr
    canvas.height = box.h * dpr
    const ctx = canvas.getContext('2d')
    if (!ctx) return
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
    ctx.clearRect(0, 0, box.w, box.h)
    const c = readPalette()
    const sx = (x: number) => x * current.scale + current.tx
    const sy = (y: number) => y * current.scale + current.ty

    ctx.lineWidth = 1
    for (const e of edges) {
      const a = layout.byId.get(e.source)!
      const b = layout.byId.get(e.target)!
      const active = !!neighborIds && (e.source === selectedId || e.target === selectedId)
      ctx.globalAlpha = neighborIds ? (active ? 1 : 0.12) : 0.55
      ctx.strokeStyle = active ? c.primary : c.border
      ctx.lineWidth = active ? 1.5 : 1
      ctx.beginPath()
      ctx.moveTo(sx(a.x), sy(a.y))
      ctx.lineTo(sx(b.x), sy(b.y))
      ctx.stroke()
      if (active) {
        ctx.globalAlpha = 1
        ctx.fillStyle = c.muted
        ctx.font = '10px system-ui, sans-serif'
        ctx.textAlign = 'center'
        ctx.fillText(e.label, (sx(a.x) + sx(b.x)) / 2, (sy(a.y) + sy(b.y)) / 2 - 3)
      }
    }

    ctx.font = '12px system-ui, sans-serif'
    ctx.textAlign = 'center'
    ctx.lineJoin = 'round'
    for (const n of layout.placed) {
      const dim = !!neighborIds && !neighborIds.has(n.id)
      const x = sx(n.x)
      const y = sy(n.y)
      ctx.globalAlpha = dim ? 0.2 : 1
      ctx.beginPath()
      ctx.arc(x, y, n.r, 0, Math.PI * 2)
      ctx.fillStyle = c.primary
      ctx.fill()
      if (n.id === selectedId || n.id === hoverId) {
        ctx.lineWidth = 2.5
        ctx.strokeStyle = c.fg
        ctx.stroke()
      }
      const showLabel = n.id === selectedId || n.id === hoverId || (neighborIds?.has(n.id) ?? false) || n.r >= 11 || current.scale >= 1.4
      if (showLabel && !dim) {
        const text = layout.labels.get(n.id) ?? n.id
        ctx.lineWidth = 3
        ctx.strokeStyle = c.bg
        ctx.strokeText(text, x, y + n.r + 13)
        ctx.fillStyle = c.fg
        ctx.fillText(text, x, y + n.r + 13)
      }
    }
    ctx.globalAlpha = 1
  }, [current, box, layout, edges, selectedId, hoverId, neighborIds, theme])

  const drag = useRef<{ x: number; y: number; moved: boolean; view: Transform } | null>(null)

  const localPoint = (e: { clientX: number; clientY: number }) => {
    const rect = canvasRef.current!.getBoundingClientRect()
    return { x: e.clientX - rect.left, y: e.clientY - rect.top }
  }

  const zoomAt = (px: number, py: number, factor: number) => {
    const base = latest.current
    if (!base) return
    const scale = Math.min(MAX_SCALE, Math.max(MIN_SCALE, base.scale * factor))
    const k = scale / base.scale
    apply({ scale, tx: px - (px - base.tx) * k, ty: py - (py - base.ty) * k })
  }

  // React registers wheel listeners as passive, which would not let the page stay still while zooming.
  const zoomRef = useRef(zoomAt)
  useEffect(() => {
    zoomRef.current = zoomAt
  })
  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return
    const onWheel = (e: WheelEvent) => {
      e.preventDefault()
      const rect = canvas.getBoundingClientRect()
      zoomRef.current(e.clientX - rect.left, e.clientY - rect.top, Math.exp(-e.deltaY * 0.0015))
    }
    canvas.addEventListener('wheel', onWheel, { passive: false })
    return () => canvas.removeEventListener('wheel', onWheel)
  }, [])

  return (
    <div ref={wrapRef} className="relative h-full min-h-96 w-full overflow-hidden rounded-xl border border-border bg-surface">
      <canvas
        ref={canvasRef}
        role="img"
        aria-label={`知识图谱，${nodes.length} 个节点，${edges.length} 条关系。可在右侧列表中选择节点。`}
        style={{ width: box.w, height: box.h, cursor: dragging ? 'grabbing' : hoverId ? 'pointer' : 'grab', touchAction: 'none' }}
        onPointerDown={(e) => {
          if (!current) return
          e.currentTarget.setPointerCapture(e.pointerId)
          drag.current = { x: e.clientX, y: e.clientY, moved: false, view: current }
          setDragging(true)
        }}
        onPointerMove={(e) => {
          const d = drag.current
          if (d) {
            const dx = e.clientX - d.x
            const dy = e.clientY - d.y
            if (!d.moved && Math.hypot(dx, dy) < DRAG_THRESHOLD) return
            d.moved = true
            apply({ scale: d.view.scale, tx: d.view.tx + dx, ty: d.view.ty + dy })
            return
          }
          if (!current) return
          const p = localPoint(e)
          const id = hitTest(layout.placed, current, p.x, p.y)
          if (id !== hoverId) setHoverId(id)
        }}
        onPointerUp={(e) => {
          const d = drag.current
          drag.current = null
          setDragging(false)
          if (!d || d.moved || !current) return
          const p = localPoint(e)
          onSelect(hitTest(layout.placed, current, p.x, p.y))
        }}
        onPointerLeave={() => setHoverId(null)}
      />
      <div className="absolute right-3 bottom-3 flex flex-col gap-1">
        <Button size="icon" aria-label="放大" onClick={() => zoomAt(box.w / 2, box.h / 2, 1.3)}>
          <Plus className="size-4" />
        </Button>
        <Button size="icon" aria-label="缩小" onClick={() => zoomAt(box.w / 2, box.h / 2, 1 / 1.3)}>
          <Minus className="size-4" />
        </Button>
        <Button size="icon" aria-label="适应窗口" onClick={() => apply(fitTransform(layout.placed, box.w, box.h, PADDING))}>
          <Maximize className="size-4" />
        </Button>
      </div>
    </div>
  )
}
