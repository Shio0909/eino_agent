import { useState } from 'react'
import { Link, useLocation, useParams } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowLeft, History, Pencil, RefreshCw, Search } from 'lucide-react'
import { PageHeader } from '@/components/layout/PageHeader'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { EmptyState } from '@/components/ui/EmptyState'
import { Input } from '@/components/ui/Input'
import { PageSpinner } from '@/components/ui/Spinner'
import { ApiError, apiGet, apiPost, errorMessage } from '@/lib/api'
import { truncate } from '@/lib/format'
import { statusLabel } from '@/lib/knowledge'
import { toast } from '@/store/toast'
import type { ChunkEditResponse, ChunkItem, ChunkListResponse, DocumentStatusResponse } from '@/types/knowledge'
import { ChunkEditDialog } from './ChunkEditDialog'
import { RevisionsDialog } from './RevisionsDialog'
import { useKb } from './useKb'

const PREVIEW_CHARS = 600

function ChunkCard({
  chunk,
  stale,
  reindexing,
  onEdit,
  onHistory,
  onReindex,
}: {
  chunk: ChunkItem
  stale: boolean
  reindexing: boolean
  onEdit: () => void
  onHistory: () => void
  onReindex: () => void
}) {
  const [expanded, setExpanded] = useState(false)
  const shown = expanded ? chunk.content : truncate(chunk.content, PREVIEW_CHARS)
  const long = shown !== chunk.content || expanded
  return (
    <li className="rounded-xl border border-border bg-surface p-4 shadow-sm">
      <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
        <div className="flex items-center gap-2 text-xs text-muted">
          <Badge variant="primary">#{chunk.index + 1}</Badge>
          <span>{chunk.char_count} 字</span>
          {stale ? <Badge variant="warning">向量未更新</Badge> : null}
        </div>
        <div className="flex items-center gap-1">
          <Button size="sm" variant="ghost" icon={<Pencil className="size-3.5" />} onClick={onEdit}>
            编辑
          </Button>
          <Button size="sm" variant="ghost" icon={<History className="size-3.5" />} onClick={onHistory}>
            历史
          </Button>
          <Button size="sm" variant={stale ? 'primary' : 'ghost'} icon={<RefreshCw className="size-3.5" />} loading={reindexing} onClick={onReindex}>
            重建索引
          </Button>
        </div>
      </div>
      <p className="text-[13.5px] leading-6 break-words whitespace-pre-wrap">{shown}</p>
      {long ? (
        <button className="mt-1 text-xs text-primary hover:underline" onClick={() => setExpanded((v) => !v)}>
          {expanded ? '收起' : '展开全文'}
        </button>
      ) : null}
    </li>
  )
}

export default function ChunksPage() {
  const { kbId = '', docId = '' } = useParams()
  const location = useLocation()
  const qc = useQueryClient()
  const kb = useKb(kbId)
  const [filter, setFilter] = useState('')
  const [editing, setEditing] = useState<ChunkItem | null>(null)
  const [history, setHistory] = useState<ChunkItem | null>(null)
  // Chunks whose text was changed here and whose vectors have not been rebuilt yet.
  const [stale, setStale] = useState<ReadonlySet<string>>(new Set())

  const chunksKey = ['kb', kbId, 'doc', docId, 'chunks']
  const chunks = useQuery({
    queryKey: chunksKey,
    queryFn: () => apiGet<ChunkListResponse>(`/knowledge-bases/${kbId}/documents/${docId}/chunks`),
  })
  const status = useQuery({
    queryKey: ['kb', kbId, 'doc', docId, 'status'],
    queryFn: () => apiGet<DocumentStatusResponse>(`/knowledge-bases/${kbId}/documents/${docId}/status`),
  })

  const title = (location.state as { title?: string } | null)?.title ?? '文档片段'
  const list = chunks.data?.chunks ?? []
  const needle = filter.trim().toLowerCase()
  const shown = needle ? list.filter((c) => c.content.toLowerCase().includes(needle)) : list

  function markStale(id: string, on: boolean) {
    setStale((prev) => {
      const next = new Set(prev)
      if (on) next.add(id)
      else next.delete(id)
      return next
    })
  }

  function afterChange(res: ChunkEditResponse, chunk: ChunkItem, message: string) {
    void qc.invalidateQueries({ queryKey: chunksKey })
    void qc.invalidateQueries({ queryKey: ['kb', kbId, 'doc', docId, 'revisions', chunk.id] })
    if (res.reindex_required) {
      markStale(chunk.id, true)
      toast.info(`${message}，向量尚未更新，请点击“重建索引”`)
    } else {
      toast.success(message)
    }
  }

  const reindex = useMutation({
    mutationFn: (chunk: ChunkItem) => apiPost<{ indexed: boolean }>(`/knowledge-bases/${kbId}/documents/${docId}/chunks/${chunk.id}/reindex`),
    onSuccess: (_, chunk) => {
      markStale(chunk.id, false)
      toast.success(`片段 #${chunk.index + 1} 已重建索引`)
    },
    onError: (err) => {
      if (err instanceof ApiError && err.status === 409) {
        toast.error('片段在重建期间被修改，没有写入，请重新点击“重建索引”')
        void qc.invalidateQueries({ queryKey: chunksKey })
      } else {
        toast.error(errorMessage(err))
      }
    },
  })

  return (
    <>
      <Link to={`/knowledge/${kbId}`} className="mb-3 inline-flex items-center gap-1 text-[13px] text-muted hover:text-fg">
        <ArrowLeft className="size-3.5" />
        {kb.data?.name ?? '返回知识库'}
      </Link>
      <PageHeader
        title={title}
        description={status.data ? `${statusLabel(status.data.status)} · 共 ${chunks.data?.total ?? status.data.chunk_count} 个片段` : undefined}
      />

      {status.data?.status === 'failed' && status.data.error ? (
        <div role="alert" className="mb-4 rounded-lg border border-danger/40 bg-danger-soft px-3 py-2.5 text-[13px] text-danger">
          解析失败：{status.data.error}
        </div>
      ) : null}

      {chunks.isPending ? (
        <PageSpinner />
      ) : chunks.isError ? (
        <Card>
          <EmptyState
            title="加载失败"
            description={errorMessage(chunks.error)}
            action={
              <Button onClick={() => void chunks.refetch()} loading={chunks.isFetching}>
                重试
              </Button>
            }
          />
        </Card>
      ) : list.length === 0 ? (
        <Card>
          <EmptyState title="这个文档还没有片段" description="文档解析完成后，片段会出现在这里。" />
        </Card>
      ) : (
        <>
          <div className="relative mb-4 max-w-sm">
            <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted" />
            <Input aria-label="在片段中查找" placeholder="在片段中查找" value={filter} onChange={(e) => setFilter(e.target.value)} className="pl-9" />
          </div>
          {shown.length === 0 ? (
            <Card>
              <EmptyState title="没有匹配的片段" />
            </Card>
          ) : (
            <ul className="space-y-3">
              {shown.map((c) => (
                <ChunkCard
                  key={c.id}
                  chunk={c}
                  stale={stale.has(c.id)}
                  reindexing={reindex.isPending && reindex.variables?.id === c.id}
                  onEdit={() => setEditing(c)}
                  onHistory={() => setHistory(c)}
                  onReindex={() => reindex.mutate(c)}
                />
              ))}
            </ul>
          )}
        </>
      )}

      <ChunkEditDialog
        kbId={kbId}
        docId={docId}
        chunk={editing}
        onClose={() => setEditing(null)}
        onSaved={(res, chunk) => {
          setEditing(null)
          afterChange(res, chunk, '片段已保存')
        }}
        onConflict={() => void qc.invalidateQueries({ queryKey: chunksKey })}
      />
      <RevisionsDialog
        kbId={kbId}
        docId={docId}
        chunk={history}
        onClose={() => setHistory(null)}
        onRestored={(res, chunk) => {
          setHistory(null)
          afterChange(res, chunk, '已恢复旧版本')
        }}
      />
    </>
  )
}
