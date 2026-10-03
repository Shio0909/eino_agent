import { useMutation, useQuery } from '@tanstack/react-query'
import { History } from 'lucide-react'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Dialog } from '@/components/ui/Dialog'
import { EmptyState } from '@/components/ui/EmptyState'
import { PageSpinner } from '@/components/ui/Spinner'
import { apiGet, apiPost, errorMessage } from '@/lib/api'
import { formatDateTime } from '@/lib/format'
import { revisionReasonLabel } from '@/lib/knowledge'
import { toast } from '@/store/toast'
import type { ChunkEditResponse, ChunkItem, ChunkRevision } from '@/types/knowledge'

interface RevisionsDialogProps {
  kbId: string
  docId: string
  chunk: ChunkItem | null
  onClose: () => void
  onRestored: (res: ChunkEditResponse, chunk: ChunkItem) => void
}

export function RevisionsDialog({ chunk, ...rest }: RevisionsDialogProps) {
  return chunk ? <Revisions key={chunk.id} chunk={chunk} {...rest} /> : null
}

function Revisions({ kbId, docId, chunk, onClose, onRestored }: Omit<RevisionsDialogProps, 'chunk'> & { chunk: ChunkItem }) {
  const base = `/knowledge-bases/${kbId}/documents/${docId}/chunks/${chunk.id}`
  const revisions = useQuery({
    queryKey: ['kb', kbId, 'doc', docId, 'revisions', chunk.id],
    queryFn: () => apiGet<{ revisions: ChunkRevision[] }>(`${base}/revisions`),
  })
  const restore = useMutation({
    mutationFn: (rev: ChunkRevision) => apiPost<ChunkEditResponse>(`${base}/revisions/${rev.id}/restore`),
    onSuccess: (res) => onRestored(res, chunk),
    onError: (err) => toast.error(errorMessage(err)),
  })

  const list = revisions.data?.revisions ?? []

  return (
    <Dialog open onClose={onClose} title={`片段 #${chunk.index + 1} 的修改历史`} description="这里保存的是每次修改前的旧内容。恢复后需要重建此片段的索引才会影响检索。" className="max-w-2xl">
      {revisions.isPending ? (
        <PageSpinner />
      ) : revisions.isError ? (
        <EmptyState title="加载失败" description={errorMessage(revisions.error)} action={<Button onClick={() => void revisions.refetch()}>重试</Button>} />
      ) : list.length === 0 ? (
        <EmptyState icon={<History className="size-5" />} title="还没有修改历史" description="编辑这个片段后，旧版本会保存在这里。" />
      ) : (
        <ul className="space-y-3">
          {list.map((rev) => {
            const current = rev.content_hash === chunk.content_hash
            return (
              <li key={rev.id} className="rounded-lg border border-border p-3">
                <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
                  <div className="flex flex-wrap items-center gap-2 text-xs text-muted">
                    <span>{formatDateTime(rev.created_at)}</span>
                    {rev.reason ? <Badge>{revisionReasonLabel(rev.reason)}</Badge> : null}
                    {current ? <Badge variant="success">与当前内容相同</Badge> : null}
                  </div>
                  <Button size="sm" disabled={current || restore.isPending} loading={restore.isPending && restore.variables?.id === rev.id} onClick={() => restore.mutate(rev)}>
                    恢复此版本
                  </Button>
                </div>
                <pre className="max-h-40 overflow-auto rounded-md bg-code p-2.5 text-xs leading-5 whitespace-pre-wrap break-words">{rev.content}</pre>
              </li>
            )
          })}
        </ul>
      )}
    </Dialog>
  )
}
