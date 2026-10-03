import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Database, FileText, Layers, Plus } from 'lucide-react'
import { PageHeader } from '@/components/layout/PageHeader'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { EmptyState } from '@/components/ui/EmptyState'
import { PageSpinner } from '@/components/ui/Spinner'
import { apiGet, errorMessage } from '@/lib/api'
import { formatRelative } from '@/lib/format'
import type { KnowledgeBaseListResponse } from '@/types/knowledge'
import { KbFormDialog } from './KbFormDialog'

export default function KnowledgeListPage() {
  const navigate = useNavigate()
  const [creating, setCreating] = useState(false)
  const list = useQuery({
    queryKey: ['knowledge-bases', 'list'],
    queryFn: () => apiGet<KnowledgeBaseListResponse>('/knowledge-bases', { page_size: 100 }),
  })
  const kbs = list.data?.knowledge_bases ?? []

  return (
    <>
      <PageHeader
        title="知识库"
        description="管理用于检索问答的文档、Wiki 和图谱"
        actions={
          <Button variant="primary" icon={<Plus className="size-4" />} onClick={() => setCreating(true)}>
            新建知识库
          </Button>
        }
      />

      {list.isPending ? (
        <PageSpinner />
      ) : list.isError ? (
        <Card>
          <EmptyState
            title="加载失败"
            description={errorMessage(list.error)}
            action={
              <Button onClick={() => void list.refetch()} loading={list.isFetching}>
                重试
              </Button>
            }
          />
        </Card>
      ) : kbs.length === 0 ? (
        <Card>
          <EmptyState
            icon={<Database className="size-5" />}
            title={list.data?.message ?? '还没有知识库'}
            description="创建一个知识库，再上传文档或导入网页，就可以在对话中检索它们。"
            action={
              <Button variant="primary" icon={<Plus className="size-4" />} onClick={() => setCreating(true)}>
                新建知识库
              </Button>
            }
          />
        </Card>
      ) : (
        <ul className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {kbs.map((kb) => (
            <li key={kb.id}>
              <Link
                to={`/knowledge/${kb.id}`}
                className="flex h-full flex-col gap-3 rounded-xl border border-border bg-surface p-4 shadow-sm transition-colors hover:border-primary/50"
              >
                <div className="flex items-start justify-between gap-2">
                  <h2 className="min-w-0 truncate text-[15px] font-semibold">{kb.name}</h2>
                  <div className="flex shrink-0 gap-1">
                    {kb.embed_stale ? <Badge variant="warning">向量过期</Badge> : null}
                    <Badge variant={kb.mode === 'wiki' ? 'primary' : 'neutral'}>{kb.mode === 'wiki' ? 'Wiki' : '向量'}</Badge>
                  </div>
                </div>
                <p className="line-clamp-2 min-h-10 text-[13px] text-muted">{kb.description || '暂无描述'}</p>
                <div className="mt-auto flex items-center gap-4 text-xs text-muted">
                  <span className="inline-flex items-center gap-1">
                    <FileText className="size-3.5" />
                    {kb.document_count} 篇文档
                  </span>
                  <span className="inline-flex items-center gap-1">
                    <Layers className="size-3.5" />
                    {kb.chunk_count} 个片段
                  </span>
                  <span className="ml-auto">{formatRelative(kb.updated_at)}</span>
                </div>
              </Link>
            </li>
          ))}
        </ul>
      )}

      <KbFormDialog open={creating} onClose={() => setCreating(false)} onCreated={(kb) => navigate(`/knowledge/${kb.id}`)} />
    </>
  )
}
