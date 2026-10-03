import { useMemo, useState } from 'react'
import { useParams } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowLeft, ArrowRight, Network, RefreshCw, Search, Sparkles, Trash2 } from 'lucide-react'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { ConfirmDialog } from '@/components/ui/Dialog'
import { EmptyState } from '@/components/ui/EmptyState'
import { Input, Select } from '@/components/ui/Input'
import { PageSpinner } from '@/components/ui/Spinner'
import { apiDelete, apiGet, apiPost, errorMessage } from '@/lib/api'
import { cn } from '@/lib/cn'
import { describeGraphBuild, neighborsOf, normalizeGraph } from '@/lib/graph'
import { toast } from '@/store/toast'
import type { GraphBuildResponse, GraphRagStatus, GraphResponse } from '@/types/knowledge'
import { GraphCanvas } from './GraphCanvas'

const LIMITS = [100, 200, 500, 1000]
const LIST_LIMIT = 100

export default function GraphTab() {
  const { kbId = '' } = useParams()
  const qc = useQueryClient()
  const [limit, setLimit] = useState(200)
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [filter, setFilter] = useState('')
  const [confirm, setConfirm] = useState<'build' | 'delete' | null>(null)

  const status = useQuery({ queryKey: ['graphrag', 'status'], queryFn: () => apiGet<GraphRagStatus>('/graphrag/status') })
  const enabled = status.data?.enabled === true
  const graph = useQuery({
    queryKey: ['kb', kbId, 'graph', limit],
    queryFn: () => apiGet<GraphResponse>(`/knowledge-bases/${kbId}/graph`, { limit }),
    enabled,
  })

  const data = useMemo(() => normalizeGraph(graph.data ?? { nodes: null, edges: null }), [graph.data])
  const selected = data.nodes.find((n) => n.id === selectedId) ?? null
  const neighbors = useMemo(() => (selected ? neighborsOf(data.nodes, data.edges, selected.id) : []), [data, selected])

  const listed = useMemo(() => {
    const needle = filter.trim().toLowerCase()
    const matches = needle ? data.nodes.filter((n) => n.label.toLowerCase().includes(needle)) : data.nodes
    return [...matches].sort((a, b) => b.degree - a.degree).slice(0, LIST_LIMIT)
  }, [data.nodes, filter])

  const refresh = () => void qc.invalidateQueries({ queryKey: ['kb', kbId, 'graph'] })

  const build = useMutation({
    mutationFn: () => apiPost<GraphBuildResponse>(`/knowledge-bases/${kbId}/graph/build`),
    onSuccess: (res) => {
      toast.success(describeGraphBuild(res))
      refresh()
    },
    onError: (err) => toast.error(errorMessage(err)),
    onSettled: () => setConfirm(null),
  })
  const remove = useMutation({
    mutationFn: () => apiDelete(`/knowledge-bases/${kbId}/graph`),
    onSuccess: () => {
      toast.success('图谱已删除')
      setSelectedId(null)
      refresh()
    },
    onError: (err) => toast.error(errorMessage(err)),
    onSettled: () => setConfirm(null),
  })

  if (status.isPending) return <PageSpinner />
  if (status.isError) {
    return (
      <Card>
        <EmptyState title="无法获取图谱服务状态" description={errorMessage(status.error)} action={<Button onClick={() => void status.refetch()}>重试</Button>} />
      </Card>
    )
  }
  if (!enabled) {
    return (
      <Card>
        <EmptyState icon={<Network className="size-5" />} title="GraphRAG 未启用" description={status.data.message ?? '服务端没有配置图数据库，暂时不能构建或查看知识图谱。'} />
      </Card>
    )
  }

  const toolbar = (
    <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
      <div className="flex items-center gap-2 text-[13px] text-muted">
        <Badge variant={status.data.connected === false ? 'danger' : 'success'}>{status.data.connected === false ? '图数据库未连接' : '已连接'}</Badge>
        {graph.data ? (
          <span>
            {data.nodes.length} 个节点 · {data.edges.length} 条关系
          </span>
        ) : null}
      </div>
      <div className="flex flex-wrap items-center gap-2">
        <label className="flex items-center gap-1.5 text-[13px] whitespace-nowrap text-muted">
          节点上限
          <Select aria-label="节点上限" value={limit} onChange={(e) => setLimit(Number(e.target.value))} className="h-8 w-24">
            {LIMITS.map((l) => (
              <option key={l} value={l}>
                {l}
              </option>
            ))}
          </Select>
        </label>
        <Button size="sm" icon={<RefreshCw className="size-3.5" />} onClick={refresh} loading={graph.isFetching}>
          刷新
        </Button>
        <Button size="sm" variant="primary" icon={<Sparkles className="size-3.5" />} onClick={() => setConfirm('build')}>
          构建图谱
        </Button>
        <Button size="sm" variant="danger" icon={<Trash2 className="size-3.5" />} onClick={() => setConfirm('delete')}>
          删除图谱
        </Button>
      </div>
    </div>
  )

  return (
    <>
      {toolbar}

      {graph.isPending ? (
        <PageSpinner />
      ) : graph.isError ? (
        <Card>
          <EmptyState title="无法加载图谱" description={errorMessage(graph.error)} action={<Button onClick={() => void graph.refetch()}>重试</Button>} />
        </Card>
      ) : data.nodes.length === 0 ? (
        <Card>
          <EmptyState
            icon={<Network className="size-5" />}
            title="图谱还是空的"
            description="点击“构建图谱”，让大模型从这个知识库的片段里抽取实体和关系。"
            action={
              <Button variant="primary" icon={<Sparkles className="size-4" />} onClick={() => setConfirm('build')}>
                构建图谱
              </Button>
            }
          />
        </Card>
      ) : (
        <div className="grid gap-4 lg:grid-cols-[1fr_20rem]">
          <div className="h-[28rem] lg:h-[calc(100vh-18rem)] lg:min-h-[28rem]">
            <GraphCanvas nodes={data.nodes} edges={data.edges} selectedId={selected?.id ?? null} onSelect={setSelectedId} />
          </div>

          <Card className="flex max-h-[28rem] flex-col overflow-hidden lg:max-h-[calc(100vh-18rem)]">
            {selected ? (
              <div className="border-b border-border p-4">
                <div className="flex items-start justify-between gap-2">
                  <h2 className="text-[15px] font-semibold break-words">{selected.label}</h2>
                  <button className="shrink-0 text-xs text-muted hover:text-fg" onClick={() => setSelectedId(null)}>
                    取消选择
                  </button>
                </div>
                <p className="mt-1 text-xs text-muted">
                  {selected.degree} 条关系 · 出现在 {selected.chunk_count} 个片段
                </p>
                {neighbors.length > 0 ? (
                  <ul className="mt-2 max-h-40 space-y-0.5 overflow-y-auto">
                    {neighbors.map((nb, i) => (
                      <li key={`${nb.id}:${i}`}>
                        <button onClick={() => setSelectedId(nb.id)} className="flex w-full items-center gap-1.5 rounded-md px-1.5 py-1 text-left text-[13px] hover:bg-surface-2">
                          {nb.direction === 'out' ? <ArrowRight className="size-3 shrink-0 text-muted" /> : <ArrowLeft className="size-3 shrink-0 text-muted" />}
                          <span className="truncate">{nb.label}</span>
                          <span className="ml-auto shrink-0 text-xs text-muted">{nb.relation}</span>
                        </button>
                      </li>
                    ))}
                  </ul>
                ) : null}
              </div>
            ) : null}

            <div className="relative border-b border-border p-3">
              <Search className="pointer-events-none absolute top-1/2 left-6 size-4 -translate-y-1/2 text-muted" />
              <Input aria-label="查找节点" placeholder="查找节点" value={filter} onChange={(e) => setFilter(e.target.value)} className="pl-9" />
            </div>
            <ul className="min-h-0 flex-1 overflow-y-auto p-2" aria-label="节点列表">
              {listed.map((n) => (
                <li key={n.id}>
                  <button
                    onClick={() => setSelectedId(n.id)}
                    aria-pressed={n.id === selected?.id}
                    className={cn('flex w-full items-center justify-between gap-2 rounded-md px-2 py-1.5 text-left text-[13px]', n.id === selected?.id ? 'bg-primary-soft text-primary' : 'hover:bg-surface-2')}
                  >
                    <span className="truncate">{n.label}</span>
                    <span className="shrink-0 text-xs text-muted">{n.degree}</span>
                  </button>
                </li>
              ))}
              {listed.length === 0 ? <li className="px-2 py-6 text-center text-[13px] text-muted">没有匹配的节点</li> : null}
            </ul>
            {data.nodes.length >= limit ? <p className="border-t border-border px-3 py-2 text-xs text-muted">已达到节点上限 {limit}，调高上限可以看到更多。</p> : null}
          </Card>
        </div>
      )}

      <ConfirmDialog
        open={confirm === 'build'}
        title="构建知识图谱"
        confirmLabel="开始构建"
        loading={build.isPending}
        message="会读取这个知识库的全部片段，并调用大模型抽取实体和关系。片段多时可能要几分钟，期间请不要关闭页面。"
        onConfirm={() => build.mutate()}
        onClose={() => !build.isPending && setConfirm(null)}
      />
      <ConfirmDialog
        open={confirm === 'delete'}
        danger
        title="删除知识图谱"
        confirmLabel="删除"
        loading={remove.isPending}
        message="会清除这个知识库在图数据库中的全部节点和关系，文档和片段不受影响。"
        onConfirm={() => remove.mutate()}
        onClose={() => !remove.isPending && setConfirm(null)}
      />
    </>
  )
}
