import { useState } from 'react'
import { Link, NavLink, Outlet, useNavigate, useParams } from 'react-router-dom'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { AlertTriangle, ArrowLeft, Pencil, Trash2 } from 'lucide-react'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { ConfirmDialog } from '@/components/ui/Dialog'
import { EmptyState } from '@/components/ui/EmptyState'
import { PageSpinner } from '@/components/ui/Spinner'
import { apiDelete, errorMessage } from '@/lib/api'
import { cn } from '@/lib/cn'
import { toast } from '@/store/toast'
import { KbFormDialog } from './KbFormDialog'
import { useKb } from './useKb'

export default function KbLayout() {
  const { kbId } = useParams()
  const navigate = useNavigate()
  const qc = useQueryClient()
  const kb = useKb(kbId)
  const [editing, setEditing] = useState(false)
  const [deleting, setDeleting] = useState(false)

  const remove = useMutation({
    mutationFn: () => apiDelete(`/knowledge-bases/${kbId}`),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['knowledge-bases'] })
      toast.success('知识库已删除')
      navigate('/knowledge', { replace: true })
    },
    onError: (err) => {
      setDeleting(false)
      toast.error(errorMessage(err))
    },
  })

  if (kb.isPending) return <PageSpinner />
  if (kb.isError) {
    return (
      <Card>
        <EmptyState
          title="无法打开这个知识库"
          description={errorMessage(kb.error)}
          action={
            <Link to="/knowledge">
              <Button>返回知识库列表</Button>
            </Link>
          }
        />
      </Card>
    )
  }

  const data = kb.data
  const tabs = [{ to: `/knowledge/${data.id}`, label: '文档', end: true }]

  return (
    <>
      <Link to="/knowledge" className="mb-3 inline-flex items-center gap-1 text-[13px] text-muted hover:text-fg">
        <ArrowLeft className="size-3.5" />
        全部知识库
      </Link>

      <div className="mb-4 flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <h1 className="text-xl font-semibold break-all">{data.name}</h1>
            <Badge variant={data.mode === 'wiki' ? 'primary' : 'neutral'}>{data.mode === 'wiki' ? 'Wiki' : '向量'}</Badge>
          </div>
          {data.description ? <p className="mt-1 text-[13px] text-muted">{data.description}</p> : null}
          <p className="mt-1 text-xs text-muted">
            {data.document_count} 篇文档 · {data.chunk_count} 个片段 · 向量维度 {data.embedding_dimensions}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button icon={<Pencil className="size-4" />} onClick={() => setEditing(true)}>
            编辑
          </Button>
          <Button variant="danger" icon={<Trash2 className="size-4" />} onClick={() => setDeleting(true)}>
            删除
          </Button>
        </div>
      </div>

      {data.embed_stale ? (
        <div role="alert" className="mb-4 flex gap-2 rounded-lg border border-warning/40 bg-warning-soft px-3 py-2.5 text-[13px] text-warning">
          <AlertTriangle className="mt-0.5 size-4 shrink-0" />
          <p>这个知识库的向量由另一个嵌入模型生成，检索结果可能不准确。请对文档执行“重建索引”。</p>
        </div>
      ) : null}

      <nav className="mb-4 flex gap-1 border-b border-border" aria-label="知识库分区">
        {tabs.map((t) => (
          <NavLink
            key={t.to}
            to={t.to}
            end={t.end}
            className={({ isActive }) =>
              cn('-mb-px border-b-2 px-3 py-2 text-[13.5px] font-medium', isActive ? 'border-primary text-primary' : 'border-transparent text-muted hover:text-fg')
            }
          >
            {t.label}
          </NavLink>
        ))}
      </nav>

      <Outlet />

      <KbFormDialog open={editing} onClose={() => setEditing(false)} kb={data} />
      <ConfirmDialog
        open={deleting}
        danger
        title="删除知识库"
        confirmLabel="删除"
        loading={remove.isPending}
        message={`确定删除“${data.name}”吗？其中的 ${data.document_count} 篇文档、向量和导入记录都会被删除，无法恢复。`}
        onConfirm={() => remove.mutate()}
        onClose={() => setDeleting(false)}
      />
    </>
  )
}
