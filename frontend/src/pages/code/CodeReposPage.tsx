import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Download, GitBranch, GitCommit, Network, Plus, RefreshCw, Trash2 } from 'lucide-react'
import { PageHeader } from '@/components/layout/PageHeader'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { ConfirmDialog } from '@/components/ui/Dialog'
import { EmptyState } from '@/components/ui/EmptyState'
import { PageSpinner } from '@/components/ui/Spinner'
import { useIsAdmin } from '@/hooks/useIsAdmin'
import { ApiError, apiDelete, apiGet, apiPost, errorMessage } from '@/lib/api'
import { describeIndexStats, describePull } from '@/lib/codeRepos'
import { formatRelative } from '@/lib/format'
import { toast } from '@/store/toast'
import type { CodeRepo, CodeRepoIndexResponse, CodeRepoPullResponse } from '@/types/settings'
import { CloneDialog } from './CloneDialog'

type Action = 'pull' | 'index' | 'delete'

const QUERY_KEY = ['code-repos']

function failure(err: unknown): string {
  if (err instanceof ApiError && err.status === 503) return `代码索引器没有启动：${err.message}`
  if (err instanceof ApiError && err.status === 404) return '仓库已经不存在，列表已刷新'
  return errorMessage(err)
}

export default function CodeReposPage() {
  const isAdmin = useIsAdmin()
  const queryClient = useQueryClient()
  const [cloning, setCloning] = useState(false)
  const [deleting, setDeleting] = useState<CodeRepo | null>(null)
  // Repos that pulled new commits and have not been indexed since.
  const [stale, setStale] = useState<Record<string, true>>({})

  const list = useQuery({
    queryKey: QUERY_KEY,
    queryFn: () => apiGet<{ repos: CodeRepo[] | null }>('/code-repos'),
  })
  const repos = list.data?.repos ?? []

  const clearStale = (name: string) =>
    setStale((s) => Object.fromEntries(Object.entries(s).filter(([key]) => key !== name)) as Record<string, true>)
  const refresh = () => queryClient.invalidateQueries({ queryKey: QUERY_KEY })

  const run = useMutation({
    mutationFn: async ({ repo, action }: { repo: CodeRepo; action: Action }) => {
      const path = `/code-repos/${encodeURIComponent(repo.name)}`
      if (action === 'pull') return apiPost<CodeRepoPullResponse>(`${path}/pull`)
      if (action === 'index') return apiPost<CodeRepoIndexResponse>(`${path}/index`)
      return apiDelete<{ message: string }>(path)
    },
    onSuccess: (res, { repo, action }) => {
      if (action === 'pull') {
        const outcome = describePull(res as CodeRepoPullResponse)
        toast.success(`${repo.name}：${outcome.text}`)
        if (outcome.stale) setStale((s) => ({ ...s, [repo.name]: true }))
      } else if (action === 'index') {
        toast.success(`${repo.name} 索引完成：${describeIndexStats((res as CodeRepoIndexResponse).stats)}`)
        clearStale(repo.name)
      } else {
        toast.success(`已删除 ${repo.name}`)
        clearStale(repo.name)
        setDeleting(null)
      }
      void refresh()
    },
    onError: (err) => {
      toast.error(failure(err))
      if (err instanceof ApiError && err.status === 404) {
        setDeleting(null)
        void refresh()
      }
    },
  })

  const busy = (repo: CodeRepo, action: Action) => run.isPending && run.variables.repo.name === repo.name && run.variables.action === action

  return (
    <>
      <PageHeader
        title="代码仓库"
        description="供代码图谱检索使用的本地 Git 仓库"
        actions={
          <>
            <Button icon={<RefreshCw className={list.isFetching ? 'size-4 animate-spin' : 'size-4'} />} onClick={() => void list.refetch()} disabled={list.isFetching}>
              刷新
            </Button>
            {isAdmin ? (
              <Button variant="primary" icon={<Plus className="size-4" />} onClick={() => setCloning(true)}>
                克隆仓库
              </Button>
            ) : null}
          </>
        }
      />
      {!isAdmin ? <p className="-mt-2 mb-4 text-[13px] text-muted">克隆、拉取、索引和删除需要管理员权限。</p> : null}

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
      ) : repos.length === 0 ? (
        <Card>
          <EmptyState
            icon={<GitBranch className="size-5" />}
            title="还没有代码仓库"
            description="克隆一个仓库并建立索引后，智能体就能按函数和调用关系检索代码。"
            action={
              isAdmin ? (
                <Button variant="primary" icon={<Plus className="size-4" />} onClick={() => setCloning(true)}>
                  克隆仓库
                </Button>
              ) : undefined
            }
          />
        </Card>
      ) : (
        <ul className="space-y-3">
          {repos.map((repo) => (
            <li key={repo.name}>
              <Card className="flex flex-wrap items-center gap-x-6 gap-y-3 p-4">
                <div className="min-w-0 flex-1 basis-72">
                  <div className="flex flex-wrap items-center gap-2">
                    <h2 className="truncate text-[15px] font-semibold">{repo.name}</h2>
                    {repo.branch ? (
                      <Badge>
                        <GitBranch className="size-3" />
                        {repo.branch}
                      </Badge>
                    ) : null}
                    {repo.indexed ? <Badge variant="success">已索引</Badge> : <Badge variant="warning">未索引</Badge>}
                    {stale[repo.name] ? <Badge variant="warning">索引已过期</Badge> : null}
                  </div>
                  <p className="mt-1 truncate font-mono text-xs text-muted" title={repo.path}>
                    {repo.path}
                  </p>
                  <p className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted">
                    {repo.last_commit ? (
                      <span className="inline-flex items-center gap-1">
                        <GitCommit className="size-3.5" />
                        <code className="font-mono">{repo.last_commit}</code>
                        {repo.last_commit_date ? <span>· {formatRelative(repo.last_commit_date)}</span> : null}
                      </span>
                    ) : null}
                    {repo.index_stats ? (
                      <span>
                        {repo.index_stats.files} 个文件 · {repo.index_stats.entities} 个实体 · {repo.index_stats.relations} 条关系
                      </span>
                    ) : null}
                  </p>
                </div>
                {isAdmin ? (
                  <div className="flex flex-wrap items-center gap-2">
                    <Button size="sm" icon={<Download className="size-3.5" />} loading={busy(repo, 'pull')} disabled={run.isPending} onClick={() => run.mutate({ repo, action: 'pull' })}>
                      拉取
                    </Button>
                    <Button size="sm" icon={<Network className="size-3.5" />} loading={busy(repo, 'index')} disabled={run.isPending} onClick={() => run.mutate({ repo, action: 'index' })}>
                      {repo.indexed ? '重新索引' : '建立索引'}
                    </Button>
                    <Button size="icon" variant="ghost" aria-label={`删除 ${repo.name}`} disabled={run.isPending} onClick={() => setDeleting(repo)}>
                      <Trash2 className="size-4" />
                    </Button>
                  </div>
                ) : null}
              </Card>
            </li>
          ))}
        </ul>
      )}

      <CloneDialog open={cloning} onClose={() => setCloning(false)} onCloned={() => void refresh()} />
      <ConfirmDialog
        open={!!deleting}
        title="删除代码仓库"
        message={deleting ? `将删除 ${deleting.name} 的本地代码目录和它的代码图谱数据，无法恢复。` : ''}
        confirmLabel="删除"
        danger
        loading={run.isPending && run.variables.action === 'delete'}
        onConfirm={() => deleting && run.mutate({ repo: deleting, action: 'delete' })}
        onClose={() => setDeleting(null)}
      />
    </>
  )
}
