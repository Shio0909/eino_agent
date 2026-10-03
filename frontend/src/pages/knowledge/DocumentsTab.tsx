import { useEffect, useRef, useState, type DragEvent } from 'react'
import { Link, useParams } from 'react-router-dom'
import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { CloudDownload, FileText, Globe, Link2, ListTree, RotateCw, Trash2, Upload } from 'lucide-react'
import { Badge, type BadgeVariant } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Card, CardHeader } from '@/components/ui/Card'
import { ConfirmDialog } from '@/components/ui/Dialog'
import { EmptyState } from '@/components/ui/EmptyState'
import { Pager } from '@/components/ui/Pager'
import { PageSpinner } from '@/components/ui/Spinner'
import { apiDelete, apiGet, apiPost, errorMessage } from '@/lib/api'
import { cn } from '@/lib/cn'
import { formatBytes, formatRelative } from '@/lib/format'
import { canResync, hasActiveImport, isDocActive, stageLabel, statusLabel } from '@/lib/knowledge'
import { toast } from '@/store/toast'
import type { DocumentListResponse, KbDocument } from '@/types/knowledge'
import { UrlImportDialog } from './UrlImportDialog'
import { useDocumentImport } from './useDocumentImport'
import { useKb } from './useKb'

const PAGE_SIZE = 50
const POLL_MS = 3000

function statusVariant(d: KbDocument): BadgeVariant {
  if (d.parse_status === 'failed') return 'danger'
  if (d.parse_status === 'completed') return d.enrichment_status === 'processing' ? 'primary' : 'success'
  return 'primary'
}

function SourceIcon({ type }: { type: KbDocument['source_type'] }) {
  const Icon = type === 'url' ? Globe : FileText
  return <Icon className="size-4 shrink-0 text-muted" aria-hidden />
}

type Pending = { kind: 'delete' | 'reindex'; doc: KbDocument }

export default function DocumentsTab() {
  const { kbId = '' } = useParams()
  const qc = useQueryClient()
  const kb = useKb(kbId)
  const [page, setPage] = useState(1)
  const [pending, setPending] = useState<Pending | null>(null)
  const [urlOpen, setUrlOpen] = useState(false)
  const [dragging, setDragging] = useState(false)
  const fileInput = useRef<HTMLInputElement>(null)
  const { busy, uploadFiles, importUrl } = useDocumentImport(kbId)

  const docs = useQuery({
    queryKey: ['kb', kbId, 'documents', page],
    queryFn: () => apiGet<DocumentListResponse>(`/knowledge-bases/${kbId}/documents`, { page, page_size: PAGE_SIZE }),
    placeholderData: keepPreviousData,
    refetchInterval: (q) => (hasActiveImport(q.state.data?.documents) ? POLL_MS : false),
  })
  const list = docs.data?.documents ?? []
  const total = docs.data?.total ?? list.length

  // Document and chunk counts on the knowledge base change once an import finishes.
  const active = hasActiveImport(docs.data?.documents)
  const wasActive = useRef(false)
  useEffect(() => {
    if (wasActive.current && !active) void qc.invalidateQueries({ queryKey: ['kb', kbId], exact: true })
    wasActive.current = active
  }, [active, qc, kbId])

  const invalidateDocs = () => void qc.invalidateQueries({ queryKey: ['kb', kbId] })

  const remove = useMutation({
    mutationFn: (doc: KbDocument) => apiDelete(`/knowledge-bases/${kbId}/documents/${doc.id}`),
    onSuccess: () => {
      setPending(null)
      invalidateDocs()
      toast.success('文档已删除')
    },
    onError: (err) => {
      setPending(null)
      toast.error(errorMessage(err))
    },
  })

  const resync = useMutation({
    mutationFn: ({ doc, action }: { doc: KbDocument; action: 'reindex' | 'sync' }) =>
      apiPost<{ message?: string; skipped?: boolean }>(`/knowledge-bases/${kbId}/documents/${doc.id}/${action}`),
    onSuccess: (res, { action }) => {
      setPending(null)
      invalidateDocs()
      if (res?.skipped) toast.info(res.message || '文档内容未变化，已跳过同步')
      else toast.success(action === 'reindex' ? '已开始重建索引' : '已开始同步')
    },
    onError: (err) => {
      setPending(null)
      toast.error(errorMessage(err))
    },
  })

  function onDrop(e: DragEvent) {
    e.preventDefault()
    setDragging(false)
    void uploadFiles(Array.from(e.dataTransfer.files))
  }

  const isWiki = kb.data?.mode === 'wiki'

  return (
    <Card
      onDragOver={(e) => {
        e.preventDefault()
        setDragging(true)
      }}
      onDragLeave={() => setDragging(false)}
      onDrop={onDrop}
      className={cn(dragging && 'border-primary ring-2 ring-primary/25')}
    >
      <CardHeader
        title="文档"
        description={isWiki ? 'Wiki 知识库在上传时由大模型同步编译成页面，较大的文档需要等待一段时间' : '支持拖拽文件到此处上传；解析和向量化在后台进行'}
        actions={
          <>
            <input
              ref={fileInput}
              type="file"
              multiple
              hidden
              onChange={(e) => {
                void uploadFiles(Array.from(e.target.files ?? []))
                e.target.value = ''
              }}
            />
            <Button icon={<Link2 className="size-4" />} onClick={() => setUrlOpen(true)} disabled={busy}>
              导入网页
            </Button>
            <Button variant="primary" icon={<Upload className="size-4" />} loading={busy} onClick={() => fileInput.current?.click()}>
              上传文件
            </Button>
          </>
        }
      />

      {docs.isPending ? (
        <PageSpinner />
      ) : docs.isError ? (
        <EmptyState
          title="加载失败"
          description={errorMessage(docs.error)}
          action={
            <Button onClick={() => void docs.refetch()} loading={docs.isFetching}>
              重试
            </Button>
          }
        />
      ) : list.length === 0 ? (
        <EmptyState title={docs.data?.message ?? '还没有文档'} description="上传文件或导入网页后，它们会出现在这里。" />
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-left text-[13px]">
            <thead className="border-b border-border text-xs text-muted">
              <tr>
                <th className="px-4 py-2 font-medium">名称</th>
                <th className="hidden px-4 py-2 font-medium sm:table-cell">状态</th>
                <th className="hidden px-4 py-2 text-right font-medium sm:table-cell">片段</th>
                <th className="hidden px-4 py-2 text-right font-medium md:table-cell">大小</th>
                <th className="hidden px-4 py-2 font-medium md:table-cell">更新</th>
                <th className="sticky right-0 bg-surface px-4 py-2 text-right font-medium">操作</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {list.map((d) => {
                const working = isDocActive(d)
                const title = d.name || d.filename
                const stage = working ? stageLabel(d.stage) : ''
                const statusBadge = (
                  <>
                    <Badge variant={statusVariant(d)}>{d.enrichment_status === 'processing' && d.parse_status === 'completed' ? '增强中' : statusLabel(d.parse_status)}</Badge>
                    {stage ? <span className="ml-1.5 text-xs text-muted">{stage}</span> : null}
                  </>
                )
                return (
                  <tr key={d.id} className="align-top">
                    <td className="max-w-72 px-4 py-2.5">
                      <div className="flex items-center gap-2">
                        <SourceIcon type={d.source_type} />
                        <span className="truncate font-medium" title={title}>
                          {title}
                        </span>
                      </div>
                      <div className="mt-1.5 sm:hidden">
                        {statusBadge}
                        <span className="ml-1.5 text-xs text-muted">{d.chunk_count} 个片段</span>
                      </div>
                      {d.parse_status === 'failed' && d.parse_error ? <p className="mt-1 text-xs break-words text-danger">{d.parse_error}</p> : null}
                      {d.enrichment_status === 'failed' && d.enrichment_error ? (
                        <p className="mt-1 text-xs break-words text-warning">增强失败：{d.enrichment_error}</p>
                      ) : null}
                    </td>
                    <td className="hidden px-4 py-2.5 sm:table-cell">{statusBadge}</td>
                    <td className="hidden px-4 py-2.5 text-right tabular-nums sm:table-cell">{d.chunk_count}</td>
                    <td className="hidden px-4 py-2.5 text-right whitespace-nowrap tabular-nums md:table-cell">{formatBytes(d.file_size)}</td>
                    <td className="hidden px-4 py-2.5 whitespace-nowrap text-muted md:table-cell">{formatRelative(d.updated_at)}</td>
                    <td className="sticky right-0 bg-surface px-4 py-2.5">
                      <div className="flex justify-end gap-0.5">
                        {!isWiki ? (
                          <Link
                            to={`/knowledge/${kbId}/documents/${d.id}`}
                            state={{ title }}
                            aria-label={`查看 ${title} 的片段`}
                            title="查看片段"
                            className="inline-flex size-8 items-center justify-center rounded-lg text-muted hover:bg-surface-2 hover:text-fg"
                          >
                            <ListTree className="size-4" />
                          </Link>
                        ) : null}
                        {canResync(d) ? (
                          <>
                            <Button
                              variant="ghost"
                              size="icon"
                              title="重建索引"
                              aria-label={`重建 ${title} 的索引`}
                              disabled={working || resync.isPending}
                              onClick={() => setPending({ kind: 'reindex', doc: d })}
                            >
                              <RotateCw className="size-4" />
                            </Button>
                            <Button
                              variant="ghost"
                              size="icon"
                              title="同步最新内容"
                              aria-label={`同步 ${title} 的最新内容`}
                              disabled={working || resync.isPending}
                              onClick={() => resync.mutate({ doc: d, action: 'sync' })}
                            >
                              <CloudDownload className="size-4" />
                            </Button>
                          </>
                        ) : null}
                        <Button variant="ghost" size="icon" title="删除" aria-label={`删除 ${title}`} className="hover:text-danger" onClick={() => setPending({ kind: 'delete', doc: d })}>
                          <Trash2 className="size-4" />
                        </Button>
                      </div>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}
      <Pager page={page} pageSize={PAGE_SIZE} total={total} onChange={setPage} />

      <UrlImportDialog open={urlOpen} busy={busy} onClose={() => setUrlOpen(false)} onImport={importUrl} />

      <ConfirmDialog
        open={pending?.kind === 'delete'}
        danger
        title="删除文档"
        confirmLabel="删除"
        loading={remove.isPending}
        message={`确定删除“${pending?.doc.name || pending?.doc.filename}”吗？它的向量和片段会一并删除，无法恢复。`}
        onConfirm={() => pending && remove.mutate(pending.doc)}
        onClose={() => setPending(null)}
      />
      <ConfirmDialog
        open={pending?.kind === 'reindex'}
        title="重建索引"
        confirmLabel="重建"
        loading={resync.isPending}
        message={`将从原始来源重新解析“${pending?.doc.name || pending?.doc.filename}”并重新向量化。旧的片段会先被清除，手动编辑过的片段内容也会丢失。`}
        onConfirm={() => pending && resync.mutate({ doc: pending.doc, action: 'reindex' })}
        onClose={() => setPending(null)}
      />
    </Card>
  )
}
